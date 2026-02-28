package ingest

/*
Name: ingest/internal/ingest/validation_internal_test.go
Description: Tests validation helpers, file loading errors, and validation failure paths.
Programmer: Barrett Brown
Date Created: 2026-02-28
Dates Revised: 2026-02-28
Revision History:
- 2026-02-28, Barrett Brown: Created internal validation helper coverage tests.
Preconditions:
- Temporary files can be written for validation spec tests.
Acceptable Input Values/Types:
- Valid and invalid operator strings.
- Numeric and non numeric record values.
- Missing and malformed validation files.
Unacceptable Input Values/Types:
- No special invalid runtime dependencies beyond malformed test data.
Postconditions:
- Confirms helper functions and validation error paths return expected results.
Return Values/Types:
- Test functions return no value.
Error/Exception Conditions:
- Unexpected missing errors for malformed rules or invalid operators.
Side Effects:
- Writes temp files for validation spec loading tests.
Invariants:
- Stub record event stays local to test usage.
Known Faults:
- Does not cover every comparison and bounds combination.
*/

import (
	"os"
	"path/filepath"
	"testing"
)

type stubRecordEvent struct {
	record map[string]any
}

func (s stubRecordEvent) ToRecord() map[string]any { return s.record }
func (s stubRecordEvent) EventType() string        { return "stub" }
func (s stubRecordEvent) Payload() any             { return s.record }
func (s stubRecordEvent) AnomalyLabels() []string  { return nil }

func TestNewRuleEngineErrorsOnMissingOrInvalidFile(t *testing.T) {
	if _, err := NewRuleEngine(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Fatalf("NewRuleEngine() missing file error = nil, want non-nil")
	}

	specPath := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(specPath, []byte("{not-json"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, err := NewRuleEngine(specPath); err == nil {
		t.Fatalf("NewRuleEngine() invalid json error = nil, want non-nil")
	}
}

func TestCompareFloatOperators(t *testing.T) {
	tests := []struct {
		op   string
		want bool
	}{
		{op: ">", want: true},
		{op: ">=", want: true},
		{op: "<", want: false},
		{op: "<=", want: false},
		{op: "==", want: false},
		{op: "!=", want: true},
	}

	for _, tt := range tests {
		got, err := compareFloat(2, tt.op, 1)
		if err != nil {
			t.Fatalf("compareFloat(%q) error = %v", tt.op, err)
		}
		if got != tt.want {
			t.Fatalf("compareFloat(%q) = %v, want %v", tt.op, got, tt.want)
		}
	}

	if _, err := compareFloat(1, "???", 1); err == nil {
		t.Fatalf("compareFloat() invalid operator error = nil, want non-nil")
	}
}

func TestAsFloat64AndGetFloatNumber(t *testing.T) {
	if got, ok := asFloat64(int16(8)); !ok || got != 8 {
		t.Fatalf("asFloat64(int16) = (%v, %v), want (8, true)", got, ok)
	}
	if _, ok := asFloat64("bad"); ok {
		t.Fatalf("asFloat64(string) ok = true, want false")
	}

	record := map[string]any{
		"good":  uint16(9),
		"bad":   "nope",
		"empty": "   ",
	}

	if got, ok, errMsg := getFloatNumber(record, "good"); !ok || errMsg != "" || got != 9 {
		t.Fatalf("getFloatNumber(good) = (%v, %v, %q), want (9, true, \"\")", got, ok, errMsg)
	}
	if _, ok, errMsg := getFloatNumber(record, "bad"); ok || errMsg == "" {
		t.Fatalf("getFloatNumber(bad) = ok:%v err:%q, want ok:false and non-empty err", ok, errMsg)
	}
	if _, ok, errMsg := getFloatNumber(record, "empty"); ok || errMsg != "" {
		t.Fatalf("getFloatNumber(empty) = ok:%v err:%q, want ok:false and empty err", ok, errMsg)
	}
}

func TestIsMissingValue(t *testing.T) {
	if !isMissingValue(nil) {
		t.Fatalf("isMissingValue(nil) = false, want true")
	}
	if !isMissingValue("   ") {
		t.Fatalf("isMissingValue(whitespace) = false, want true")
	}
	if isMissingValue("value") {
		t.Fatalf("isMissingValue(value) = true, want false")
	}
}

func TestValidateAddsErrorsForInvalidOperatorAndNonNumericField(t *testing.T) {
	engine := &RuleEngine{
		Spec: ValidationSpec{
			Bounds: map[string]Bounds{
				"temperature": {},
			},
			AnomalyRules: []AnomalyRule{
				{Field: "temperature", Op: "??", Value: 5, Label: "bad"},
				{Field: "mode", Op: "==", Value: 1, Label: "ignored"},
			},
		},
	}

	result := engine.Validate(stubRecordEvent{
		record: map[string]any{
			"temperature": 10.0,
			"mode":        "auto",
		},
	})

	if result.Valid {
		t.Fatalf("Validate() Valid = true, want false")
	}
	if len(result.Errors) < 2 {
		t.Fatalf("Validate() errors = %v, want at least 2", result.Errors)
	}
}
