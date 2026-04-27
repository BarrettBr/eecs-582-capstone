-- name: InsertTempSample :execresult
INSERT INTO temp_samples (
    service_name, machine_id, timestamp, sensor_type, sensor_number, fan_on, temperature, heater_power
) VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: InsertTempSampleAnomaly :exec
INSERT INTO temp_sample_anomalies (
    temp_sample_id, anomaly_label, created_at
) VALUES (?, ?, ?);

-- name: GetTempSampleByID :one
SELECT id, service_name, machine_id, timestamp, sensor_type, sensor_number, fan_on, temperature, heater_power
FROM temp_samples
WHERE id = ?;

-- name: GetAllTempSamples :many
SELECT id, service_name, machine_id, timestamp, sensor_type, sensor_number, fan_on, temperature, heater_power
FROM temp_samples;

-- name: GetTempSampleCount :one
SELECT COUNT(*) AS count
FROM temp_samples;
