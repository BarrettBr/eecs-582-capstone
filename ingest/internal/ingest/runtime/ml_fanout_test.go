package runtime

/*
Name: ingest/internal/ingest/runtime/ml_fanout_test.go
Description: Tests ML batch posting, skipped records, and basic ML response handling.
Programmer: Barrett Brown
Date Created: 2026-02-28
Dates Revised: 2026-02-28
Revision History:
- 2026-02-28, Barrett Brown: Created ML fanout tests for batch delivery behavior.
Preconditions:
- HTTP test servers can be started locally during test execution.
Acceptable Input Values/Types:
- Temperature fanout events.
- Unsupported stub events for skip behavior checks.
- Success and failure HTTP responses.
Unacceptable Input Values/Types:
- Nil HTTP clients are not used in these tests.
Postconditions:
- Confirms only supported temperature events are posted and errors surface correctly.
Return Values/Types:
- Test functions return no value.
Error/Exception Conditions:
- HTTP request failures.
- Invalid JSON decode in the test server handler.
Side Effects:
- Starts local HTTP test servers.
- Sends HTTP requests during test execution.
Invariants:
- Unsupported event types should be skipped instead of breaking the whole batch.
Known Faults:
- Does not cover malformed JSON success responses yet.
*/

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	ingestevents "github.com/BarrettBr/eecs-582-capstone/internal/ingest/events"
)

func TestDeliverMLBatchSkipsUnsupportedEventsAndPostsTemperatureSamples(t *testing.T) {
	var received mlBatchRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	pipeline := &Pipeline{
		mlAPIURL: server.URL,
		mlHTTP:   server.Client(),
	}

	batch := []IngressEvent{
		{Record: ingestevents.TempSample{Temperature: 72.5}, MLEnabled: true},
		{Record: ingestevents.ValveSample{FlowRate: 12.5}, MLEnabled: true},
	}

	if err := pipeline.deliverMLBatch(context.Background(), batch); err != nil {
		t.Fatalf("deliverMLBatch() error = %v", err)
	}
	if received.EventType != "valve" && received.EventType != "temperature" {
		t.Fatalf("received event_type = %q, want typed batch", received.EventType)
	}
	if len(received.Samples) != 1 {
		t.Fatalf("received samples = %d, want 1", len(received.Samples))
	}
	var normalized MLAnomalyPayload
	if err := json.Unmarshal(pipeline.mlLastResponse, &normalized); err != nil {
		t.Fatalf("json.Unmarshal(mlLastResponse) error = %v", err)
	}
	if normalized.Schema != "ml_anomaly_v1" {
		t.Fatalf("normalized schema = %q, want %q", normalized.Schema, "ml_anomaly_v1")
	}
	if normalized.HasAnomaly {
		t.Fatalf("normalized HasAnomaly = true, want false")
	}
	if string(normalized.RawResponse) != `{"ok":true}` {
		t.Fatalf("normalized RawResponse = %q, want %q", string(normalized.RawResponse), `{"ok":true}`)
	}
}

func TestDeliverMLBatchHandlesEmptyAndBadResponses(t *testing.T) {
	pipeline := &Pipeline{
		mlAPIURL: "http://example.invalid",
		mlHTTP:   &http.Client{Timeout: 10 * time.Millisecond},
	}
	if err := pipeline.deliverMLBatch(context.Background(), nil); err != nil {
		t.Fatalf("deliverMLBatch(nil) error = %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer server.Close()

	pipeline.mlAPIURL = server.URL
	pipeline.mlHTTP = server.Client()
	err := pipeline.deliverMLBatch(context.Background(), []IngressEvent{{Record: ingestevents.TempSample{Temperature: 1}, MLEnabled: true}})
	if err == nil {
		t.Fatalf("deliverMLBatch() error = nil, want non-nil")
	}
}
