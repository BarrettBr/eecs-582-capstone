package events_test

/*
Name: ingest/internal/ingest/events/events_test.go
Description: Tests event payload helpers and record event behavior for normalized ingest event types.
Programmer: Barrett Brown
Date Created: 2026-03-14
Dates Revised: 2026-03-14
Revision History:
- 2026-03-14, Barrett Brown: Added event package coverage after removing the old root ingest compatibility layer.
Preconditions:
- Event helper methods are available through the events package.
Acceptable Input Values/Types:
- TempSample and ValveSample values with basic field data and anomaly labels.
Unacceptable Input Values/Types:
- No special invalid inputs are required for these tests.
Postconditions:
- Confirms event helpers expose stable routing, payload, and defensive anomaly-copy behavior.
Return Values/Types:
- Test functions return no value.
Error/Exception Conditions:
- Unexpected mutation of original anomaly slices.
Side Effects:
- No external side effects.
Invariants:
- AnomalyLabels should return a copied slice.
Known Faults:
- Does not verify every field returned by ValidationValue.
*/

import (
	"testing"

	ingestevents "github.com/BarrettBr/eecs-582-capstone/internal/ingest/events"
)

func TestTempSampleImplementsRecordEvent(t *testing.T) {
	sample := ingestevents.TempSample{
		ID:           7,
		Timestamp:    "2026-02-28T12:00:00Z",
		SensorType:   "temperature_control_system",
		SensorNumber: 3,
		FanOn:        true,
		Temperature:  72.5,
		HeaterPower:  55.0,
		Anomalies:    []string{"temperature_high"},
	}

	if got := sample.EventType(); got != "temperature" {
		t.Fatalf("EventType() = %q, want %q", got, "temperature")
	}
	if got, ok := sample.ValidationValue("temperature"); !ok || got != sample.Temperature {
		t.Fatalf("ValidationValue(temperature) = (%v, %v), want (%v, true)", got, ok, sample.Temperature)
	}

	payload, ok := sample.Payload().(ingestevents.TempSample)
	if !ok {
		t.Fatalf("Payload() type assertion failed")
	}
	if payload.Temperature != sample.Temperature {
		t.Fatalf("Payload().Temperature = %v, want %v", payload.Temperature, sample.Temperature)
	}

	copyLabels := sample.AnomalyLabels()
	copyLabels[0] = "mutated"
	if sample.Anomalies[0] != "temperature_high" {
		t.Fatalf("AnomalyLabels() did not return a defensive copy")
	}
}

func TestValveSampleImplementsRecordEvent(t *testing.T) {
	sample := ingestevents.ValveSample{
		ID:          11,
		Timestamp:   "2026-03-14T12:00:00Z",
		SensorType:  "valve_control_system",
		ValveNumber: 2,
		IsOpen:      true,
		FlowRate:    14.25,
		Anomalies:   []string{"flow_high"},
	}

	if got := sample.EventType(); got != "valve" {
		t.Fatalf("EventType() = %q, want %q", got, "valve")
	}
	if got, ok := sample.ValidationValue("flow_rate"); !ok || got != sample.FlowRate {
		t.Fatalf("ValidationValue(flow_rate) = (%v, %v), want (%v, true)", got, ok, sample.FlowRate)
	}

	payload, ok := sample.Payload().(ingestevents.ValveSample)
	if !ok {
		t.Fatalf("Payload() type assertion failed")
	}
	if payload.ValveNumber != sample.ValveNumber {
		t.Fatalf("Payload().ValveNumber = %v, want %v", payload.ValveNumber, sample.ValveNumber)
	}

	copyLabels := sample.AnomalyLabels()
	copyLabels[0] = "mutated"
	if sample.Anomalies[0] != "flow_high" {
		t.Fatalf("AnomalyLabels() did not return a defensive copy")
	}
}
