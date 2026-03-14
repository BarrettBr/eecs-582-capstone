package validation_test

/*
Name: ingest/internal/ingest/validation/engine_test.go
Description: External test for rule loading and record based validation behavior in the validation package.
Programmer: Barrett Brown
Date Created: 2026-03-14
Dates Revised: 2026-03-14
Revision History:
- 2026-03-14, Barrett Brown: Added external validation coverage after removing the old root ingest compatibility layer.
Preconditions:
- Temporary files can be written for validation spec loading.
Acceptable Input Values/Types:
- Valid JSON validation specs.
- TempSample values that should pass and fail validation.
Unacceptable Input Values/Types:
- No special invalid runtime dependencies are required.
Postconditions:
- Confirms rule loading works and validation returns expected errors and anomalies.
Return Values/Types:
- Test functions return no value.
Error/Exception Conditions:
- Validation spec write failures.
- Unexpected missing errors for invalid samples.
Side Effects:
- Writes a temporary validation spec file.
Invariants:
- Validation should use the rule file loaded for the test only.
Known Faults:
- Does not cover multiple anomaly rule combinations.
*/

import (
	"os"
	"path/filepath"
	"testing"

	ingestevents "github.com/BarrettBr/eecs-582-capstone/internal/ingest/events"
	ingestvalidation "github.com/BarrettBr/eecs-582-capstone/internal/ingest/validation"
)

func TestRuleEngineValidateUsesRecordEvent(t *testing.T) {
	specPath := filepath.Join(t.TempDir(), "validation_rules.json")
	spec := `{
		"required_fields": ["timestamp", "temperature"],
		"bounds": {
			"temperature": {
				"max": 75.0
			}
		},
		"anomaly_rules": [
			{
				"field": "temperature",
				"op": ">",
				"value": 70.0,
				"label": "temperature_high"
			}
		]
	}`
	if err := os.WriteFile(specPath, []byte(spec), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	engine, err := ingestvalidation.NewRuleEngine(specPath)
	if err != nil {
		t.Fatalf("NewRuleEngine() error = %v", err)
	}

	valid := engine.Validate(ingestevents.TempSample{
		Timestamp:   "2026-02-28T12:00:00Z",
		Temperature: 72.0,
	})
	if !valid.Valid {
		t.Fatalf("expected valid sample, got errors: %v", valid.Errors)
	}
	if len(valid.Anomalies) != 1 || valid.Anomalies[0] != "temperature_high" {
		t.Fatalf("expected temperature_high anomaly, got %v", valid.Anomalies)
	}

	invalid := engine.Validate(ingestevents.TempSample{})
	if invalid.Valid {
		t.Fatalf("expected invalid sample")
	}
	if len(invalid.Errors) == 0 {
		t.Fatalf("expected validation errors")
	}
}
