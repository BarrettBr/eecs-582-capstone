# Ingest Core Package Guide

## What this package is

This is the main application logic for the ingest service.

It owns the Modbus polling loop, data normalization, validation, and fanout to the downstream sinks.

If you want to understand how one PLC sample turns into stored data and frontend updates, this is the package to read first.

## Main flow

The normal flow through this package is

1. `NewModbusLoop` builds the runtime object
2. `Run` starts the sink workers and loops on the poll interval
3. `handleTick` reads Modbus registers
4. `dataNormalizer` converts raw bytes into a `TempSample`
5. `RuleEngine.Validate` checks the sample and adds anomaly labels
6. The sample is JSON encoded
7. `fanOut` sends the event to ML, SQL, and websocket sinks

That is the core ingestion path.

## Main pieces

- `modbus_loop.go`
  - Main runtime loop and Modbus byte parsing
- `TempSample.go`
  - Current normalized event shape
- `record_event.go`
  - Generic event interface used by validation and fanout
- `validation.go`
  - Rule loading and record validation
- `fanout.go`
  - Sink worker loops and sink delivery logic

## Sink behavior

The package currently has three downstream paths

- ML sink
  - Batches temperature events and posts them to the ML API
- SQL sink
  - Batches writes to SQLite
  - Stores all anomalous samples
  - Samples normal records based on config
- Websocket sink
  - Publishes live event payloads to the stream package

ML responses are also fed back into the stream package so the frontend can show ML anomaly output.

## How to extend it

If you want to add a new event type later

1. Create a new struct that implements `RecordEvent`
2. Add the normalization logic that builds it
3. Update validation rules if needed
4. Update sink handling where that event type should go

If you want to add a new sink

1. Add a new channel to `ModbusLoop`
2. Start a new worker in `startFanoutWorkers`
3. Add enqueue logic in `fanOut`
4. Implement the sink delivery function

## Development notes

- This package is where performance matters most because it sits on the hot path.
- Keep blocking work out of `handleTick` when possible.
- Batch external writes where possible so the Modbus loop stays responsive.
- Keep event shapes stable because both SQL and the frontend depend on them.
