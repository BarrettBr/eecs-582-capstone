package ingest

import "time"

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

func nowUTC() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}
