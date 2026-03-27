package api

/*
Name: ingest/internal/api/routes.go
Description: Registers the shared ingest API endpoints and provides small JSON response helpers.
Programmer: Barrett Brown
Date Created: 2026-03-07
Dates Revised: 2026-03-15
Revision History:
- 2026-03-07, Barrett Brown: Added standardized prologue documentation block.
- 2026-03-13, Barrett Brown: Added clearer route registration and response helper comments.
- 2026-03-15, Barrett Brown: Added permissive API CORS headers for frontend dev-server requests.
Preconditions:
- A shared HTTP registrar is available to mount handlers.
- API base path is normalized before registration.
Acceptable Input Values/Types:
- Route config with base path, query access, and optional status provider.
- JSON serializable response payloads.
Unacceptable Input Values/Types:
- Nil registrar.
- Non serializable response payloads.
Postconditions:
- Registers the ingest API handlers and returns key route paths.
Return Values/Types:
- RegisterRoutes: Registration
- Response helpers write HTTP responses directly.
Error/Exception Conditions:
- JSON marshal failures return 500 responses.
Side Effects:
- Registers handlers on the shared HTTP mux and writes logs for response errors.
Invariants:
- Registered route paths match the returned Registration values.
Known Faults:
- Response helpers still do one marshal per response payload.
*/

import (
	"encoding/json"
	"log"
	"net/http"
	"path"

	"github.com/BarrettBr/eecs-582-capstone/internal/database"
	ingestruntime "github.com/BarrettBr/eecs-582-capstone/internal/ingest/runtime"
)

// HTTPRegistrar is implemented by components that expose RegisterHTTPHandler.
type HTTPRegistrar interface {
	RegisterHTTPHandler(path string, handler http.HandlerFunc)
}

// Config holds API registration settings.
type Config struct {
	BasePath         string
	Queries          *database.Queries
	Status           StatusProvider
	SourceConfigPath string
	SourceProfile    string
}

// Registration contains important registered endpoint paths for logging.
type Registration struct {
	PingPath             string
	HistoryPath          string
	ReportPath           string
	IngestionStatusPath  string
	IngestionMetricsPath string
	AdminConfigPath      string
}

type apiConfig struct {
	basePath         string
	queries          *database.Queries
	status           StatusProvider
	sourceConfigPath string
	sourceProfile    string
}

type StatusProvider interface {
	StatusSnapshot() ingestruntime.SystemStatusSnapshot
}

// description: Mounts the ingest API routes on the shared HTTP registrar.
// input: HTTP registrar plus base path, query service, and optional status provider.
// output: Returns the final registered route paths for logging and startup output.
func RegisterRoutes(registrar HTTPRegistrar, cfg Config) Registration {
	apiCfg := &apiConfig{
		basePath:         cfg.BasePath,
		queries:          cfg.Queries,
		status:           cfg.Status,
		sourceConfigPath: cfg.SourceConfigPath,
		sourceProfile:    cfg.SourceProfile,
	}

	pingPath := path.Join(apiCfg.basePath, "ping")
	historyPath := path.Join(apiCfg.basePath, "history", "{kind}", "{window}")
	reportPath := path.Join(apiCfg.basePath, "report", "{kind}", "{window}")
	ingestionStatusPath := path.Join(apiCfg.basePath, "ingestion", "status")
	ingestionMetricsPath := path.Join(apiCfg.basePath, "ingestion", "metrics")
	adminConfigPath := path.Join(apiCfg.basePath, "admin", "config")
	routes := map[string]http.HandlerFunc{
		"GET " + pingPath:             apiCfg.pingHandler,
		"GET " + historyPath:          apiCfg.historyHandler,
		"GET " + reportPath:           apiCfg.reportHandler,
		"GET " + ingestionStatusPath:  apiCfg.ingestionStatusHandler,
		"GET " + ingestionMetricsPath: apiCfg.ingestionMetricsHandler,
		"GET " + adminConfigPath:      apiCfg.adminConfigHandler,
		"PUT " + adminConfigPath:      apiCfg.adminConfigHandler,
		"OPTIONS " + adminConfigPath:  apiCfg.optionsHandler,
	}

	for pattern, handler := range routes {
		registrar.RegisterHTTPHandler(pattern, handler)
	}

	// Pass back to echo to the server different endpoints
	return Registration{
		PingPath:             pingPath,
		HistoryPath:          historyPath,
		ReportPath:           reportPath,
		IngestionStatusPath:  ingestionStatusPath,
		IngestionMetricsPath: ingestionMetricsPath,
		AdminConfigPath:      adminConfigPath,
	}
}

// description: Returns a simple health response for API reachability checks.
// input: HTTP response writer and request for the ping endpoint.
// output: Writes a small JSON pong payload.
func (cfg *apiConfig) pingHandler(w http.ResponseWriter, _ *http.Request) {
	respondWithJSON(w, http.StatusOK, map[string]string{"message": "pong"})
}

func (cfg *apiConfig) optionsHandler(w http.ResponseWriter, _ *http.Request) {
	respondWithJSON(w, http.StatusNoContent, nil)
}

// description: Writes a JSON error response and logs server side errors when present.
// input: HTTP response writer, status code, public message, and optional internal error.
// output: Sends a JSON error body to the client.
func respondWithError(w http.ResponseWriter, status_code int, msg string, err error) {
	if err != nil {
		log.Println(err)
	}
	if status_code >= http.StatusInternalServerError {
		log.Printf("Responding with 5XX error: %s", msg)
	}

	type errorResponse struct {
		Error string `json:"error"`
	}
	respondWithJSON(w, status_code, errorResponse{
		Error: msg,
	})
}

// description: Marshals one payload as JSON and writes it to the response.
// input: HTTP response writer, status code, and JSON serializable payload.
// output: Sends a JSON response body or a 500 error on marshal failure.
func respondWithJSON(w http.ResponseWriter, status_code int, payload any) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, PUT, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")
	if payload == nil {
		w.WriteHeader(status_code)
		return
	}
	dat, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshalling JSON: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(status_code)
	_, _ = w.Write(dat)
}
