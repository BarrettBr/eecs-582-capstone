-- name: GetTempHistorySinceUnix :many
SELECT
    ts.id,
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
WHERE unixepoch(ts.timestamp) >= CAST(sqlc.arg(since_unix) AS INTEGER)
GROUP BY ts.id
ORDER BY unixepoch(ts.timestamp) DESC;

-- name: GetValveHistorySinceUnix :many
SELECT
    vs.id,
    vs.timestamp,
    vs.sensor_type,
    vs.valve_number,
    vs.is_open,
    vs.flow_rate,
    CAST(COALESCE(COUNT(va.id), 0) AS INTEGER) AS alert_count,
    CAST(COALESCE(GROUP_CONCAT(va.anomaly_label), '') AS TEXT) AS anomaly_labels
FROM valve_samples vs
LEFT JOIN valve_sample_anomalies va ON va.valve_sample_id = vs.id
WHERE unixepoch(vs.timestamp) >= CAST(sqlc.arg(since_unix) AS INTEGER)
GROUP BY vs.id
ORDER BY unixepoch(vs.timestamp) DESC;

-- name: GetReportSummarySinceUnix :one
WITH
    temp_filtered AS (
        SELECT ts.id, ts.temperature
        FROM temp_samples ts
        WHERE unixepoch(ts.timestamp) >= CAST(sqlc.arg(since_unix) AS INTEGER)
    ),
    valve_filtered AS (
        SELECT vs.id, CASE WHEN vs.is_open THEN 1.0 ELSE 0.0 END AS open_rate
        FROM valve_samples vs
        WHERE unixepoch(vs.timestamp) >= CAST(sqlc.arg(since_unix) AS INTEGER)
    ),
    temp_alerts AS (
        SELECT COUNT(*) AS count
        FROM temp_sample_anomalies ta
        JOIN temp_samples ts ON ts.id = ta.temp_sample_id
        WHERE unixepoch(ts.timestamp) >= CAST(sqlc.arg(since_unix) AS INTEGER)
    ),
    valve_alerts AS (
        SELECT COUNT(*) AS count
        FROM valve_sample_anomalies va
        JOIN valve_samples vs ON vs.id = va.valve_sample_id
        WHERE unixepoch(vs.timestamp) >= CAST(sqlc.arg(since_unix) AS INTEGER)
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
    CAST((SELECT count FROM valve_alerts) AS INTEGER) AS valve_anomalies;
