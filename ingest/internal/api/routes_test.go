package api

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/BarrettBr/eecs-582-capstone/internal/database"
	_ "modernc.org/sqlite"
)

type muxRegistrar struct {
	mux *http.ServeMux
}

func (m muxRegistrar) RegisterHTTPHandler(pattern string, handler http.HandlerFunc) {
	m.mux.HandleFunc(pattern, handler)
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
