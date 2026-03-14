package stream

/*
Name: ingest/internal/stream/server_test.go
Description: Tests websocket batch encoding, service room subscriptions, and catalog pruning behavior.
Programmer: Barrett Brown
Date Created: 2026-02-28
Dates Revised: 2026-03-14
Revision History:
- 2026-02-28, Barrett Brown: Created tests for websocket stream helper coverage.
- 2026-03-14, Barrett Brown: Added service room subscription, pruning, and slow-client coverage.
Preconditions:
- Stream helper functions are available in the package.
Acceptable Input Values/Types:
- Valid JSON websocket messages, service names, and room membership state.
Unacceptable Input Values/Types:
- No special invalid runtime dependencies are needed for these tests.
Postconditions:
- Confirms room routing and catalog updates keep websocket client state consistent.
Return Values/Types:
- Test functions return no value.
Error/Exception Conditions:
- JSON decode failures after extracting frame payloads.
Side Effects:
- No external side effects.
Invariants:
- Slow clients should be removable without blocking other subscribers.
Known Faults:
- Does not test a full live websocket connection.
*/

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMarshalBatchPayloadEncodesBatchPayload(t *testing.T) {
	payload, err := marshalBatchPayload([]Message{
		{
			Kind:        "event",
			EventType:   "temperature",
			Source:      "ingest",
			ServiceName: "svc_a",
			Timestamp:   "2026-02-28T12:00:00Z",
			Data:        json.RawMessage(`{"temperature":72.5}`),
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
	if batch.Messages[0].ServiceName != "svc_a" {
		t.Fatalf("batch.Messages[0].ServiceName = %q, want %q", batch.Messages[0].ServiceName, "svc_a")
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
	server.RegisterReadHandler("custom", func(ctx context.Context, session ClientSession, msg Message) {
		called = true
		if msg.Kind != "custom" {
			t.Fatalf("handler msg.Kind = %q, want %q", msg.Kind, "custom")
		}
		if session.ID != "ws-1" {
			t.Fatalf("session.ID = %q, want %q", session.ID, "ws-1")
		}
	})
	server.dispatchIncoming(&client{session: ClientSession{ID: "ws-1"}}, Message{Kind: "custom", Source: "frontend"})
	if !called {
		t.Fatalf("expected read handler to be called")
	}
}

func TestHandleSetSubscriptionsValidatesAndReplacesRoomMembership(t *testing.T) {
	server := NewServer(Config{})
	testClient := &client{
		session:       ClientSession{ID: "ws-1"},
		send:          make(chan []byte, 2),
		subscriptions: map[string]struct{}{"svc_old": {}},
	}
	server.clients[testClient] = struct{}{}
	server.validServices["svc_a"] = struct{}{}
	server.validServices["svc_b"] = struct{}{}
	server.roomMembers["svc_old"] = map[*client]struct{}{testClient: {}}

	server.handleSetSubscriptions(setSubscriptionsRequest{
		client:    testClient,
		requested: []string{"svc_a", "svc_a", "svc_missing"},
	})
	if _, ok := server.roomMembers["svc_a"][testClient]; !ok {
		t.Fatalf("client missing from svc_a room after subscription update")
	}
	if _, ok := server.roomMembers["svc_old"]; ok {
		t.Fatalf("svc_old room still present after replacement")
	}

	message := decodeSingleMessage(t, <-testClient.send)
	if message.Kind != "subscription_ack" {
		t.Fatalf("message.Kind = %q, want %q", message.Kind, "subscription_ack")
	}
	var ack subscriptionAckPayload
	if err := json.Unmarshal(message.Data, &ack); err != nil {
		t.Fatalf("json.Unmarshal(message.Data): %v", err)
	}
	if len(ack.AcceptedServices) != 1 || ack.AcceptedServices[0] != "svc_a" {
		t.Fatalf("AcceptedServices = %v, want [svc_a]", ack.AcceptedServices)
	}
	if len(ack.RejectedServices) != 1 || ack.RejectedServices[0] != "svc_missing" {
		t.Fatalf("RejectedServices = %v, want [svc_missing]", ack.RejectedServices)
	}
	if len(ack.CurrentServices) != 1 || ack.CurrentServices[0] != "svc_a" {
		t.Fatalf("CurrentServices = %v, want [svc_a]", ack.CurrentServices)
	}
}

func TestHandleApplyCatalogPrunesRemovedServicesAndBroadcastsCatalog(t *testing.T) {
	server := NewServer(Config{})
	clientA := &client{
		session:       ClientSession{ID: "ws-1"},
		send:          make(chan []byte, 4),
		subscriptions: map[string]struct{}{"svc_a": {}, "svc_b": {}},
	}
	clientB := &client{
		session:       ClientSession{ID: "ws-2"},
		send:          make(chan []byte, 2),
		subscriptions: map[string]struct{}{},
	}
	server.clients[clientA] = struct{}{}
	server.clients[clientB] = struct{}{}
	server.validServices["svc_a"] = struct{}{}
	server.validServices["svc_b"] = struct{}{}
	server.roomMembers["svc_a"] = map[*client]struct{}{clientA: {}}
	server.roomMembers["svc_b"] = map[*client]struct{}{clientA: {}}

	server.handleApplyCatalog(applyCatalogRequest{
		catalog: ServiceCatalogPayload{
			Revision: "2026-03-14T12:00:00Z",
			Services: []ServiceCatalogEntry{
				{Name: "svc_b", Mode: "simulator", EventType: "temperature"},
			},
		},
		removed: []string{"svc_a"},
	})

	if _, ok := clientA.subscriptions["svc_a"]; ok {
		t.Fatalf("svc_a still present in clientA subscriptions after prune")
	}
	if _, ok := clientA.subscriptions["svc_b"]; !ok {
		t.Fatalf("svc_b missing from clientA subscriptions after prune")
	}
	if _, ok := server.roomMembers["svc_a"]; ok {
		t.Fatalf("svc_a room still present after prune")
	}

	pruned := decodeSingleMessage(t, <-clientA.send)
	if pruned.Kind != "subscriptions_pruned" {
		t.Fatalf("pruned.Kind = %q, want %q", pruned.Kind, "subscriptions_pruned")
	}
	catalogA := decodeSingleMessage(t, <-clientA.send)
	catalogB := decodeSingleMessage(t, <-clientB.send)
	if catalogA.Kind != "service_catalog" || catalogB.Kind != "service_catalog" {
		t.Fatalf("catalog kinds = [%s %s], want both service_catalog", catalogA.Kind, catalogB.Kind)
	}
}

func TestBroadcastRoomTargetsSubscribersAndDropsOnlySlowClient(t *testing.T) {
	server := NewServer(Config{})
	fastClient := &client{
		session:       ClientSession{ID: "ws-fast"},
		send:          make(chan []byte, 1),
		subscriptions: map[string]struct{}{"svc_a": {}},
	}
	slowClient := &client{
		session:       ClientSession{ID: "ws-slow"},
		send:          make(chan []byte, 1),
		subscriptions: map[string]struct{}{"svc_a": {}},
	}
	otherClient := &client{
		session:       ClientSession{ID: "ws-other"},
		send:          make(chan []byte, 1),
		subscriptions: map[string]struct{}{"svc_b": {}},
	}
	server.clients[fastClient] = struct{}{}
	server.clients[slowClient] = struct{}{}
	server.clients[otherClient] = struct{}{}
	server.roomMembers["svc_a"] = map[*client]struct{}{
		fastClient: {},
		slowClient: {},
	}
	server.roomMembers["svc_b"] = map[*client]struct{}{
		otherClient: {},
	}
	slowClient.send <- []byte("full")

	frame, err := marshalBatchPayload([]Message{{
		Kind:        "event",
		EventType:   "temperature",
		Source:      "ingest",
		ServiceName: "svc_a",
		Timestamp:   "2026-03-14T12:00:00Z",
		Data:        json.RawMessage(`{"temperature":72.5}`),
	}})
	if err != nil {
		t.Fatalf("marshalBatchPayload() error = %v", err)
	}

	server.broadcastRoom("svc_a", frame)

	if got := decodeSingleMessage(t, <-fastClient.send); got.ServiceName != "svc_a" {
		t.Fatalf("fast client service = %q, want %q", got.ServiceName, "svc_a")
	}
	select {
	case <-otherClient.send:
		t.Fatalf("unexpected frame delivered to non-subscriber")
	default:
	}
	if _, ok := server.clients[slowClient]; ok {
		t.Fatalf("slow client still registered after full queue")
	}
}

func TestRegisterHTTPHandlerMountsRouteOnSharedMux(t *testing.T) {
	server := NewServer(Config{})
	server.RegisterHTTPHandler("/api/v1/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("pong"))
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ping", nil)
	rec := httptest.NewRecorder()
	server.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if body := rec.Body.String(); body != "pong" {
		t.Fatalf("body = %q, want %q", body, "pong")
	}
}

func decodeSingleMessage(t *testing.T, frame []byte) Message {
	t.Helper()

	var batch batchPayload
	if err := json.Unmarshal(frame, &batch); err != nil {
		t.Fatalf("json.Unmarshal(frame) error = %v", err)
	}
	if batch.Kind != "batch" {
		t.Fatalf("batch.Kind = %q, want %q", batch.Kind, "batch")
	}
	if len(batch.Messages) != 1 {
		t.Fatalf("len(batch.Messages) = %d, want 1", len(batch.Messages))
	}
	return batch.Messages[0]
}
