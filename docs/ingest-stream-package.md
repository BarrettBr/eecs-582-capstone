# Ingest Stream Package Guide

## What this package is

The stream package is the live frontend delivery layer for ingest.

It runs a websocket server, accepts browser clients, batches outgoing messages, and sends live event updates plus ML result updates to the frontend.

## Main flow

The stream flow is

1. `main.go` builds `stream.NewServer(...)`
2. `Start` launches the batch loop and HTTP server
3. Browser clients connect to the configured websocket path
4. The ingest package calls `PublishEvent` for live records
5. The ingest package calls `PublishMLResult` for ML outputs
6. The stream server batches these messages and broadcasts them to connected clients
7. The stream server can also read inbound client messages and route them by message kind

This package keeps the frontend path separate from the Modbus loop itself.

## Message shape

There are two levels of payload

- `Message`
  - One logical item, such as one ingest event or one ML result
- `batchPayload`
  - One websocket payload that wraps a slice of messages

That batching is important for lower performance machines and for avoiding too many tiny websocket writes.

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

Basic flow

1. Decide the `kind`
2. Decide the `source`
3. Set `event_type` if the message is tied to a specific event family
4. Marshal or reuse the JSON payload for `Data`
5. Call `streamServer.Publish(...)`

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

## How to read a new event

The stream package can now accept inbound websocket messages from clients and route them by `kind`.

To support a new inbound message

1. Pick a `kind` name for the incoming message
2. Register a handler with `RegisterReadHandler(kind, handler)`
3. In that handler, unmarshal `msg.Data` into the shape you expect
4. Do the small action needed for that message

Example uses

- Frontend sends a filter update
- Frontend sends a subscribe request
- Frontend sends a simple command or ping style message

The handler gets the whole `Message`, so you can inspect

- `Kind`
- `Source`
- `EventType`
- `Timestamp`
- `Data`

The stream package handles the websocket read and decode part. Your handler should stay small and pass real business logic to another package if it starts growing.

## How to extend it

Good future additions here would be

- Ping and pong handling for connection health
- Reconnect and heartbeat support
- Different message kinds for new event types
- Optional per client filtering
- Better origin checks than the current open default

## Development notes

- This package should stay focused on transport, not business logic.
- If message contents change, update the frontend consumer at the same time.
- If backpressure becomes a problem, tune batch size and flush interval first before changing the main ingest loop.
