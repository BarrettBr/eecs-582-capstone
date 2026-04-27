-- +goose Up
CREATE TABLE valve_samples (
    id INTEGER PRIMARY KEY,
    service_name TEXT NOT NULL,
    machine_id TEXT NOT NULL,
    timestamp INTEGER NOT NULL,
    sensor_type TEXT NOT NULL,
    valve_number INTEGER NOT NULL,
    is_open BOOLEAN NOT NULL,
    flow_rate REAL NOT NULL
);
CREATE INDEX idx_valve_samples_timestamp ON valve_samples(timestamp);
CREATE INDEX idx_valve_samples_service_machine_time ON valve_samples(service_name, machine_id, timestamp);

CREATE TABLE valve_sample_anomalies (
    id INTEGER PRIMARY KEY,
    valve_sample_id INTEGER NOT NULL,
    anomaly_label TEXT NOT NULL,
    created_at TEXT NOT NULL,
    FOREIGN KEY (valve_sample_id) REFERENCES valve_samples(id) ON DELETE CASCADE
);
CREATE INDEX idx_valve_sample_anomalies_valve_sample_id ON valve_sample_anomalies(valve_sample_id);

-- +goose Down
DROP TABLE valve_sample_anomalies;
DROP TABLE valve_samples;
