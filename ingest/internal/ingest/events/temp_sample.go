package events

/*
Name: ingest/internal/ingest/events/temp_sample.go
Description: Defines the normalized temperature sample shape shared by ingest validation and fanout paths.
Programmer: Barrett Brown
Date Created: 2026-02-01
Dates Revised: 2026-03-14
Revision History:
- 2026-02-01, Barrett Brown: Created file and filled out the base temperature sample structure.
- 2026-02-15, Barrett Brown: Added standardized prologue documentation block.
- 2026-02-28, Barrett Brown: Added generic record helper methods for validation and fanout use.
- 2026-03-14, Barrett Brown: Replaced ToRecord map building with direct validation field accessors.
- 2026-03-14, Barrett Brown: Moved the temperature sample into the events package during the ingest file structure refactor.
Preconditions:
- Temperature samples are populated from simulator or Modbus normalization.
Acceptable Input Values/Types:
- TempSample fields accept native Go scalar types as defined in the struct.
- Anomalies accepts []string labels.
Unacceptable Input Values/Types:
- Out-of-band semantic values are not prevented by struct typing alone.
Postconditions:
- TempSample instances represent one normalized temperature event.
Return Values/Types:
- Exported struct and helper methods.
Error/Exception Conditions:
- No explicit error return paths in this file.
Side Effects:
- None.
Invariants:
- TempSample field types and JSON tags remain stable for serialization contracts.
Known Faults:
- Struct-level domain constraints are enforced externally, not in this file.
*/

// TempSample is the canonical normalized temperature record emitted from source adapters.
type TempSample struct {
	ID           uint16   `json:"id"`
	Timestamp    string   `json:"timestamp"`
	TimestampMs  int64    `json:"timestamp_ms"`
	SensorType   string   `json:"sensor_type"`
	SensorNumber uint16   `json:"sensor_number"`
	FanOn        bool     `json:"fan_on"`
	Temperature  float64  `json:"temperature"`
	HeaterPower  float64  `json:"heater_power"`
	Anomalies    []string `json:"anomalies,omitempty"`
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

// AnomalyLabels returns a copy of the sample anomaly labels.
func (s TempSample) AnomalyLabels() []string {
	return append([]string(nil), s.Anomalies...)
}

// ValidationValue returns one named field without building an intermediate map.
func (s TempSample) ValidationValue(field string) (any, bool) {
	switch field {
	case "id":
		return s.ID, true
	case "timestamp":
		return s.Timestamp, true
	case "timestamp_ms":
		return s.TimestampMs, true
	case "sensor_type":
		return s.SensorType, true
	case "sensor_number":
		return s.SensorNumber, true
	case "fan_on":
		return s.FanOn, true
	case "temperature":
		return s.Temperature, true
	case "heater_power":
		return s.HeaterPower, true
	case "anomalies":
		return s.Anomalies, true
	default:
		return nil, false
	}
}
