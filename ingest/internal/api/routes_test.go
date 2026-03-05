package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
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
	_ = RegisterRoutes(muxRegistrar{mux: mux}, Config{BasePath: "/api/v1"})

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
	_ = RegisterRoutes(muxRegistrar{mux: mux}, Config{BasePath: "/api/v1"})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/report/valve/week", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if body := rec.Body.String(); body != "{\"selection\":{\"endpoint\":\"report\",\"kind\":\"valve\",\"window\":\"week\",\"table\":\"valve_samples\",\"group_by\":\"day\",\"range\":\"7d\"},\"items\":[]}" {
		t.Fatalf("body = %q, want %q", body, "{\"selection\":{\"endpoint\":\"report\",\"kind\":\"valve\",\"window\":\"week\",\"table\":\"valve_samples\",\"group_by\":\"day\",\"range\":\"7d\"},\"items\":[]}")
	}
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
