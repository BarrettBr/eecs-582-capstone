# Ingest Main Package Guide

## What this package is

This is the startup layer for the ingest service.

The main package does not hold the core business logic. Its job is to wire the other packages together, start the long running services, and stop them cleanly when the process is shutting down.

## Main flow

When the ingest service starts, `ingest/main.go` does this in order

1. Loads env settings through the config package
2. Opens the SQLite database
3. Runs goose migrations
4. Starts the websocket stream server
5. Builds the Modbus loop
6. Runs the Modbus loop until shutdown

## What it owns

- Process startup
- Dependency wiring
- Database migration bootstrapping
- Shared shutdown context
- Logging startup state

## What it does not own

- Parsing env values
- SQL query definitions
- Modbus data normalization
- Validation rules
- Websocket batching logic

Those live in the internal packages.

## How to extend it

If you add a new runtime service, this is usually where you start it.

Examples

- A new background worker
- A new API server
- A new sink that needs its own shared dependency

Keep this file focused on wiring. If logic starts getting specific or stateful, move it into an internal package and call it from here.

## Development notes

- If startup is failing, check this file first because it controls the service boot order.
- If a new package needs config, add it in the config package first, then pass it through here.
- If a service should shut down with the app, make sure it uses the same shared context.
