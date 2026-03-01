# Frontend Websocket Guide

## What this is

This is the quick guide for using the ingest websocket stream from the frontend.

The frontend connects to the ingest service over websocket, receives batched messages, and can also send messages back when needed.

## Where the helper is

The shared frontend helper lives at:

- `web/src/utils/wsHelper.ts`

This helper handles:

- Building the default websocket URL
- Opening the connection
- Queueing outbound messages while the socket is still connecting
- Sending messages
- Closing the connection
- Parsing incoming batch payloads

## Main helper functions

- `getDefaultStreamURL()`
  - Returns the websocket URL the frontend should use by default
  - Uses `VITE_INGEST_WS_URL` if set
  - Otherwise falls back to `ws://<current-host>:8080/ws`

- `connect(url?)`
  - Opens the websocket connection
  - Adds a small queue so messages are not lost during startup

- `send(socket, msg)`
  - Sends either a string or JSON payload
  - If the socket is still opening, it queues the message

- `onBatch(socket, handler)`
  - Attaches one listener for incoming websocket batches
  - Parses the payload and gives the handler a typed batch object

- `close(socket)`
  - Closes the websocket cleanly

## Incoming message shape

The backend sends batches in this shape:

```json
{
  "kind": "batch",
  "messages": [
    {
      "kind": "event",
      "event_type": "temperature",
      "source": "ingest",
      "timestamp": "2026-03-01T12:00:00Z",
      "data": {}
    }
  ]
}
```

Each item in `messages` is one message.

Common message kinds right now:

- `event`
  - Live ingest records
- `ml_result`
  - ML anomaly results

## Basic usage

Example flow inside a component:

1. Import the helper
2. Call `connect()`
3. Listen with `onBatch()`
4. Update state from the batch messages
5. Call `close()` on unmount

The current dashboard already uses this pattern if you want a reference.

Example for listening with `onBatch()`:

```ts
import { connect, onBatch, close } from "@/utils/wsHelper";

const socket = connect();

onBatch(socket, (batch) => {
  for (const message of batch.messages) {
    if (message.kind === "event") {
      console.log("event", message.data);
    }
    if (message.kind === "ml_result") {
      console.log("ml", message.data);
    }
  }
});

window.addEventListener("beforeunload", () => {
  close(socket);
});
```

## Sending messages to the backend

When the frontend needs to send a request:

1. Build a small message object
2. Give it a `kind`
3. Call `send(socket, message)`

Example:

```ts
send(socket, {
  kind: "filter_update",
  source: "frontend",
  timestamp: new Date().toISOString(),
  data: {
    event_type: "temperature",
  },
});
```

The backend stream package can register handlers by `kind`, so new request types are simple to add.

## Config

If you want to override the default websocket location, set:

- `VITE_INGEST_WS_URL`

Example:

```bash
VITE_INGEST_WS_URL=ws://127.0.0.1:8080/ws
```

Restart the Vite dev server after changing Vite env values. You shouldn't have to though since the system is set up end-to-end to work already.

## Debugging

For quick smoke testing:

- Open the browser console
- Watch for:
  - `Attempting websocket connection`
  - `Websocket connected`
  - `Websocket batch received`
  - `Stream event received`
  - `ML result received`

Those messages come from the current dashboard component. The helper itself only logs parse errors in `onBatch()`.

If you do not see those:

1. Make sure you are on the dashboard route
2. Make sure ingest is running
3. Make sure the websocket server is listening on the expected host and port

## Development notes

- If you add new frontend request kinds, document the `kind` name and payload shape
- If the websocket contract changes, update both the helper consumers and the backend stream package at the same time so the system doesn't fall out of sync view `ingest-stream-package.md` for how to add a new stream to consume
