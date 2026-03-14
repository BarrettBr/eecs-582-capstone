# Ingest Core Package Guide

## What this package is

This is the main application logic for the ingest service.

It now owns four runtime pieces that work together:

- registrar managed source services
- source adapters plus shared event and validation packages
- BufferManager buffering and overload control
- shared downstream fanout

If you want to understand how one PLC sample turns into stored data and frontend updates, this is the package to read first.

For the full backend reading order and end-to-end flow, use `docs/ingest-quick-guide.md` first. This guide stays focused on what is specific to the core ingest package itself.

## What this package owns

- source normalization for simulator and Modbus inputs
- event contracts in `events/`
- validation logic in `validation/`
- service lifecycle under the registrar
- shared buffering and overload control
- shared downstream fanout to ML, SQL, and websocket sinks

## Main pieces

- `runtime/modbus_loop.go`
  - Shared normalization helpers retained from the original loop implementation
- `runtime/service.go`
  - Managed source runtime and pressure aware polling behavior
- `runtime/registrar.go`
  - Source lifecycle ownership and hot reload handling
- `runtime/buffer_manager.go`
  - Per service queues, shared unit accounting, fairness, and pressure state tracking
- `runtime/pipeline.go`
  - Shared downstream fanout owner
- `events/temp_sample.go`
  - Current normalized temperature event shape
- `events/valve_sample.go`
  - Current normalized valve event shape
- `events/record_event.go`
  - Generic event interface used by validation and fanout
- `validation/engine.go`
  - Rule loading and record validation
- `runtime/fanout.go`
  - Sink worker loops and shared sink delivery logic

## One concrete example

For one temperature sample, the practical flow is:

1. `runtime/service.go` reads from one source.
2. `runtime/modbus_loop.go` normalizes the raw input into an event in `events/`.
3. `validation/engine.go` checks required fields, bounds, and anomaly rules.
4. `runtime/buffer_manager.go` decides whether the event is buffered, sampled, or evicted.
5. `runtime/fanout.go` sends admitted events to SQL, ML, and websocket paths.

The quick guide covers the bigger system flow in more detail, so this guide does not repeat all of that machinery again.

## How to extend it

If you want to add a new event type later

1. Create a new struct that implements `RecordEvent`
2. Put it in `events/`
3. Add the normalization logic that builds it
4. Update validation rules if needed
5. Update sink handling where that event type should go

If you want to add a new sink

1. Add a new bounded queue to `Pipeline`
2. Start a new worker in `startWorkers`
3. Add publish logic in `Pipeline.Publish`
4. Implement the sink delivery function

## Development notes

- This package is where performance matters most because it sits on the hot path.
- Use `docs/backend-performance-review.md` for the current optimization priorities instead of duplicating them here.
- Keep blocking work out of individual service read paths when possible.
- Batch external writes where possible so `BufferManager` can absorb short bursts cleanly.
- Keep event shapes stable because both SQL and the frontend depend on them.
- Keep overload rules simple and local. `BufferManager` is the only place that should make fairness and eviction decisions.
