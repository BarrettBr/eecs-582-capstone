package api

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/BarrettBr/eecs-582-capstone/internal/database"
	_ "modernc.org/sqlite"
)

func TestRegisterRoutesAddsOAuthTokenEndpoint(t *testing.T) {
	mux := http.NewServeMux()
	_ = RegisterRoutes(muxRegistrar{mux: mux}, Config{BasePath: "/api/v1"})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/oauth/token", strings.NewReader("grant_type=password&username=test&password=test"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if body := rec.Body.String(); !strings.Contains(body, "\"access_token\":") || !strings.Contains(body, "\"token_type\":\"Bearer\"") {
		t.Fatalf("body = %q, want oauth token payload", body)
	}
}

func TestRegisterRoutesOAuthTokenRejectsBadCredentials(t *testing.T) {
	mux := http.NewServeMux()
	_ = RegisterRoutes(muxRegistrar{mux: mux}, Config{BasePath: "/api/v1"})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/oauth/token", strings.NewReader("grant_type=password&username=test&password=wrong"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if body := rec.Body.String(); body != "{\"error\":\"invalid_grant\",\"error_description\":\"invalid username or password\"}" {
		t.Fatalf("body = %q, want %q", body, "{\"error\":\"invalid_grant\",\"error_description\":\"invalid username or password\"}")
	}
}

func TestRegisterRoutesTokenRejectsUnsupportedGrantType(t *testing.T) {
	mux := http.NewServeMux()
	_ = RegisterRoutes(muxRegistrar{mux: mux}, Config{BasePath: "/api/v1"})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/oauth/token", strings.NewReader("grant_type=client_credentials"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if body := rec.Body.String(); body != "{\"error\":\"unsupported_grant_type\",\"error_description\":\"grant_type must be password\"}" {
		t.Fatalf("body = %q, want %q", body, "{\"error\":\"unsupported_grant_type\",\"error_description\":\"grant_type must be password\"}")
	}
}

func TestRegisterRoutesAddsAuthMeEndpoint(t *testing.T) {
	mux := http.NewServeMux()
	_ = RegisterRoutes(muxRegistrar{mux: mux}, Config{BasePath: "/api/v1"})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+issueAuthToken("test", "test"))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if body := rec.Body.String(); body != "{\"username\":\"test\"}" {
		t.Fatalf("body = %q, want %q", body, "{\"username\":\"test\"}")
	}
}

func TestRegisterRoutesAuthOptionsSupportsPreflight(t *testing.T) {
	mux := http.NewServeMux()
	_ = RegisterRoutes(muxRegistrar{mux: mux}, Config{BasePath: "/api/v1"})

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/oauth/token", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if allowMethods := rec.Header().Get("Access-Control-Allow-Methods"); allowMethods != "GET, POST, OPTIONS" {
		t.Fatalf("Access-Control-Allow-Methods = %q, want %q", allowMethods, "GET, POST, OPTIONS")
	}
	if allowHeaders := rec.Header().Get("Access-Control-Allow-Headers"); allowHeaders != "Authorization, Content-Type" {
		t.Fatalf("Access-Control-Allow-Headers = %q, want %q", allowHeaders, "Authorization, Content-Type")
	}
}

func TestRegisterRoutesSeedsTestUser(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	if _, err := db.Exec(`CREATE TABLE users (
		id TEXT PRIMARY KEY,
		username TEXT NOT NULL UNIQUE,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`); err != nil {
		t.Fatalf("db.Exec() error = %v", err)
	}

	queries := database.New(db)
	mux := http.NewServeMux()
	_ = RegisterRoutes(muxRegistrar{mux: mux}, Config{
		BasePath: "/api/v1",
		Queries:  queries,
	})

	user, err := queries.LookupUser(context.Background(), "test")
	if err != nil {
		t.Fatalf("LookupUser() error = %v", err)
	}
	if user.ID != "test-user" {
		t.Fatalf("user.ID = %q, want %q", user.ID, "test-user")
	}
}
