package ingest

/*
Name: ingest/internal/ingest/TempSample.go
Description: Defines the sample data shape used by ingest and a helper to get UTC time.
Programmer: Barrett Brown
Date Created: 2026-02-01
Dates Revised: 2026-02-28
Revision History:
- 2026-02-01, Barrett Brown: Created File and filled out
- 2026-02-15, Barrett Brown: Added standardized prologue documentation block.
- 2026-02-28, Barrett Brown: Added more generic toRecord method and other helper functions for use with generic data
Preconditions:
- System clock is available for timestamp generation.
Acceptable Input Values/Types:
- TempSample field types exactly as declared in the struct.
- Anomalies as a []string list.
Unacceptable Input Values/Types:
- Wrong semantic values are still possible even if types are correct.
Postconditions:
- TempSample instances represent one normalized sensor sample.
- nowUTC returns current UTC timestamp string.
Return Values/Types:
- nowUTC: string timestamp in RFC3339Nano format.
Error/Exception Conditions:
- No explicit error return paths in this file.
Side Effects:
- Reads current system time when nowUTC is called.
Invariants:
- TempSample JSON field names stay consistent.
Known Faults:
- Value validation (like min/max ranges) is handled outside this file.
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

// ToRecord returns the validation-facing normalized field map for a temperature sample.
func (s TempSample) ToRecord() map[string]any {
	return map[string]any{
		"id":            s.ID,
		"timestamp":     s.Timestamp,
		"sensor_type":   s.SensorType,
		"sensor_number": s.SensorNumber,
		"fan_on":        s.FanOn,
		"temperature":   s.Temperature,
		"heater_power":  s.HeaterPower,
		"anomalies":     append([]string(nil), s.Anomalies...),
	}
}

// EventType returns the routing key for temperature samples.
func (s TempSample) EventType() string {
	_ = s
	return "temperature"
}

// Payload returns the concrete transport payload for this sample.
func (s TempSample) Payload() any {
	return s
}

// AnomalyLabels returns a copy of the sample anomaly labels. Used as a helper function
func (s TempSample) AnomalyLabels() []string {
	return append([]string(nil), s.Anomalies...)
}

// description: Produces the current UTC timestamp in RFC3339Nano format.
// input: None.
// output: Returns a timestamp string suitable for sample serialization.
func nowUTC() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}
