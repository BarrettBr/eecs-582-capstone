package runtime

/*
Name: ingest/internal/ingest/runtime/fanout_test.go
Description: Tests SQL fanout batching, anomaly persistence, and normal sample rate behavior.
Programmer: Barrett Brown
Date Created: 2026-02-28
Dates Revised: 2026-02-28
Revision History:
- 2026-02-28, Barrett Brown: Created SQL fanout tests for batch persistence coverage.
Preconditions:
- SQLite test database can be opened in memory.
- Required ingest tables can be created before running assertions.
Acceptable Input Values/Types:
- TempSample events with and without anomalies.
- Positive SQL normal sample rate values.
Unacceptable Input Values/Types:
- Nil database handles.
- Missing required test tables.
Postconditions:
- Confirms anomalous records are always stored and normal records are sampled.
Return Values/Types:
- Test helpers return *sql.DB.
- Test functions return no value.
Error/Exception Conditions:
- SQL execution failures.
- Unexpected row counts after batch persistence.
Side Effects:
- Creates tables and writes test rows to an in memory SQLite database.
Invariants:
- Each test uses an isolated in memory database.
Known Faults:
- Does not yet cover unsupported event type behavior in the SQL batch path.
*/

import (
	"context"
	"database/sql"
	"testing"

	"github.com/BarrettBr/eecs-582-capstone/internal/database"
	ingestevents "github.com/BarrettBr/eecs-582-capstone/internal/ingest/events"
	_ "modernc.org/sqlite"
)

func TestDeliverSQLBatchStoresAllAnomaliesAndSamplesNormalEvents(t *testing.T) {
	db := openTestSQLite(t)
	createIngestTables(t, db)

	pipeline := &Pipeline{
		db:                  db,
		queries:             database.New(db),
		sqlNormalSampleRate: 2,
	}

	batch := []IngressEvent{
		{
			Record: ingestevents.TempSample{
				ServiceName:  "temp_dev",
				MachineID:    "m1",
				Timestamp:    "2026-02-28T12:00:00Z",
				SensorType:   "temperature_control_system",
				SensorNumber: 1,
				FanOn:        true,
				Temperature:  70.0,
				HeaterPower:  50.0,
			},
		},
		{
			Record: ingestevents.TempSample{
				ServiceName:  "temp_dev",
				MachineID:    "m2",
				Timestamp:    "2026-02-28T12:00:01Z",
				SensorType:   "temperature_control_system",
				SensorNumber: 2,
				FanOn:        true,
				Temperature:  90.0,
				HeaterPower:  80.0,
				Anomalies:    []string{"temperature_high"},
			},
		},
		{
			Record: ingestevents.TempSample{
				ServiceName:  "temp_dev",
				MachineID:    "m3",
				Timestamp:    "2026-02-28T12:00:02Z",
				SensorType:   "temperature_control_system",
				SensorNumber: 3,
				FanOn:        false,
				Temperature:  68.0,
				HeaterPower:  45.0,
			},
		},
	}

	if err := pipeline.deliverSQLBatch(context.Background(), batch); err != nil {
		t.Fatalf("deliverSQLBatch() error = %v", err)
	}

	var sampleCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM temp_samples").Scan(&sampleCount); err != nil {
		t.Fatalf("count temp_samples: %v", err)
	}
	if sampleCount != 2 {
		t.Fatalf("temp_samples count = %d, want 2", sampleCount)
	}

	var anomalyCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM temp_sample_anomalies").Scan(&anomalyCount); err != nil {
		t.Fatalf("count temp_sample_anomalies: %v", err)
	}
	if anomalyCount != 1 {
		t.Fatalf("temp_sample_anomalies count = %d, want 1", anomalyCount)
	}
}

func openTestSQLite(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	return db
}

func createIngestTables(t *testing.T, db *sql.DB) {
	t.Helper()

	statements := []string{
		`CREATE TABLE temp_samples (
			id INTEGER PRIMARY KEY,
			service_name TEXT NOT NULL,
			machine_id TEXT NOT NULL,
			timestamp INTEGER NOT NULL,
			sensor_type TEXT NOT NULL,
			sensor_number INTEGER NOT NULL,
			fan_on BOOLEAN NOT NULL,
			temperature REAL NOT NULL,
			heater_power REAL NOT NULL
		);`,
		`CREATE TABLE temp_sample_anomalies (
			id INTEGER PRIMARY KEY,
			temp_sample_id INTEGER NOT NULL,
			anomaly_label TEXT NOT NULL,
			created_at TEXT NOT NULL,
			FOREIGN KEY (temp_sample_id) REFERENCES temp_samples(id) ON DELETE CASCADE
		);`,
	}

	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("db.Exec() error = %v", err)
		}
	}
}
