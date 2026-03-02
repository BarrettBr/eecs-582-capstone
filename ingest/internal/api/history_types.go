package api

type bucketPoint struct {
	BucketStartMs int64 `json:"bucket_start_ms"`
	BucketEndMs   int64 `json:"bucket_end_ms"`
	Count         int64 `json:"count"`
}

type ratesResponse struct {
	EventType     string        `json:"event_type"`
	Window        string        `json:"window"`
	Bucket        string        `json:"bucket"`
	GeneratedAt   string        `json:"generated_at"`
	GeneratedAtMs int64         `json:"generated_at_ms"`
	TotalCount    int64         `json:"total_count"`
	Series        []bucketPoint `json:"series"`
}

type tempHistoryItem struct {
	ID           int64    `json:"id"`
	Timestamp    string   `json:"timestamp"`
	TimestampMs  int64    `json:"timestamp_ms"`
	SensorType   string   `json:"sensor_type"`
	SensorNumber int64    `json:"sensor_number"`
	FanOn        bool     `json:"fan_on"`
	Temperature  float64  `json:"temperature"`
	HeaterPower  float64  `json:"heater_power"`
	Anomalies    []string `json:"anomalies"`
}

type valveHistoryItem struct {
	ID          int64    `json:"id"`
	Timestamp   string   `json:"timestamp"`
	TimestampMs int64    `json:"timestamp_ms"`
	SensorType  string   `json:"sensor_type"`
	ValveNumber int64    `json:"valve_number"`
	IsOpen      bool     `json:"is_open"`
	FlowRate    float64  `json:"flow_rate"`
	Anomalies   []string `json:"anomalies"`
}

type historyResponse struct {
	EventType  string  `json:"event_type"`
	Limit      int     `json:"limit"`
	NextCursor *string `json:"next_cursor"`
	Items      any     `json:"items"`
}
