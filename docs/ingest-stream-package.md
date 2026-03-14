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
5. The ingest package calls `PublishEvent` for live records
6. The ingest package calls `PublishMLResult` for ML outputs
7. The stream server batches these messages and broadcasts them to connected clients
8. The stream server can also read inbound client messages and route them by message kind

This package keeps the frontend path separate from the Modbus loop itself.

## What it owns

- Websocket listener
- Client registration and cleanup
- Outbound batching
- Broadcast fanout to connected frontend clients
- Inbound websocket reads
- Inbound message routing by kind

## How it fits with the rest of the service

- The ingest websocket sink does not talk to browsers directly
- It calls this package instead
- This package handles buffering and broadcasting

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

- `PublishEvent`
- `PublishMLResult`

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
type filterRequest struct {
	EventType string `json:"event_type"`
}

wsServer.RegisterReadHandler("set_filter", func(ctx context.Context, msg stream.Message) {
	var req filterRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		log.Printf("Stream read decode error kind=%s: %v", msg.Kind, err)
		return
	}

	log.Printf("Frontend requested filter for event_type=%s", req.EventType)

	// Call the package that owns this behavior here.
})
```

Keep the handler small. If it starts doing real business logic, move that logic back into the owning package and call it from the handler.

## Development notes

- This package should stay focused on transport, not business logic.
- If message contents change, update the frontend consumer at the same time.
- If backpressure becomes a problem, tune batch size and flush interval first before changing the main ingest loop.
- For client-side examples and payload handling, point teammates to `docs/frontend-websockets.md`.
