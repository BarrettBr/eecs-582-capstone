/*
Name: web/src/utils/wsHelper.ts
Description: Small frontend helper for connecting to the ingest websocket stream, sending messages, and receiving parsed payloads.
Programmer: Barrett Brown
Date Created: 2026-03-01
Dates Revised: 2026-03-01
Revision History:
- 2026-03-01, Barrett Brown: Created websocket helper for frontend usage.
Preconditions:
- Browser supports WebSocket.
- Ingest websocket server is reachable.
Acceptable Input Values/Types:
- Valid websocket URLs.
- String or JSON serializable outbound messages.
Unacceptable Input Values/Types:
- Empty websocket URLs.
- Non serializable message payloads.
Postconditions:
- Returns a connected or connecting WebSocket and helps queue outbound messages while opening.
Return Values/Types:
- connect: ManagedWebSocket
- send: boolean
- close: void
Error/Exception Conditions:
- WebSocket constructor errors for invalid URLs.
Side Effects:
- Opens websocket connections and writes messages.
Invariants:
- Outbound messages are queued while the socket is still connecting.
Known Faults:
- Does not add reconnect logic by itself.
*/

export interface StreamMessage {
	kind: string;
	event_type?: string;
	source: string;
	timestamp: string;
	data: unknown;
}

export interface StreamBatch {
	kind: string;
	messages: StreamMessage[];
}

export interface ManagedWebSocket extends WebSocket {
	_queue: string[];
}

// description: Builds the websocket URL used by the frontend stream client.
// input: Reads the Vite env override or current browser location.
// output: Returns the websocket endpoint string.
export function getDefaultStreamURL(): string {
	const envURL = import.meta.env.VITE_INGEST_WS_URL as string | undefined;
	if (envURL && envURL.trim() !== "") {
		return envURL;
	}

	const protocol = window.location.protocol === "https:" ? "wss" : "ws";
	const host = window.location.hostname || "127.0.0.1";
	return `${protocol}://${host}:8080/ws`;
}

// description: Opens a managed websocket and buffers messages until it connects.
// input: Optional websocket URL; defaults to the app stream endpoint.
// output: Returns a websocket with a small outbound queue.
export function connect(url = getDefaultStreamURL()): ManagedWebSocket {
	if (!url || url.trim() === "") {
		throw new Error("WebSocket url not set");
	}

	const socket = new WebSocket(url) as ManagedWebSocket;
	socket._queue = [];
	socket.addEventListener("open", () => {
		while (socket._queue.length > 0) {
			const payload = socket._queue.shift();
			if (payload !== undefined) {
				socket.send(payload);
			}
		}
	});
	return socket;
}

// description: Sends or queues one outbound websocket message.
// input: The managed socket and a string or JSON-friendly payload.
// output: Returns true if the message was sent or queued.
export function send(
	socket: ManagedWebSocket | null,
	msg: string | object,
): boolean {
	if (!socket) {
		return false;
	}

	const payload = typeof msg === "string" ? msg : JSON.stringify(msg);
	if (socket.readyState === WebSocket.OPEN) {
		socket.send(payload);
		return true;
	}
	if (socket.readyState === WebSocket.CONNECTING) {
		socket._queue ??= [];
		socket._queue.push(payload);
		return true;
	}
	return false;
}

// description: Closes the websocket if one is active.
// input: The managed socket or null.
// output: Ends the connection and returns nothing.
export function close(socket: ManagedWebSocket | null): void {
	socket?.close();
}

// description: Parses batch messages and hands valid batches to a callback.
// input: The managed socket and a batch handler function.
// output: Registers a message listener on the socket.
export function onBatch(
	socket: ManagedWebSocket,
	handler: (batch: StreamBatch) => void,
): void {
	socket.addEventListener("message", (event) => {
		try {
			const payload = JSON.parse(String(event.data)) as StreamBatch;
			if (payload.kind !== "batch" || !Array.isArray(payload.messages)) {
				return;
			}
			handler(payload);
		} catch (err) {
			console.error("Failed to parse websocket payload", err);
		}
	});
}
