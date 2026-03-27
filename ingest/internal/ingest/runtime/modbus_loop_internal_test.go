package runtime

/*
Name: ingest/internal/ingest/runtime/modbus_loop_internal_test.go
Description: Tests Modbus payload normalization and sensor type mapping helpers.
Programmer: Barrett Brown
Date Created: 2026-02-28
Dates Revised: 2026-02-28
Revision History:
- 2026-02-28, Barrett Brown: Created internal tests for data normalization logic.
Preconditions:
- Test byte payloads match the expected 12 byte register format when valid.
Acceptable Input Values/Types:
- Valid 12 byte Modbus payloads.
- Invalid payload lengths and unsupported sensor codes.
Unacceptable Input Values/Types:
- Nil loops are not used in these tests.
Postconditions:
- Confirms valid payloads populate TempSample fields and invalid inputs error.
Return Values/Types:
- Test functions return no value.
Error/Exception Conditions:
- Unexpected missing errors for bad payload sizes or unsupported sensor types.
Side Effects:
- Reads current system time through normalization timestamp generation.
Invariants:
- normalizeTemperatureRegisters should only accept the expected payload size.
Known Faults:
- Does not cover every field combination that could be encoded in the payload.
*/

import (
	"encoding/binary"
	"math/rand"
	"testing"
	"time"

	"github.com/BarrettBr/eecs-582-capstone/internal/config"
	ingestevents "github.com/BarrettBr/eecs-582-capstone/internal/ingest/events"
)

func TestDataNormalizerPopulatesTempSample(t *testing.T) {
	var payload [12]byte
	binary.BigEndian.PutUint16(payload[0:2], 9)
	binary.BigEndian.PutUint16(payload[2:4], 1)
	binary.BigEndian.PutUint16(payload[4:6], 4)
	binary.BigEndian.PutUint16(payload[6:8], 1)
	binary.BigEndian.PutUint16(payload[8:10], 725)
	binary.BigEndian.PutUint16(payload[10:12], 5050)

	sample, err := normalizeTemperatureRegisters(payload[:])
	if err != nil {
		t.Fatalf("normalizeTemperatureRegisters() error = %v", err)
	}

	if sample.ID != 9 {
		t.Fatalf("sample.ID = %d, want 9", sample.ID)
	}
	if sample.SensorType != "temperature_control_system" {
		t.Fatalf("sample.SensorType = %q", sample.SensorType)
	}
	if sample.Temperature != 72.5 {
		t.Fatalf("sample.Temperature = %v, want 72.5", sample.Temperature)
	}
	if sample.HeaterPower != 50.5 {
		t.Fatalf("sample.HeaterPower = %v, want 50.5", sample.HeaterPower)
	}
	if _, err := time.Parse(time.RFC3339Nano, sample.Timestamp); err != nil {
		t.Fatalf("sample.Timestamp parse error = %v", err)
	}
}

func TestDataNormalizerRejectsBadPayloadAndSensorType(t *testing.T) {
	if _, err := normalizeTemperatureRegisters([]byte{1, 2}); err == nil {
		t.Fatalf("normalizeTemperatureRegisters() short payload error = nil, want non-nil")
	}

	var payload [12]byte
	binary.BigEndian.PutUint16(payload[2:4], 99)
	if _, err := normalizeTemperatureRegisters(payload[:]); err == nil {
		t.Fatalf("normalizeTemperatureRegisters() unsupported sensor type error = nil, want non-nil")
	}
}

func TestSensorTypeFromCode(t *testing.T) {
	got, err := sensorTypeFromCode(1)
	if err != nil {
		t.Fatalf("sensorTypeFromCode() error = %v", err)
	}
	if got != "temperature_control_system" {
		t.Fatalf("sensorTypeFromCode() = %q, want %q", got, "temperature_control_system")
	}

	if _, err := sensorTypeFromCode(2); err == nil {
		t.Fatalf("sensorTypeFromCode() invalid code error = nil, want non-nil")
	}
}

func TestNextSimulatedRecordReturnsValveSample(t *testing.T) {
	runner := &sourceRunner{
		definition: config.SourceDefinition{
			Name:      "valve_dev",
			EventType: "valve",
			Mode:      "simulator",
		},
		rng:          rand.New(rand.NewSource(84)),
		simValveFlow: 145.0,
		simValveOpen: true,
	}

	record, err := runner.nextSimulatedRecord()
	if err != nil {
		t.Fatalf("nextSimulatedRecord() error = %v", err)
	}

	sample, ok := record.(ingestevents.ValveSample)
	if !ok {
		t.Fatalf("record type = %T, want ingestevents.ValveSample", record)
	}
	if sample.EventType() != "valve" {
		t.Fatalf("sample.EventType() = %q, want %q", sample.EventType(), "valve")
	}
	if sample.SensorType != "valve_control_system" {
		t.Fatalf("sample.SensorType = %q, want %q", sample.SensorType, "valve_control_system")
	}
	if sample.ValveNumber != 1 {
		t.Fatalf("sample.ValveNumber = %d, want 1", sample.ValveNumber)
	}
	if sample.FlowRate < 0 || sample.FlowRate > 1000 {
		t.Fatalf("sample.FlowRate = %v, want value within validation bounds", sample.FlowRate)
	}
	if _, err := time.Parse(time.RFC3339Nano, sample.Timestamp); err != nil {
		t.Fatalf("sample.Timestamp parse error = %v", err)
	}
}

func TestNextSimulatedValveRecordFaultInjectionProducesHighFlow(t *testing.T) {
	runner := &sourceRunner{
		definition: config.SourceDefinition{
			Name:      "valve_dev",
			EventType: "valve",
			Mode:      "simulator",
		},
		rng:             rand.New(rand.NewSource(84)),
		simValveFlow:    145.0,
		simValveOpen:    false,
		forceFaultTicks: 1,
	}

	record, err := runner.nextSimulatedRecord()
	if err != nil {
		t.Fatalf("nextSimulatedRecord() error = %v", err)
	}

	sample, ok := record.(ingestevents.ValveSample)
	if !ok {
		t.Fatalf("record type = %T, want ingestevents.ValveSample", record)
	}
	if !sample.IsOpen {
		t.Fatalf("sample.IsOpen = false, want true during forced fault")
	}
	if sample.FlowRate <= 500 {
		t.Fatalf("sample.FlowRate = %v, want > 500 during forced fault", sample.FlowRate)
	}
}
