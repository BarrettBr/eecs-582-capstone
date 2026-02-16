package ingest

/*
Name: ingest/internal/ingest/validation.go
Description: Loads validation rules from JSON and checks each sample for required fields, bounds, and anomalies.
Programmer: Barrett Brown
Date Created: 2026-02-01
Dates Revised: 2026-02-15
Revision History:
- 2026-02-13, Barrett Brown: Created file and started structing out
- 2026-02-15, Barrett Brown: Added standardized prologue documentation block.
Preconditions:
- Validation JSON file exists and matches expected format.
- Input sample is a struct (or pointer to struct).
Acceptable Input Values/Types:
- Rule JSON with required_fields, bounds, and anomaly_rules.
- Sample fields with numeric types that can be converted to float64.
Unacceptable Input Values/Types:
- Non-struct sample values for Validate.
- Unsupported comparison operators in anomaly rules.
- Non-numeric values for fields expected to be numeric.
Postconditions:
- Validate returns a ValidationResult for the sample using the loaded rules.
Return Values/Types:
- NewRuleEngine: (*RuleEngine, error)
- Validate: ValidationResult
- Helper functions return parsed/comparison values and error info.
Error/Exception Conditions:
- Rule file read/unmarshal failures.
- Bad operators or bad field types are reported in ValidationResult errors.
Side Effects:
- NewRuleEngine reads a file from disk; validation itself is in-memory.
Invariants:
- RuleEngine.spec remains unchanged during validation execution.
- Missing numeric fields are treated as absent unless marked required.
Known Faults:
- Reflection adds overhead and may not cover complex nested structs perfectly.
*/

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
)

// description: Declarative validation definition loaded from JSON config.
// input: JSON file fields required_fields, bounds, and anomaly_rules.
// output: Used by RuleEngine to validate and annotate incoming samples.
type ValidationSpec struct {
	RequiredFields []string          `json:"required_fields"`
	Bounds         map[string]Bounds `json:"bounds"`
	AnomalyRules   []AnomalyRule     `json:"anomaly_rules"`
}

// description: Numeric min/max bound constraints for a sample field.
// input: Optional Min and Max float pointers from config.
// output: Applied by validation to detect out-of-range values.
type Bounds struct {
	Min *float64 `json:"min,omitempty"`
	Max *float64 `json:"max,omitempty"`
}

// description: Rule defining anomaly detection using a comparison operator on one field.
// input: Field name, operator token, threshold value, and anomaly label.
// output: Produces labeled anomaly entries when comparison matches.
type AnomalyRule struct {
	Field string  `json:"field"`
	Op    string  `json:"op"`
	Value float64 `json:"value"`
	Label string  `json:"label"`
}

// description: Result object returned after validating one sample.
// input: Aggregated errors/anomaly labels produced by RuleEngine checks.
// output: Valid flag plus detailed errors and anomaly labels.
type ValidationResult struct {
	Valid     bool
	Errors    []string
	Anomalies []string
}

// description: Validation executor built from a loaded ValidationSpec.
// input: spec loaded from JSON rules file.
// output: Provides Validate to evaluate samples consistently each tick.
type RuleEngine struct {
	spec ValidationSpec
}

// description: Loads validation rules from disk and constructs a RuleEngine.
// input: specPath (path to validation JSON rules).
// output: Returns initialized RuleEngine or an error on read/parse failures.
func NewRuleEngine(specPath string) (*RuleEngine, error) {
	// Read in the file
	content, err := os.ReadFile(specPath)
	if err != nil {
		return nil, fmt.Errorf("Read validation spec %s: %w", specPath, err)
	}

	// Unmarshal the files data into the spec for it
	var spec ValidationSpec
	if err := json.Unmarshal(content, &spec); err != nil {
		return nil, fmt.Errorf("Parse validation spec %s: %w", specPath, err)
	}
	if spec.Bounds == nil {
		spec.Bounds = make(map[string]Bounds)
	}

	return &RuleEngine{spec: spec}, nil
}

// description: Validates one sample against required fields, bounds, and anomaly rules.
// input: sample (struct or pointer-to-struct containing json-tagged fields).
// output: Returns ValidationResult with validity status, errors, and anomaly labels.
func (e *RuleEngine) Validate(sample any) ValidationResult {
	// Convert the struct to a record format and then validate it
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

	// If any errors exist this isnt a valid record anymore
	if len(result.Errors) > 0 {
		result.Valid = false
	}
	return result
}

// description: Converts a struct sample into a map keyed by JSON tag (or field name fallback).
// input: sample (struct or pointer-to-struct).
// output: Returns record map and true on success; empty map and false on invalid input.
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

// description: Compares two float64 values using a string operator token.
// input: left operand, op token (>, >=, <, <=, ==, !=), right operand.
// output: Returns comparison result or error for unsupported operators.
func compareFloat(left float64, op string, right float64) (bool, error) {
	// Basic switch statement to take the rule operator in the file and do the operation
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

// description: Normalizes supported numeric Go types into float64.
// input: value (any numeric primitive type).
// output: Returns converted float64 and true, or zero and false if non-numeric.
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

// description: Detects missing values for validation-required checks.
// input: value (any field value).
// output: Returns true for nil or empty/whitespace strings, false otherwise.
func isMissingValue(value any) bool {
	// Check if a value is missing or if it is just whitespace
	if value == nil {
		return true
	}

	if s, ok := value.(string); ok {
		return strings.TrimSpace(s) == ""
	}
	return false
}

// description: Retrieves a field from a record and coerces it to float64 when possible.
// input: record map and field key.
// output: Returns number, found flag, and error message; missing fields return found=false with empty error.
func getFloatNumber(record map[string]any, field string) (float64, bool, string) {
	// Get a value from the record and validate it is non-missing
	value, exists := record[field]
	if !exists || isMissingValue(value) {
		return 0, false, ""
	}

	// Ensure the value is a float64 value
	num, ok := asFloat64(value)
	if !ok {
		return 0, false, fmt.Sprintf("Field %s is not numeric", field)
	}

	return num, true, ""
}
