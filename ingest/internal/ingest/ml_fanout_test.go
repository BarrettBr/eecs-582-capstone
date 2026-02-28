package ingest

/*
Name: ingest/internal/ingest/ml_fanout_test.go
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

	loop := &ModbusLoop{
		mlAPIURL: server.URL,
		mlHTTP:   server.Client(),
	}

	batch := []fanoutEvent{
		{record: TempSample{Temperature: 72.5}},
		{record: stubRecordEvent{record: map[string]any{"ignored": true}}},
	}

	if err := loop.deliverMLBatch(context.Background(), batch); err != nil {
		t.Fatalf("deliverMLBatch() error = %v", err)
	}
	if len(received.Samples) != 1 {
		t.Fatalf("received samples = %d, want 1", len(received.Samples))
	}
	if received.Samples[0].Temperature != 72.5 {
		t.Fatalf("received sample temperature = %v, want 72.5", received.Samples[0].Temperature)
	}
	if string(loop.mlLastResponse) != `{"ok":true}` {
		t.Fatalf("mlLastResponse = %q, want %q", string(loop.mlLastResponse), `{"ok":true}`)
	}
}

func TestDeliverMLBatchHandlesEmptyAndBadResponses(t *testing.T) {
	loop := &ModbusLoop{
		mlAPIURL: "http://example.invalid",
		mlHTTP:   &http.Client{Timeout: 10 * time.Millisecond},
	}
	if err := loop.deliverMLBatch(context.Background(), nil); err != nil {
		t.Fatalf("deliverMLBatch(nil) error = %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer server.Close()

	loop.mlAPIURL = server.URL
	loop.mlHTTP = server.Client()
	err := loop.deliverMLBatch(context.Background(), []fanoutEvent{{record: TempSample{Temperature: 1}}})
	if err == nil {
		t.Fatalf("deliverMLBatch() error = nil, want non-nil")
	}
}
