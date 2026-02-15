package ingest

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
)

type ValidationSpec struct {
	RequiredFields []string          `json:"required_fields"`
	Bounds         map[string]Bounds `json:"bounds"`
	AnomalyRules   []AnomalyRule     `json:"anomaly_rules"`
}

type Bounds struct {
	Min *float64 `json:"min,omitempty"`
	Max *float64 `json:"max,omitempty"`
}

type AnomalyRule struct {
	Field string  `json:"field"`
	Op    string  `json:"op"`
	Value float64 `json:"value"`
	Label string  `json:"label"`
}

type ValidationResult struct {
	Valid     bool
	Errors    []string
	Anomalies []string
}

type RuleEngine struct {
	spec ValidationSpec
}

// Fills out the rule engine based off of the rules file
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

	return &RuleEngine{spec: spec}, nil
}

// Validates a sample against the rule engine, appending errors and then
// Returning the result of this
func (e *RuleEngine) Validate(sample any) ValidationResult {
	record, ok := structToRecord(sample)
	if !ok {
		return ValidationResult{
			Valid:  false,
			Errors: []string{"Sample value must be a struct or pointer to struct"},
		}
	}

	result := ValidationResult{
		Valid: true,
	}

	// Go over record and make sure every required field exists
	for _, field := range e.spec.RequiredFields {
		value, exists := record[field]
		if !exists || isMissingValue(value) {
			result.Errors = append(result.Errors, fmt.Sprintf("Missing required field: %s", field))
		}
	}

	// Check over records bounds
	for field, bound := range e.spec.Bounds {
		num, ok, errMsg := getFloatNumber(record, field)
		if errMsg != "" {
			result.Errors = append(result.Errors, errMsg)
			continue
		}
		if !ok {
			continue // missing or not present -> skip
		}

		// If the value is outside bounds append and error for it
		if bound.Min != nil && num < *bound.Min {
			result.Errors = append(result.Errors, fmt.Sprintf("Field %s below min: %.4f < %.4f", field, num, *bound.Min))
		}
		if bound.Max != nil && num > *bound.Max {
			result.Errors = append(result.Errors, fmt.Sprintf("Field %s above max: %.4f > %.4f", field, num, *bound.Max))
		}
	}

	// Check over manual anomaly rules
	for _, rule := range e.spec.AnomalyRules {
		num, ok, errMsg := getFloatNumber(record, rule.Field)
		if errMsg != "" {
			result.Errors = append(result.Errors, errMsg)
			continue
		}
		if !ok {
			continue
		}

		// Compare the number against the rule using the op stored
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

// ------ Helper Function ------

// Change this down the line, just ripped some code for ezpz testing
// But don't really know reflection or how this works
func structToRecord(sample any) (map[string]any, bool) {
	record := make(map[string]any)

	v := reflect.ValueOf(sample)
	if !v.IsValid() {
		return record, false
	}
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return record, false
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return record, false
	}

	t := v.Type()
	for i := range t.NumField() {
		field := t.Field(i)
		if field.PkgPath != "" {
			continue
		}

		tag := field.Tag.Get("json")
		name := strings.Split(tag, ",")[0]
		if name == "" {
			name = field.Name
		}
		if name == "-" {
			continue
		}
		record[name] = v.Field(i).Interface()
	}
	return record, true
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
	// Helper function that just goes over the number types and cast them all to float64
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

// Looks up field in record and returns a float64
// bool represents success and string is the error
// false + "" = missing / absent so up to caller to define handling
func getFloatNumber(record map[string]any, field string) (float64, bool, string) {
	value, exists := record[field]
	if !exists || isMissingValue(value) {
		return 0, false, ""
	}

	num, ok := asFloat64(value)
	if !ok {
		return 0, false, fmt.Sprintf("Field %s is not numeric", field)
	}

	return num, true, ""
}
