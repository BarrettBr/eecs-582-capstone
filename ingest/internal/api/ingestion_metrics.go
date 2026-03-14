package api

/*
Name: ingest/internal/api/ingestion_metrics.go
Description: Serves persisted ingest record counts for dashboard metric initialization.
Programmer: Codex
Date Created: 2026-03-14
Dates Revised: 2026-03-14
Revision History:
- 2026-03-14, Codex: Added ingest metrics handler for initial dashboard totals.
Preconditions:
- Query helpers are configured when the endpoint is called.
Acceptable Input Values/Types:
- HTTP GET requests for the ingestion metrics endpoint.
Unacceptable Input Values/Types:
- Nil query dependencies when the endpoint is called.
Postconditions:
- Returns persisted temperature, valve, and total record counts as JSON.
Return Values/Types:
- Handler writes an HTTP response directly.
Error/Exception Conditions:
- Missing query dependencies return 503.
- Count query failures return 500.
Side Effects:
- Reads aggregate counts from the database and writes an HTTP response.
Invariants:
- TotalRecords equals TempRecords plus ValveRecords.
Known Faults:
- Counts reflect committed database state only and not in-flight buffered records.
*/

import "net/http"

type ingestionMetricsResponse struct {
	TempRecords  int64 `json:"temp_records"`
	ValveRecords int64 `json:"valve_records"`
	TotalRecords int64 `json:"total_records"`
}

// description: Returns the current persisted ingest record totals.
// input: HTTP response writer and request for the metrics endpoint.
// output: Writes persisted record counts as JSON.
func (cfg *apiConfig) ingestionMetricsHandler(w http.ResponseWriter, r *http.Request) {
	if cfg.queries == nil {
		respondWithError(w, http.StatusServiceUnavailable, "ingestion metrics unavailable", nil)
		return
	}

	tempRecords, err := cfg.queries.GetTempSampleCount(r.Context())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed ingest metrics query", err)
		return
	}

	valveRecords, err := cfg.queries.GetValveSampleCount(r.Context())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed ingest metrics query", err)
		return
	}

	respondWithJSON(w, http.StatusOK, ingestionMetricsResponse{
		TempRecords:  tempRecords,
		ValveRecords: valveRecords,
		TotalRecords: tempRecords + valveRecords,
	})
}
