package ingest

type ValveSample struct {
	ID          uint16   `json:"id"`
	Timestamp   string   `json:"timestamp"`
	SensorType  string   `json:"sensor_type"`
	ValveNumber uint16   `json:"valve_number"`
	IsOpen      bool     `json:"is_open"`
	FlowRate    float64  `json:"flow_rate"`
	Anomalies   []string `json:"anomalies,omitempty"`
}

func (s ValveSample) ToRecord() map[string]any {
	return map[string]any{
		"id":           s.ID,
		"timestamp":    s.Timestamp,
		"sensor_type":  s.SensorType,
		"valve_number": s.ValveNumber,
		"is_open":      s.IsOpen,
		"flow_rate":    s.FlowRate,
		"anomalies":    append([]string(nil), s.Anomalies...),
	}
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
