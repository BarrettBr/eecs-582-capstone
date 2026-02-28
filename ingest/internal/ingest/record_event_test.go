package ingest_test

/*
Name: ingest/internal/ingest/record_event_test.go
Description: External test for TempSample record event methods and slice copy behavior.
Programmer: Barrett Brown
Date Created: 2026-02-28
Dates Revised: 2026-02-28
Revision History:
- 2026-02-28, Barrett Brown: Created external record event coverage test.
Preconditions:
- TempSample methods are available through the ingest package.
Acceptable Input Values/Types:
- TempSample values with basic temperature data and anomaly labels.
Unacceptable Input Values/Types:
- No special invalid inputs are required for this test.
Postconditions:
- Confirms TempSample exposes the expected record event behavior.
Return Values/Types:
- Test functions return no value.
Error/Exception Conditions:
- Unexpected mutation of original anomaly slices.
Side Effects:
- No external side effects.
Invariants:
- AnomalyLabels should return a copied slice.
Known Faults:
- Does not verify every field returned by ToRecord.
*/

import (
	"testing"

	"github.com/BarrettBr/eecs-582-capstone/internal/ingest"
)

func TestTempSampleImplementsRecordEvent(t *testing.T) {
	sample := ingest.TempSample{
		ID:           7,
		Timestamp:    "2026-02-28T12:00:00Z",
		SensorType:   "temperature_control_system",
		SensorNumber: 3,
		FanOn:        true,
		Temperature:  72.5,
		HeaterPower:  55.0,
		Anomalies:    []string{"temperature_high"},
	}

	record := sample.ToRecord()
	if got := sample.EventType(); got != "temperature" {
		t.Fatalf("EventType() = %q, want %q", got, "temperature")
	}
	if got := record["temperature"]; got != sample.Temperature {
		t.Fatalf("ToRecord()[temperature] = %v, want %v", got, sample.Temperature)
	}

	copyLabels := sample.AnomalyLabels()
	copyLabels[0] = "mutated"
	if sample.Anomalies[0] != "temperature_high" {
		t.Fatalf("AnomalyLabels() did not return a defensive copy")
	}
}
