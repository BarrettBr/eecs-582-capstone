-- +goose Up
CREATE TABLE temp_samples (
    id INTEGER PRIMARY KEY,
    timestamp TEXT NOT NULL,
    sensor_type TEXT NOT NULL,
    sensor_number INTEGER NOT NULL,
    fan_on BOOLEAN NOT NULL,
    temperature REAL NOT NULL,
    heater_power REAL NOT NULL,
    anomalies TEXT
);

-- +goose Down
DROP TABLE temp_samples;
