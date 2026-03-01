# Ingest Stream Package Guide

## What this package is

The stream package is the live frontend delivery layer for ingest.

It runs a websocket server, accepts browser clients, batches outgoing messages, and sends live event updates plus ML result updates to the frontend.

## Main flow

The stream flow is

1. `main.go` builds `stream.NewServer(...)`
2. If needed, `main.go` registers inbound read handlers on that server
3. `Start` launches the batch loop and HTTP server
4. Browser clients connect to the configured websocket path
5. The ingest package calls `PublishEvent` for live records
6. The ingest package calls `PublishMLResult` for ML outputs
7. The stream server batches these messages and broadcasts them to connected clients
8. The stream server can also read inbound client messages and route them by message kind

This package keeps the frontend path separate from the Modbus loop itself.

## Message shape

There are two levels of payload

- `Message`
  - One logical item, such as one ingest event or one ML result
- `batchPayload`
  - One websocket payload that wraps a slice of messages

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

## How to write a new event

If you want to send a new kind of message out to the frontend, the easiest path is to use the generic `Publish` method.

Right now the existing send path comes from the ingest package. `fanout.go` pushes live records and ML results into the stream server. For a new outbound message, you follow the same pattern from the package that owns that data.

From I want to send X to sending X

1. Decide where the new data is produced
2. Build a small JSON payload for that data
3. Pick a `kind` name that the frontend can switch on
4. Pick a `source` name that explains where it came from
5. Set `event_type` if the message belongs to one event family
6. Call `streamServer.Publish(...)` or add a small wrapper like `PublishEvent`
7. Update the frontend listener to check for that `kind`

Where to add it

- Most of the time you add this in the package that already owns the data
- For ingest records or ML results, that is `ingest/internal/ingest/fanout.go`
- If you create a brand new outbound family that will be used more than once, add a helper method in `ingest/internal/stream/server.go`

Example shape

- `kind`
  - The message category such as `event`, `ml_result`, or a future value like `system_status`
- `source`
  - Where it came from such as `ingest`, `ml`, or `frontend`
- `event_type`
  - Optional event family such as `temperature`
- `data`
  - The actual JSON payload the frontend will read

Use the small wrapper methods when they already fit

- `PublishEvent`
  - Good for normal live ingest records
- `PublishMLResult`
  - Good for ML response payloads

If your new message does not fit those, add a small wrapper or call `Publish` directly.

Minimal example

1. Build the payload in the package that owns the data
2. Marshal it
3. Publish it

This is the pattern to follow

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

## How to read a new event

The stream package can accept websocket messages from clients and route them by `kind`.

Important note

To actually read and consume a new message, you need to register the handler in startup code, usually in `ingest/main.go`, after `stream.NewServer(...)` is created.

From I want to read X to reading and consuming X

1. Decide what the frontend will send
2. Pick a `kind` name for that message
3. Define the payload shape you expect that will consume it
4. In `ingest/main.go`, register a handler with `RegisterReadHandler(kind, handler)` on the stream server
5. In that handler, unmarshal `msg.Data` into your payload struct
6. Validate the fields if needed
7. Call the logic from that handler

Where to add it

- Register the handler in `ingest/main.go` right after `wsServer := stream.NewServer(...)`
- Keep the handler small
- If the action is more than a few lines, call into the package that owns that behavior

Minimal example

This is the practical startup pattern

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

The handler gets the whole `Message`, so you can inspect

- `Kind`
- `Source`
- `EventType`
- `Timestamp`
- `Data`

The stream package handles the websocket read loop and routes by `kind`. Your handler is responsible for decoding `Data` into the payload you expect and then handing it off to the real code that should consume it.

Current practical rule

- If you want to receive new client messages today, add `RegisterReadHandler(...)` calls in `ingest/main.go`
- If you want those messages to change ingest behavior, call into the ingest package from that handler
- If you want those messages to affect only websocket behavior, keep the logic in the stream package or a small helper near it

## Development notes

- This package should stay focused on transport, not business logic.
- If message contents change, update the frontend consumer at the same time.
- If backpressure becomes a problem, tune batch size and flush interval first before changing the main ingest loop.
