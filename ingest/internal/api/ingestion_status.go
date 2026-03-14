package api

/*
Name: ingest/internal/api/ingestion_status.go
Description: Serves the live ingest status endpoint from the registrar and BufferManager snapshot.
Programmer: Barrett Brown
Date Created: 2026-03-07
Dates Revised: 2026-03-13
Revision History:
- 2026-03-07, Barrett Brown: Added standardized prologue documentation block.
- 2026-03-13, Barrett Brown: Added clear status handler function comments.
Preconditions:
- Status provider is registered when the ingest runtime is running.
Acceptable Input Values/Types:
- HTTP GET requests for the ingestion status endpoint.
Unacceptable Input Values/Types:
- Nil status provider when the endpoint is called.
Postconditions:
- Returns the current ingest status snapshot as JSON when available.
Return Values/Types:
- Handler writes an HTTP response directly.
Error/Exception Conditions:
- Missing status provider returns 503.
Side Effects:
- Reads runtime status and writes an HTTP response.
Invariants:
- Status responses come from one snapshot call.
Known Faults:
- Status is point in time data and may change immediately after the response.
*/

import "net/http"

// description: Returns the current ingest runtime status snapshot.
// input: HTTP response writer and request for the status endpoint.
// output: Writes status JSON or a service unavailable error response.
func (cfg *apiConfig) ingestionStatusHandler(w http.ResponseWriter, _ *http.Request) {
	if cfg.status == nil {
		respondWithError(w, http.StatusServiceUnavailable, "ingestion status unavailable", nil)
		return
	}
	respondWithJSON(w, http.StatusOK, cfg.status.StatusSnapshot())
}
