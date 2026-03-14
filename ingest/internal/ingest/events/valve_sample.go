package events

/*
Name: ingest/internal/ingest/events/valve_sample.go
Description: Defines the normalized valve event shape shared by ingest validation, storage, and streaming paths.
Programmer: Barrett Brown
Date Created: 2026-03-01
Dates Revised: 2026-03-14
Revision History:
- 2026-03-01, Barrett Brown: Added base valve sample logic during simulator and ML generalization work.
- 2026-03-14, Barrett Brown: Replaced ToRecord map building with direct validation field accessors.
- 2026-03-14, Barrett Brown: Moved the valve sample into the events package during the ingest file structure refactor.
Preconditions:
- Valve samples are populated from simulator or Modbus normalization.
Acceptable Input Values/Types:
- Valve sample fields with timestamps, valve state, and optional anomaly labels.
Unacceptable Input Values/Types:
- None in this file directly.
Postconditions:
- Provides the normalized valve event type used across ingest flows.
Return Values/Types:
- Exported struct and helper methods.
Error/Exception Conditions:
- None in this file directly.
Side Effects:
- Helper methods copy anomaly label slices before returning them.
Invariants:
- EventType always returns valve for this type.
Known Faults:
- Validation field access still uses string switching per field lookup.
*/

// ValveSample is the canonical normalized valve record emitted from source adapters.
type ValveSample struct {
	ID          uint16   `json:"id"`
	Timestamp   string   `json:"timestamp"`
	TimestampMs int64    `json:"timestamp_ms"`
	SensorType  string   `json:"sensor_type"`
	ValveNumber uint16   `json:"valve_number"`
	IsOpen      bool     `json:"is_open"`
	FlowRate    float64  `json:"flow_rate"`
	Anomalies   []string `json:"anomalies,omitempty"`
}

func (s ValveSample) EventType() string {
	return "valve"
}

func (s ValveSample) Payload() any {
	return s
}

func (s ValveSample) AnomalyLabels() []string {
	return append([]string(nil), s.Anomalies...)
}

func (s ValveSample) ValidationValue(field string) (any, bool) {
	switch field {
	case "id":
		return s.ID, true
	case "timestamp":
		return s.Timestamp, true
	case "timestamp_ms":
		return s.TimestampMs, true
	case "sensor_type":
		return s.SensorType, true
	case "valve_number":
		return s.ValveNumber, true
	case "is_open":
		return s.IsOpen, true
	case "flow_rate":
		return s.FlowRate, true
	case "anomalies":
		return s.Anomalies, true
	default:
		return nil, false
	}
}
