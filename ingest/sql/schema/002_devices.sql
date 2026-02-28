-- +goose Up
CREATE TABLE temp_samples (
    id INTEGER PRIMARY KEY,
    timestamp TEXT NOT NULL,
    sensor_type TEXT NOT NULL,
    sensor_number INTEGER NOT NULL,
    fan_on BOOLEAN NOT NULL,
    temperature REAL NOT NULL,
    heater_power REAL NOT NULL
);

CREATE TABLE temp_sample_anomalies (
    id INTEGER PRIMARY KEY,
    temp_sample_id INTEGER NOT NULL,
    anomaly_label TEXT NOT NULL,
    created_at TEXT NOT NULL,
    FOREIGN KEY (temp_sample_id) REFERENCES temp_samples(id) ON DELETE CASCADE
);

-- +goose Down
DROP TABLE temp_sample_anomalies;
DROP TABLE temp_samples;
