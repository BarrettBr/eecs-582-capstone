package ingest

/*
Name: ingest/internal/ingest/modbus_loop_internal_test.go
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
- dataNormalizer should only accept the expected payload size.
Known Faults:
- Does not cover every field combination that could be encoded in the payload.
*/

import (
	"encoding/binary"
	"testing"
	"time"
)

func TestDataNormalizerPopulatesTempSample(t *testing.T) {
	var payload [12]byte
	binary.BigEndian.PutUint16(payload[0:2], 9)
	binary.BigEndian.PutUint16(payload[2:4], 1)
	binary.BigEndian.PutUint16(payload[4:6], 4)
	binary.BigEndian.PutUint16(payload[6:8], 1)
	binary.BigEndian.PutUint16(payload[8:10], 725)
	binary.BigEndian.PutUint16(payload[10:12], 5050)

	loop := &ModbusLoop{}
	if err := loop.dataNormalizer(payload[:]); err != nil {
		t.Fatalf("dataNormalizer() error = %v", err)
	}

	if loop.sample.ID != 9 {
		t.Fatalf("sample.ID = %d, want 9", loop.sample.ID)
	}
	if loop.sample.SensorType != "temperature_control_system" {
		t.Fatalf("sample.SensorType = %q", loop.sample.SensorType)
	}
	if loop.sample.Temperature != 72.5 {
		t.Fatalf("sample.Temperature = %v, want 72.5", loop.sample.Temperature)
	}
	if loop.sample.HeaterPower != 50.5 {
		t.Fatalf("sample.HeaterPower = %v, want 50.5", loop.sample.HeaterPower)
	}
	if _, err := time.Parse(time.RFC3339Nano, loop.sample.Timestamp); err != nil {
		t.Fatalf("sample.Timestamp parse error = %v", err)
	}
}

func TestDataNormalizerRejectsBadPayloadAndSensorType(t *testing.T) {
	loop := &ModbusLoop{}
	if err := loop.dataNormalizer([]byte{1, 2}); err == nil {
		t.Fatalf("dataNormalizer() short payload error = nil, want non-nil")
	}

	var payload [12]byte
	binary.BigEndian.PutUint16(payload[2:4], 99)
	if err := loop.dataNormalizer(payload[:]); err == nil {
		t.Fatalf("dataNormalizer() unsupported sensor type error = nil, want non-nil")
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
