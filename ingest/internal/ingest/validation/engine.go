package validation

/*
Name: ingest/internal/ingest/validation/engine.go
Description: Loads validation rules from JSON and checks normalized events for required fields, bounds, and anomalies.
Programmer: Barrett Brown
Date Created: 2026-02-01
Dates Revised: 2026-03-14
Revision History:
- 2026-02-13, Barrett Brown: Created the validation file and started structuring out the rule engine.
- 2026-02-15, Barrett Brown: Added standardized prologue documentation block.
- 2026-02-28, Barrett Brown: Changed validation to use a generic record interface instead of harder coded logic.
- 2026-03-14, Barrett Brown: Removed per-event ToRecord map allocation in favor of direct field access on RecordEvent.
- 2026-03-14, Barrett Brown: Moved the validation engine into the validation package during the ingest file structure refactor.
Preconditions:
- Validation rule JSON exists and is syntactically correct for the expected schema.
- Samples implement events.RecordEvent field access for the configured rule names.
Acceptable Input Values/Types:
- JSON rules with required_fields ([]string), bounds (field->min/max), and anomaly_rules.
- Sample values containing numeric types supported by asFloat64.
Unacceptable Input Values/Types:
- Samples that do not implement ValidationValue for the referenced fields.
- Unsupported comparison operators in anomaly rules.
- Non-numeric values for fields expected to be numeric.
Postconditions:
- Validate returns deterministic ValidationResult for the given rules and sample snapshot.
Return Values/Types:
- NewRuleEngine: (*RuleEngine, error)
- Validate: ValidationResult
- Helper functions return typed parse or compare outputs and error indicators.
Error/Exception Conditions:
- Rule file read or unmarshal failures.
- Operator or type mismatch conditions are reported through ValidationResult errors.
Side Effects:
- Reads rules from disk in NewRuleEngine; otherwise uses pure in-memory processing.
Invariants:
- RuleEngine.Spec remains unchanged during validation execution.
- Missing numeric fields are treated as absent unless marked required.
Known Faults:
- Field lookup is still string based per rule check.
*/

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	ingestevents "github.com/BarrettBr/eecs-582-capstone/internal/ingest/events"
)

// ValidationSpec is the declarative validation definition loaded from JSON config.
type ValidationSpec struct {
	RequiredFields []string          `json:"required_fields"`
	Bounds         map[string]Bounds `json:"bounds"`
	AnomalyRules   []AnomalyRule     `json:"anomaly_rules"`
}

// Bounds describes numeric min/max constraints for a field.
type Bounds struct {
	Min *float64 `json:"min,omitempty"`
	Max *float64 `json:"max,omitempty"`
}

// AnomalyRule defines a threshold-based anomaly label rule.
type AnomalyRule struct {
	Field string  `json:"field"`
	Op    string  `json:"op"`
	Value float64 `json:"value"`
	Label string  `json:"label"`
}

// ValidationResult captures validation pass/fail status and attached details.
type ValidationResult struct {
	Valid     bool
	Errors    []string
	Anomalies []string
}

// RuleEngine validates normalized events against one loaded rule spec.
type RuleEngine struct {
	Spec ValidationSpec
}

// NewRuleEngine loads validation rules from disk and constructs a RuleEngine.
func NewRuleEngine(specPath string) (*RuleEngine, error) {
	content, err := os.ReadFile(specPath)
	if err != nil {
		return nil, fmt.Errorf("Read validation spec %s: %w", specPath, err)
	}

	var spec ValidationSpec
	if err := json.Unmarshal(content, &spec); err != nil {
		return nil, fmt.Errorf("Parse validation spec %s: %w", specPath, err)
	}
	if spec.Bounds == nil {
		spec.Bounds = make(map[string]Bounds)
	}

	return &RuleEngine{Spec: spec}, nil
}

// Validate checks one event against required fields, bounds, and anomaly rules.
func (e *RuleEngine) Validate(sample ingestevents.RecordEvent) ValidationResult {
	result := ValidationResult{Valid: true}

	for _, field := range e.Spec.RequiredFields {
		value, exists := sample.ValidationValue(field)
		if !exists || isMissingValue(value) {
			result.Errors = append(result.Errors, fmt.Sprintf("Missing required field: %s", field))
		}
	}

	for field, bound := range e.Spec.Bounds {
		num, ok, errMsg := getFloatNumber(sample, field)
		if errMsg != "" {
			result.Errors = append(result.Errors, errMsg)
			continue
		}
		if !ok {
			continue
		}
		if bound.Min != nil && num < *bound.Min {
			result.Errors = append(result.Errors, fmt.Sprintf("Field %s below min: %.4f < %.4f", field, num, *bound.Min))
		}
		if bound.Max != nil && num > *bound.Max {
			result.Errors = append(result.Errors, fmt.Sprintf("Field %s above max: %.4f > %.4f", field, num, *bound.Max))
		}
	}

	for _, rule := range e.Spec.AnomalyRules {
		num, ok, errMsg := getFloatNumber(sample, rule.Field)
		if errMsg != "" {
			result.Errors = append(result.Errors, errMsg)
			continue
		}
		if !ok {
			continue
		}

		matched, err := compareFloat(num, rule.Op, rule.Value)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Invalid operator %q for field %s", rule.Op, rule.Field))
			continue
		}
		if matched {
			result.Anomalies = append(result.Anomalies, rule.Label)
		}
	}

	if len(result.Errors) > 0 {
		result.Valid = false
	}
	return result
}

func compareFloat(left float64, op string, right float64) (bool, error) {
	switch op {
	case ">":
		return left > right, nil
	case ">=":
		return left >= right, nil
	case "<":
		return left < right, nil
	case "<=":
		return left <= right, nil
	case "==":
		return left == right, nil
	case "!=":
		return left != right, nil
	default:
		return false, fmt.Errorf("unsupported operator")
	}
}

func asFloat64(value any) (float64, bool) {
	switch x := value.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int8:
		return float64(x), true
	case int16:
		return float64(x), true
	case int32:
		return float64(x), true
	case int64:
		return float64(x), true
	case uint:
		return float64(x), true
	case uint8:
		return float64(x), true
	case uint16:
		return float64(x), true
	case uint32:
		return float64(x), true
	case uint64:
		return float64(x), true
	default:
		return 0, false
	}
}

func isMissingValue(value any) bool {
	if value == nil {
		return true
	}
	if s, ok := value.(string); ok {
		return strings.TrimSpace(s) == ""
	}
	return false
}

func getFloatNumber(sample ingestevents.RecordEvent, field string) (float64, bool, string) {
	value, exists := sample.ValidationValue(field)
	if !exists || isMissingValue(value) {
		return 0, false, ""
	}

	num, ok := asFloat64(value)
	if !ok {
		return 0, false, fmt.Sprintf("Field %s is not numeric", field)
	}

	return num, true, ""
}
