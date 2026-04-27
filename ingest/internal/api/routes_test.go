package api

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/BarrettBr/eecs-582-capstone/internal/database"
	ingestruntime "github.com/BarrettBr/eecs-582-capstone/internal/ingest/runtime"
	_ "modernc.org/sqlite"
)

type muxRegistrar struct {
	mux *http.ServeMux
}

type stubStatusProvider struct{}
type machineStatusProvider struct{}

func (m muxRegistrar) RegisterHTTPHandler(pattern string, handler http.HandlerFunc) {
	m.mux.HandleFunc(pattern, handler)
}

func (stubStatusProvider) StatusSnapshot() ingestruntime.SystemStatusSnapshot {
	return ingestruntime.SystemStatusSnapshot{
		PressureState:       ingestruntime.PressureHigh,
		SharedUnitsUsed:     2,
		SharedUnitsCapacity: 4,
	}
}

func (machineStatusProvider) StatusSnapshot() ingestruntime.SystemStatusSnapshot {
	return ingestruntime.SystemStatusSnapshot{
		Services: []ingestruntime.ServiceStatus{
			{
				Name:      "temp_dev",
				EventType: "temperature",
				Machines: []ingestruntime.MachineStatus{
					{ID: "m1", ServiceName: "temp_dev"},
				},
			},
		},
	}
}

