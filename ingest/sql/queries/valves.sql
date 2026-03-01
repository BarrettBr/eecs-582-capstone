-- name: InsertValveSample :execresult
INSERT INTO valve_samples (
    timestamp, sensor_type, valve_number, is_open, flow_rate
) VALUES (?, ?, ?, ?, ?);

-- name: InsertValveSampleAnomaly :exec
INSERT INTO valve_sample_anomalies (
    valve_sample_id, anomaly_label, created_at
) VALUES (?, ?, ?);

-- name: GetValveSampleByID :one
SELECT id, timestamp, sensor_type, valve_number, is_open, flow_rate
FROM valve_samples
WHERE id = ?;

-- name: GetAllValveSamples :many
SELECT id, timestamp, sensor_type, valve_number, is_open, flow_rate
FROM valve_samples;
