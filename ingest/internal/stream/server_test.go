package stream

/*
Name: ingest/internal/stream/server_test.go
Description: Tests websocket helper functions and stream batch encoding.
Programmer: Barrett Brown
Date Created: 2026-02-28
Dates Revised: 2026-02-28
Revision History:
- 2026-02-28, Barrett Brown: Created tests for websocket stream helper coverage.
Preconditions:
- Stream helper functions are available in the package.
Acceptable Input Values/Types:
- Valid websocket keys.
- Small JSON messages for batch encoding.
Unacceptable Input Values/Types:
- No special invalid runtime dependencies are needed for these tests.
Postconditions:
- Confirms websocket accept keys and frames are encoded as expected.
Return Values/Types:
- Test functions return no value.
Error/Exception Conditions:
- JSON decode failures after extracting frame payloads.
Side Effects:
- No external side effects.
Invariants:
- Text websocket frames should start with the text opcode header.
Known Faults:
- Does not test a full live websocket connection.
*/

import (
	"context"
	"encoding/json"
	"testing"
)

func TestMarshalBatchPayloadEncodesBatchPayload(t *testing.T) {
	payload, err := marshalBatchPayload([]Message{
		{
			Kind:      "event",
			EventType: "temperature",
			Source:    "ingest",
			Timestamp: "2026-02-28T12:00:00Z",
			Data:      json.RawMessage(`{"temperature":72.5}`),
		},
	})
	if err != nil {
		t.Fatalf("marshalBatchPayload() error = %v", err)
	}
	var batch batchPayload
	if err := json.Unmarshal(payload, &batch); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if batch.Kind != "batch" {
		t.Fatalf("batch.Kind = %q, want %q", batch.Kind, "batch")
	}
	if len(batch.Messages) != 1 {
		t.Fatalf("batch.Messages length = %d, want 1", len(batch.Messages))
	}
}

func TestParseIncomingPayloadSupportsSingleAndBatch(t *testing.T) {
	single, err := parseIncomingPayload([]byte(`{
		"kind":"command",
		"source":"frontend",
		"timestamp":"2026-02-28T12:00:00Z",
		"data":{"action":"ping"}
	}`))
	if err != nil {
		t.Fatalf("parseIncomingPayload(single) error = %v", err)
	}
	if len(single) != 1 || single[0].Kind != "command" {
		t.Fatalf("parseIncomingPayload(single) = %+v, want one command message", single)
	}

	batch, err := parseIncomingPayload([]byte(`{
		"kind":"batch",
		"messages":[
			{"kind":"command","source":"frontend","timestamp":"2026-02-28T12:00:00Z","data":{"action":"one"}},
			{"kind":"command","source":"frontend","timestamp":"2026-02-28T12:00:01Z","data":{"action":"two"}}
		]
	}`))
	if err != nil {
		t.Fatalf("parseIncomingPayload(batch) error = %v", err)
	}
	if len(batch) != 2 {
		t.Fatalf("parseIncomingPayload(batch) length = %d, want 2", len(batch))
	}
}

func TestPublishCopiesPayloadAndDispatchIncomingUsesHandler(t *testing.T) {
	server := NewServer(Config{})
	original := []byte(`{"value":1}`)

	server.Publish(Message{
		Kind:      "custom",
		Source:    "test",
		Timestamp: "2026-02-28T12:00:00Z",
		Data:      original,
	})
	original[0] = 'x'

	select {
	case msg := <-server.publishCh:
		if string(msg.Data) != `{"value":1}` {
			t.Fatalf("queued payload = %q, want original copied payload", string(msg.Data))
		}
	default:
		t.Fatalf("expected queued publish message")
	}

	called := false
	server.runCtx = context.Background()
	server.RegisterReadHandler("custom", func(ctx context.Context, msg Message) {
		called = true
		if msg.Kind != "custom" {
			t.Fatalf("handler msg.Kind = %q, want %q", msg.Kind, "custom")
		}
	})
	server.dispatchIncoming(Message{Kind: "custom", Source: "frontend"})
	if !called {
		t.Fatalf("expected read handler to be called")
	}
}
