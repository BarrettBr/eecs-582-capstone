package ingest

type TempSample struct {
	ID           uint16  `json:"id"`
	SensorType   string  `json:"sensor_type"`
	SensorNumber uint16  `json:"sensor_number"`
	FanOn        bool    `json:"fan_on"`
	Temperature  float64 `json:"temperature"`
	HeaterPower  float64 `json:"heater_power"`
}
