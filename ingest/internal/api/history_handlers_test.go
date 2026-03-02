package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestHistoryEventsOldestFirstPagination(t *testing.T) {
	db := openTestDB(t)
	seedTemperatureHistory(t, db)

	api := NewHistoryAPI(db)
	mux := http.NewServeMux()
	api.Register(func(path string, handler http.HandlerFunc) {
		mux.HandleFunc(path, handler)
	})

	firstReq := httptest.NewRequest(http.MethodGet, "/api/history/events?event_type=temperature&limit=2", nil)
	firstResp := httptest.NewRecorder()
	mux.ServeHTTP(firstResp, firstReq)
	if firstResp.Code != http.StatusOK {
		t.Fatalf("first page status = %d, want %d", firstResp.Code, http.StatusOK)
	}

	var firstPage struct {
		EventType  string            `json:"event_type"`
		Limit      int               `json:"limit"`
		NextCursor *string           `json:"next_cursor"`
		Items      []tempHistoryItem `json:"items"`
	}
	if err := json.Unmarshal(firstResp.Body.Bytes(), &firstPage); err != nil {
		t.Fatalf("decode first page: %v", err)
	}
	if len(firstPage.Items) != 2 {
		t.Fatalf("first page items = %d, want 2", len(firstPage.Items))
	}
	if firstPage.Items[0].ID != 1 || firstPage.Items[1].ID != 2 {
		t.Fatalf("first page ids = [%d %d], want [1 2]", firstPage.Items[0].ID, firstPage.Items[1].ID)
	}
	if firstPage.NextCursor == nil || *firstPage.NextCursor == "" {
		t.Fatalf("first page next_cursor empty, want non-empty")
	}
	if got := firstPage.Items[1].Anomalies; len(got) != 1 || got[0] != "too_hot" {
		t.Fatalf("first page second anomalies = %v, want [too_hot]", got)
	}

	// Insert a newer row after page 1 is fetched. Oldest-first cursor paging should not shift page 2.
	newerTimestamp := time.Now().UTC().Add(3 * time.Second).Format(time.RFC3339Nano)
	if _, err := db.Exec(`
INSERT INTO temp_samples (id, timestamp, sensor_type, sensor_number, fan_on, temperature, heater_power)
VALUES (4, ?, 'temperature_control_system', 4, 1, 76.5, 0.80)
`, newerTimestamp); err != nil {
		t.Fatalf("insert newer row: %v", err)
	}

	secondReq := httptest.NewRequest(http.MethodGet, "/api/history/events?event_type=temperature&limit=2&cursor="+*firstPage.NextCursor, nil)
	secondResp := httptest.NewRecorder()
	mux.ServeHTTP(secondResp, secondReq)
	if secondResp.Code != http.StatusOK {
		t.Fatalf("second page status = %d, want %d", secondResp.Code, http.StatusOK)
	}

	var secondPage struct {
		Items []tempHistoryItem `json:"items"`
	}
	if err := json.Unmarshal(secondResp.Body.Bytes(), &secondPage); err != nil {
		t.Fatalf("decode second page: %v", err)
	}
	if len(secondPage.Items) != 2 {
		t.Fatalf("second page items = %d, want 2", len(secondPage.Items))
	}
	if secondPage.Items[0].ID != 3 || secondPage.Items[1].ID != 4 {
		t.Fatalf("second page ids = [%d %d], want [3 4]", secondPage.Items[0].ID, secondPage.Items[1].ID)
	}
}

func TestHistoryRatesReturnsBuckets(t *testing.T) {
	db := openTestDB(t)
	seedTemperatureHistory(t, db)

	api := NewHistoryAPI(db)
	mux := http.NewServeMux()
	api.Register(func(path string, handler http.HandlerFunc) {
		mux.HandleFunc(path, handler)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/history/rates?event_type=temperature&window=1h&bucket=1m", nil)
	resp := httptest.NewRecorder()
	mux.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("rates status = %d, want %d body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}

	var payload ratesResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode rates: %v", err)
	}
	if payload.EventType != "temperature" {
		t.Fatalf("event_type = %q, want temperature", payload.EventType)
	}
	if payload.TotalCount != 3 {
		t.Fatalf("total_count = %d, want 3", payload.TotalCount)
	}
	if len(payload.Series) == 0 {
		t.Fatalf("series empty, want at least one bucket")
	}
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	schema := []string{
		`CREATE TABLE temp_samples (
			id INTEGER PRIMARY KEY,
			timestamp TEXT NOT NULL,
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
			timestamp TEXT NOT NULL,
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
	for _, stmt := range schema {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("create schema: %v", err)
		}
	}

	return db
}

func seedTemperatureHistory(t *testing.T, db *sql.DB) {
	t.Helper()

	base := time.Now().UTC().Add(-3 * time.Second)
	events := []struct {
		id          int
		timestamp   string
		sensor      int
		fanOn       bool
		temp        float64
		heaterPower float64
	}{
		{1, base.Format(time.RFC3339Nano), 1, true, 72.1, 0.55},
		{2, base.Add(time.Second).Format(time.RFC3339Nano), 2, false, 73.2, 0.45},
		{3, base.Add(2 * time.Second).Format(time.RFC3339Nano), 3, true, 74.3, 0.65},
	}
	for _, event := range events {
		if _, err := db.Exec(
			`INSERT INTO temp_samples (id, timestamp, sensor_type, sensor_number, fan_on, temperature, heater_power)
			VALUES (?, ?, 'temperature_control_system', ?, ?, ?, ?);`,
			event.id,
			event.timestamp,
			event.sensor,
			event.fanOn,
			event.temp,
			event.heaterPower,
		); err != nil {
			t.Fatalf("seed temperature history: %v", err)
		}
	}
	if _, err := db.Exec(
		`INSERT INTO temp_sample_anomalies (id, temp_sample_id, anomaly_label, created_at)
		VALUES (1, 2, 'too_hot', ?);`,
		events[1].timestamp,
	); err != nil {
		t.Fatalf("seed temperature anomaly: %v", err)
	}
}
