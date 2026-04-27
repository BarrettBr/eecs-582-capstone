-- name: GetTempHistorySinceUnix :many
SELECT
    ts.id,
    ts.service_name,
    ts.machine_id,
    ts.timestamp,
    ts.sensor_type,
    ts.sensor_number,
    ts.fan_on,
    ts.temperature,
    ts.heater_power,
    CAST(COALESCE(COUNT(ta.id), 0) AS INTEGER) AS alert_count,
    CAST(COALESCE(GROUP_CONCAT(ta.anomaly_label), '') AS TEXT) AS anomaly_labels
FROM temp_samples ts
LEFT JOIN temp_sample_anomalies ta ON ta.temp_sample_id = ts.id
WHERE ts.timestamp >= sqlc.arg(since_unix)
GROUP BY ts.id
ORDER BY ts.timestamp DESC;

-- name: GetValveHistorySinceUnix :many
SELECT
    vs.id,
    vs.service_name,
    vs.machine_id,
    vs.timestamp,
    vs.sensor_type,
    vs.valve_number,
    vs.is_open,
    vs.flow_rate,
    CAST(COALESCE(COUNT(va.id), 0) AS INTEGER) AS alert_count,
    CAST(COALESCE(GROUP_CONCAT(va.anomaly_label), '') AS TEXT) AS anomaly_labels
FROM valve_samples vs
LEFT JOIN valve_sample_anomalies va ON va.valve_sample_id = vs.id
WHERE vs.timestamp >= sqlc.arg(since_unix)
GROUP BY vs.id
ORDER BY vs.timestamp DESC;

-- name: GetReportSummaryBetweenUnix :one
WITH
    temp_filtered AS (
        SELECT ts.id, ts.temperature
        FROM temp_samples ts
        WHERE ts.timestamp >= sqlc.arg(since_unix)
    ),
    valve_filtered AS (
        SELECT vs.id, vs.timestamp, CASE WHEN vs.is_open THEN 1.0 ELSE 0.0 END AS open_rate, vs.is_open
        FROM valve_samples vs
        WHERE vs.timestamp >= sqlc.arg(since_unix)
    ),
    valve_open_intervals AS (
        SELECT
            timestamp AS ts,
            is_open,
            LEAD(timestamp, 1, CAST(sqlc.arg(now_unix) AS INTEGER)) OVER (ORDER BY timestamp) AS next_ts
        FROM valve_filtered
    ),
    temp_alerts AS (
        SELECT COUNT(*) AS count
        FROM temp_sample_anomalies ta
        JOIN temp_samples ts ON ts.id = ta.temp_sample_id
        WHERE ts.timestamp >= sqlc.arg(since_unix)
    ),
    valve_alerts AS (
        SELECT COUNT(*) AS count
        FROM valve_sample_anomalies va
        JOIN valve_samples vs ON vs.id = va.valve_sample_id
        WHERE vs.timestamp >= sqlc.arg(since_unix)
    )
SELECT
    CAST(COALESCE((SELECT AVG(temperature) FROM temp_filtered), 0) AS REAL) AS avg_temp,
    CAST(COALESCE((SELECT MIN(temperature) FROM temp_filtered), 0) AS REAL) AS min_temp,
    CAST(COALESCE((SELECT MAX(temperature) FROM temp_filtered), 0) AS REAL) AS max_temp,
    CAST(COALESCE((SELECT AVG(open_rate) * 100.0 FROM valve_filtered), 0) AS REAL) AS avg_valve_open_rate,
    CAST(COALESCE((SELECT MIN(open_rate) * 100.0 FROM valve_filtered), 0) AS REAL) AS min_valve_open_rate,
    CAST(COALESCE((SELECT MAX(open_rate) * 100.0 FROM valve_filtered), 0) AS REAL) AS max_valve_open_rate,
    CAST((SELECT count FROM temp_alerts) + (SELECT count FROM valve_alerts) AS INTEGER) AS total_alerts,
    CAST((SELECT count FROM temp_alerts) AS INTEGER) AS temp_anomalies,
    CAST((SELECT count FROM valve_alerts) AS INTEGER) AS valve_anomalies,
    CAST(COALESCE((
        SELECT SUM(CASE WHEN is_open THEN MAX(0, next_ts - ts) ELSE 0 END)
        FROM valve_open_intervals
    ), 0) AS INTEGER) AS valve_open_duration_seconds;

-- name: GetTempMachineHistorySinceUnix :many
SELECT
    ts.timestamp,
    CAST(AVG(ts.temperature) AS REAL) AS avg_value,
    CAST(COALESCE(COUNT(ta.id), 0) AS INTEGER) AS alert_count
FROM temp_samples ts
LEFT JOIN temp_sample_anomalies ta ON ta.temp_sample_id = ts.id
WHERE ts.service_name = sqlc.arg(service_name)
  AND ts.machine_id = sqlc.arg(machine_id)
  AND ts.timestamp >= sqlc.arg(since_unix)
GROUP BY ts.timestamp
ORDER BY ts.timestamp ASC;

-- name: GetValveMachineHistorySinceUnix :many
SELECT
    vs.timestamp,
    CAST(AVG(vs.flow_rate) AS REAL) AS avg_value,
    CAST(COALESCE(COUNT(va.id), 0) AS INTEGER) AS alert_count
FROM valve_samples vs
LEFT JOIN valve_sample_anomalies va ON va.valve_sample_id = vs.id
WHERE vs.service_name = sqlc.arg(service_name)
  AND vs.machine_id = sqlc.arg(machine_id)
  AND vs.timestamp >= sqlc.arg(since_unix)
GROUP BY vs.timestamp
ORDER BY vs.timestamp ASC;
