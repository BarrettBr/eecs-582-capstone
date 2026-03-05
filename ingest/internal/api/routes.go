package api

import (
	"encoding/json"
	"log"
	"net/http"
	"path"

	"github.com/BarrettBr/eecs-582-capstone/internal/database"
)

// HTTPRegistrar is implemented by components that expose RegisterHTTPHandler.
type HTTPRegistrar interface {
	RegisterHTTPHandler(path string, handler http.HandlerFunc)
}

// Config holds API registration settings.
type Config struct {
	BasePath string
	Queries  *database.Queries
}

// Registration contains important registered endpoint paths for logging.
type Registration struct {
	PingPath string
}

type apiConfig struct {
	basePath string
	queries  *database.Queries
}

// RegisterRoutes registers versioned API handlers on the shared ingest server mux.
func RegisterRoutes(registrar HTTPRegistrar, cfg Config) Registration {
	apiCfg := &apiConfig{
		basePath: cfg.BasePath,
		queries:  cfg.Queries,
	}

	pingPath := path.Join(apiCfg.basePath, "ping")
	historyPath := path.Join(apiCfg.basePath, "history", "{kind}", "{window}")
	reportPath := path.Join(apiCfg.basePath, "report", "{kind}", "{window}")
	routes := map[string]http.HandlerFunc{
		"GET " + pingPath:    apiCfg.pingHandler,
		"GET " + historyPath: apiCfg.historyHandler,
		"GET " + reportPath:  apiCfg.reportHandler,
	}

	for pattern, handler := range routes {
		registrar.RegisterHTTPHandler(pattern, handler)
	}

	return Registration{
		PingPath: pingPath,
	}
}

func (cfg *apiConfig) pingHandler(w http.ResponseWriter, _ *http.Request) {
	respondWithJSON(w, http.StatusOK, map[string]string{"message": "pong"})
}

func respondWithError(w http.ResponseWriter, code int, msg string, err error) {
	if err != nil {
		log.Println(err)
	}
	if code >= http.StatusInternalServerError {
		log.Printf("Responding with 5XX error: %s", msg)
	}

	type errorResponse struct {
		Error string `json:"error"`
	}
	respondWithJSON(w, code, errorResponse{
		Error: msg,
	})
}

func respondWithJSON(w http.ResponseWriter, code int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	dat, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshalling JSON: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(code)
	_, _ = w.Write(dat)
}
