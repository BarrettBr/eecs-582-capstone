-- name: InsertTempSample :exec
INSERT INTO temp_samples (
    timestamp, sensor_type, sensor_number, fan_on, temperature, heater_power, anomalies
) VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: GetTempSampleByID :one
SELECT id, timestamp, sensor_type, sensor_number, fan_on, temperature, heater_power, anomalies
FROM temp_samples
WHERE id = ?;

-- name: GetAllTempSamples :many
SELECT id, timestamp, sensor_type, sensor_number, fan_on, temperature, heater_power, anomalies
FROM temp_samples;
