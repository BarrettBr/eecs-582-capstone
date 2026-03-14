# Ingest Stream Package Guide

## What this package is

The stream package is the backend transport layer for live frontend delivery.

It runs a websocket server, accepts browser clients, batches outgoing messages, and sends live event updates plus ML result updates to the frontend.

If you need the frontend helper, browser examples, or client-side message handling, use `docs/frontend-websockets.md`. This guide is only about the backend stream package.

## Main flow

The stream flow is:

1. `main.go` builds `stream.NewServer(...)`
2. If needed, `main.go` registers inbound read handlers on that server
3. `Start` launches the batch loop and HTTP server
4. Browser clients connect to the configured websocket path
5. The ingest package calls `PublishScopedEvent` for live records
6. The ingest package calls `PublishScopedMLResult` for ML outputs
7. The stream server batches those messages by `service_name` and fans each batch out only to subscribers of that room
8. The registrar pushes service catalog updates into the stream server so removed services can be pruned from subscriptions immediately
9. The stream server can also read inbound client messages and route them by message kind

This package keeps the frontend path separate from the Modbus loop itself.

## What it owns

- Websocket listener
- Client registration and cleanup
- Per-client subscription state
- Inverse room membership indexes
- Outbound batching for service rooms plus global control messages
- Service catalog broadcast and subscription prune notifications
- Inbound websocket reads
- Inbound message routing by kind

## How it fits with the rest of the service

- The ingest websocket sink does not talk to browsers directly
- It calls this package instead
- This package handles service-room routing and transport fanout

That keeps frontend delivery concerns out of the core Modbus polling code.

## Message shape

There are two levels of payload:

- `Message`
  - one logical item, such as one ingest event or one ML result
- `batchPayload`
  - one websocket frame that wraps a slice of messages

The frontend-facing JSON examples live in `docs/frontend-websockets.md`.

## Adding outbound messages

If you want to send a new kind of message out to the frontend, prefer the generic `Publish` method or a small wrapper around it.

Today the main outbound callers are in `ingest/internal/ingest/runtime/fanout.go`.

Typical pattern:

1. Build the payload in the package that owns the data.
2. Pick a `kind` the frontend can switch on.
3. Call `Publish(...)` or add a wrapper like `PublishEvent(...)`.
4. Update the frontend listener in `docs/frontend-websockets.md` terms.

Example:

```go
payload, err := json.Marshal(map[string]any{
	"status": "ready",
	"detail": "stream is healthy",
})
if err != nil {
	return err
}

streamServer.Publish(stream.Message{
	Kind:      "system_status",
	Source:    "ingest",
	EventType: "",
	Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
	Data:      payload,
})
```

Use the built-in wrappers when they already fit:

- `PublishScopedEvent`
- `PublishScopedMLResult`

For service-scoped live traffic, set `service_name` and let the room fanout handle the rest. Leave `service_name` empty only for control messages that should go to everyone, such as `service_catalog`.

## Adding inbound handlers

The stream package can also accept websocket messages from clients and route them by `kind`.

Register handlers in startup code, usually in `ingest/main.go`, after `stream.NewServer(...)` is created.

Typical pattern:

1. Decide what the frontend sends.
2. Pick a `kind`.
3. Register `RegisterReadHandler(kind, handler)`.
4. Unmarshal `msg.Data`.
5. Call the package that owns the real behavior.

Example:

```go
type faultInjectRequest struct {
	SourceName string `json:"source_name"`
}

wsServer.RegisterReadHandler("inject_fault", func(ctx context.Context, session stream.ClientSession, msg stream.Message) {
	var req faultInjectRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		log.Printf("Stream read decode error kind=%s: %v", msg.Kind, err)
		return
	}

	log.Printf("Frontend requested fault injection for source=%s from session=%s", req.SourceName, session.ID)

	// Call the package that owns this behavior here.
})
```

Keep the handler small. If it starts doing real business logic, move that logic back into the owning package and call it from the handler.

The stream package also owns one built-in inbound control message now:

- `set_service_subscriptions`
  - `data.service_names` is treated as the caller's full replacement subscription set
  - the server validates those names against the current registrar catalog
  - the caller receives `subscription_ack`

The stream package can also emit these backend-generated control messages:

- `service_catalog`
  - broadcast to all clients on connect and after catalog apply
- `subscriptions_pruned`
  - sent only to affected clients when the registrar removes a subscribed service

## Development notes

- This package should stay focused on transport, routing, and subscription bookkeeping, not business logic.
- If message contents change, update the frontend consumer at the same time.
- Room fanout is optimized around batching once per service room rather than rebuilding a custom batch per client.
- For client-side examples and payload handling, point teammates to `docs/frontend-websockets.md`.
