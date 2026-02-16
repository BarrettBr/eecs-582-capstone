package ingest

/*
Name: ingest/internal/ingest/TempSample.go
Description: Defines the normalized ingest sample schema and timestamp helper used across the ingest pipeline.
Programmer: Barrett Brown
Date Created: 2026-02-01
Dates Revised: 2026-02-15
Revision History:
- 2026-02-01, Barrett Brown: Created File and filled out
- 2026-02-15, Barrett Brown: Added standardized prologue documentation block.
Preconditions:
- System clock is available for timestamp generation.
Acceptable Input Values/Types:
- TempSample fields accept native Go scalar types as defined in the struct.
- Anomalies accepts []string labels.
Unacceptable Input Values/Types:
- Out-of-band semantic values (for example invalid sensor domain values) are not prevented by struct typing alone.
Postconditions:
- TempSample instances represent one normalized sensor sample.
- nowUTC returns current UTC timestamp string.
Return Values/Types:
- nowUTC: string (RFC3339Nano UTC timestamp).
Error/Exception Conditions:
- No explicit error return paths in this file.
Side Effects:
- Reads current system time when nowUTC is called.
Invariants:
- TempSample field types and JSON tags remain stable for serialization contracts.
Known Faults:
- Struct-level constraints (such as valid ranges) are enforced externally, not in this file.
*/

import "time"

// description: Canonical normalized sample emitted from each Modbus poll tick.
// input: Populated by dataNormalizer and validation in ModbusLoop.
// output: Used for fan-out delivery to ML, SQL, and websocket sinks.
type TempSample struct {
	ID           uint16   `json:"id"`
	Timestamp    string   `json:"timestamp"`
	SensorType   string   `json:"sensor_type"`
	SensorNumber uint16   `json:"sensor_number"`
	FanOn        bool     `json:"fan_on"`
	Temperature  float64  `json:"temperature"`
	HeaterPower  float64  `json:"heater_power"`
	Anomalies    []string `json:"anomalies,omitempty"`
}

// description: Produces the current UTC timestamp in RFC3339Nano format.
// input: None.
// output: Returns a timestamp string suitable for sample serialization.
func nowUTC() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}