func TestRegisterRoutesAddsPingEndpoint(t *testing.T) {
	mux := http.NewServeMux()
	registration := RegisterRoutes(muxRegistrar{mux: mux}, Config{BasePath: "/api/v1"})

	if registration.PingPath != "/api/v1/ping" {
		t.Fatalf("registration.PingPath = %q, want %q", registration.PingPath, "/api/v1/ping")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ping", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if body := rec.Body.String(); body != "{\"message\":\"pong\"}" {
		t.Fatalf("body = %q, want %q", body, "{\"message\":\"pong\"}")
	}
}

func TestRegisterRoutesPingRejectsMethod(t *testing.T) {
	mux := http.NewServeMux()
	_ = RegisterRoutes(muxRegistrar{mux: mux}, Config{BasePath: "/api/v1"})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ping", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestRegisterRoutesHistoryDispatchesToSpecificHandler(t *testing.T) {
	mux := http.NewServeMux()
	_ = RegisterRoutes(muxRegistrar{mux: mux}, Config{
		BasePath: "/api/v1",
		Queries:  openAPITestQueries(t),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/history/temp/hour", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if body := rec.Body.String(); body != "{\"selection\":{\"endpoint\":\"history\",\"kind\":\"temp\",\"window\":\"hour\",\"table\":\"temp_samples\",\"group_by\":\"minute\",\"range\":\"1h\"},\"items\":[]}" {
		t.Fatalf("body = %q, want %q", body, "{\"selection\":{\"endpoint\":\"history\",\"kind\":\"temp\",\"window\":\"hour\",\"table\":\"temp_samples\",\"group_by\":\"minute\",\"range\":\"1h\"},\"items\":[]}")
	}
}

func TestRegisterRoutesReportDispatchesToSpecificHandler(t *testing.T) {
	mux := http.NewServeMux()
	_ = RegisterRoutes(muxRegistrar{mux: mux}, Config{
		BasePath: "/api/v1",
		Queries:  openAPITestQueries(t),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/report/valve/week", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if body := rec.Body.String(); !strings.Contains(body, "\"summary\":") {
		t.Fatalf("body = %q, want report summary payload", body)
	}
}

func TestRegisterRoutesAddsIngestionStatusEndpoint(t *testing.T) {
	mux := http.NewServeMux()
	registration := RegisterRoutes(muxRegistrar{mux: mux}, Config{
		BasePath: "/api/v1",
		Status:   stubStatusProvider{},
	})

	if registration.IngestionStatusPath != "/api/v1/ingestion/status" {
		t.Fatalf("registration.IngestionStatusPath = %q, want %q", registration.IngestionStatusPath, "/api/v1/ingestion/status")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ingestion/status", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if body := rec.Body.String(); !strings.Contains(body, "\"pressure_state\":\"high\"") {
		t.Fatalf("body = %q, want pressure status payload", body)
	}
}

func TestRegisterRoutesAddsIngestionMetricsEndpoint(t *testing.T) {
	mux := http.NewServeMux()
	registration := RegisterRoutes(muxRegistrar{mux: mux}, Config{
		BasePath: "/api/v1",
		Queries:  openAPITestQueries(t),
	})

	if registration.IngestionMetricsPath != "/api/v1/ingestion/metrics" {
		t.Fatalf("registration.IngestionMetricsPath = %q, want %q", registration.IngestionMetricsPath, "/api/v1/ingestion/metrics")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ingestion/metrics", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if body := rec.Body.String(); body != "{\"temp_records\":0,\"valve_records\":0,\"total_records\":0}" {
		t.Fatalf("body = %q, want %q", body, "{\"temp_records\":0,\"valve_records\":0,\"total_records\":0}")
	}
}

func TestRegisterRoutesMachineHistoryEndpoint(t *testing.T) {
	mux := http.NewServeMux()
	queries := openAPITestQueries(t)
	nowUnix := time.Now().UTC().Unix()
	if _, err := queries.InsertTempSample(context.Background(), database.InsertTempSampleParams{
		ServiceName:  "temp_dev",
		MachineID:    "m1",
		Timestamp:    nowUnix,
		SensorType:   "temperature_control_system",
		SensorNumber: 1,
		FanOn:        false,
		Temperature:  72.4,
		HeaterPower:  0,
	}); err != nil {
		t.Fatalf("InsertTempSample() error = %v", err)
	}

	registration := RegisterRoutes(muxRegistrar{mux: mux}, Config{
		BasePath: "/api/v1",
		Queries:  queries,
		Status:   machineStatusProvider{},
	})

	if registration.MachineHistoryPath != "/api/v1/machineHistory/{machineId}" {
		t.Fatalf("registration.MachineHistoryPath = %q, want %q", registration.MachineHistoryPath, "/api/v1/machineHistory/{machineId}")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/machineHistory/m1?service_name=temp_dev&window=hour", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "\"machine_id\":\"m1\"") {
		t.Fatalf("body = %q, want machine id in payload", rec.Body.String())
	}
}

func openAPITestQueries(t *testing.T) *database.Queries {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

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
			created_at TEXT NOT NULL
		);`,
		`CREATE TABLE valve_samples (
			id INTEGER PRIMARY KEY,
			service_name TEXT NOT NULL,
			machine_id TEXT NOT NULL,
			timestamp INTEGER NOT NULL,
			sensor_type TEXT NOT NULL,
			valve_number INTEGER NOT NULL,
			is_open BOOLEAN NOT NULL,
			flow_rate REAL NOT NULL
		);`,
		`CREATE TABLE valve_sample_anomalies (
			id INTEGER PRIMARY KEY,
			valve_sample_id INTEGER NOT NULL,
			anomaly_label TEXT NOT NULL,
			created_at TEXT NOT NULL
		);`,
	}
	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("db.Exec() error = %v", err)
		}
	}

	return database.New(db)
}

func TestRegisterRoutesHistoryRejectsInvalidKind(t *testing.T) {
	mux := http.NewServeMux()
	_ = RegisterRoutes(muxRegistrar{mux: mux}, Config{BasePath: "/api/v1"})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/history/nuclear/hour", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if body := rec.Body.String(); body != "{\"error\":\"invalid history kind\"}" {
		t.Fatalf("body = %q, want %q", body, "{\"error\":\"invalid history kind\"}")
	}
}

func TestRegisterRoutesReportRejectsInvalidWindow(t *testing.T) {
	mux := http.NewServeMux()
	_ = RegisterRoutes(muxRegistrar{mux: mux}, Config{BasePath: "/api/v1"})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/report/temp/month", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if body := rec.Body.String(); body != "{\"error\":\"invalid report window\"}" {
		t.Fatalf("body = %q, want %q", body, "{\"error\":\"invalid report window\"}")
	}
}
