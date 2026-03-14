# Ingest Quick Guide

## What ingest is responsible for

The `ingest/` service is the backend that sits between source data and the rest of the platform.

It does four main jobs:

1. read source data from simulator or Modbus services
2. normalize and validate that data into a consistent shape
3. buffer and pace that data so one noisy source does not overwhelm everything else
4. fan the accepted events out to SQL, ML, and websocket consumers

## The shortest mental model

Big picture:

1. `main.go` wires everything together
2. `Registrar` owns source service lifecycle
3. each `service` reads one source and produces typed events
4. `RuleEngine` validates those events
5. `BufferManager` buffers and fairly schedules them
6. `Pipeline` sends them to SQL, ML, and websocket sinks

## The files that matter first

If you are new, start with these files in roughly this order:

- `ingest/main.go`
  - startup wiring
- `ingest/internal/ingest/runtime/registrar.go`
  - owns service add, remove, restart, and config reload behavior
- `ingest/internal/ingest/runtime/service.go`
  - runs one ingest service for one source
- `ingest/internal/ingest/runtime/buffer_manager.go`
  - shared buffering, fairness, pressure, and overload behavior
- `ingest/internal/ingest/runtime/pipeline.go`
  - shared downstream publisher
- `ingest/internal/ingest/runtime/fanout.go`
  - actual SQL, ML, and websocket worker behavior
- `ingest/internal/ingest/validation/engine.go`
  - validation rule loading and record checking
- `ingest/internal/config/sources.go`
  - source config schema

If you read only those files, you will understand most of the backend structure.

## What each part does

### `main.go`

This is the wiring layer.

It loads env config, opens SQLite, starts the websocket server, builds the shared pipeline, builds the `BufferManager`, builds the `Registrar`, and then runs the backend.

If startup is broken, this is usually the first file to inspect.

### `Registrar`

The `Registrar` is the lifecycle owner for source services.

It:

- loads the current source catalog
- starts enabled services
- watches the source config file
- adds, removes, or restarts services when config changes
- merges service state with buffer state for the status API
- pushes updated service catalogs into the websocket stream layer so removed services can be pruned from active subscriptions

If you want to know "who creates services and who shuts them down", this is that.

### `service.go`

Each service handles one source definition.

A service:

- polls Modbus or simulator input
- normalizes raw values into a typed `RecordEvent`
- validates that record
- applies fault injection or anomaly labels if needed
- submits the event into the shared `BufferManager`

### `BufferManager`

This is the shared buffering and fairness layer.

It exists so the system can absorb short bursts without letting one service starve the rest.

It:

- keeps one queue per service
- gives each service reserved capacity
- lets services borrow from shared capacity during bursts
- tracks pressure states like `normal`, `elevated`, `high`, and `critical`
- applies overload behavior like service-local `drop_oldest`
- uses round-robin dispatch so busy services do not monopolize downstream delivery

If you want to understand overload behavior, start here.

### `Pipeline` and `fanout.go`

The pipeline is the shared downstream delivery layer.

After the `BufferManager` admits an event, the pipeline fans it out to:

- SQLite persistence
- ML batching and HTTP delivery
- websocket streaming

One useful detail:

- websocket JSON is now encoded only in the websocket path
- the ingest service does not eagerly JSON-encode every event at ingress anymore
- live websocket and ML messages now keep `service_name` so the frontend can subscribe to specific services instead of receiving all traffic

That keeps the hot path lighter.

### Validation and record types

The typed event shapes live in:

- `ingest/internal/ingest/events/temp_sample.go`
- `ingest/internal/ingest/events/valve_sample.go`
- `ingest/internal/ingest/events/record_event.go`

Validation logic lives in:

- `ingest/internal/ingest/validation/engine.go`

The pattern is:

- source data becomes a typed record
- validation rules check that record
- validated record moves into buffering and fanout

## How data moves through the backend

Here is the normal flow from source read to dashboard update:

1. a source service wakes up on its poll interval
2. it reads simulator or Modbus data
3. it normalizes that into a typed record
4. validation checks required fields, bounds, and anomaly rules
5. the service submits the event to `BufferManager`
6. `BufferManager` queues it using reserved units first, then shared units if needed
7. the dispatcher forwards events into the shared pipeline
8. the pipeline sends the event to SQL, ML, and websocket sinks
9. the websocket streamer batches outgoing messages by service room and sends each room only to subscribed dashboard clients

That is the main request free data path.

## How config fits in

The main ingest source config file is:

- `ingest/config/sources.json`

That file is simulator-first by default, and it can also define named profiles such as `modbus`.

- without `SOURCE_CONFIG_PROFILE`, ingest uses the base `enabled` flags in the file
- with `SOURCE_CONFIG_PROFILE=modbus`, ingest applies the `modbus` profile after loading the same file

That file defines:

- which sources exist
- whether each source is enabled
- which adapter it uses
- flow control settings like reserved units and overload policy
- runtime buffering settings like shared units and pressure thresholds
- optional named source enablement profiles

The registrar watches the configured source file and hot reloads it.

That means source changes usually do not require a full process restart.

## What to look at when something breaks

If you are debugging, here is the fast checklist:

- startup or wiring problem
  - check `ingest/main.go`
- source not starting or not reloading
  - check `registrar.go`
- one source behaving strangely
  - check `service.go` and the source entry in the config file pointed to by `SOURCE_CONFIG_PATH`
- events being dropped or sampled
  - check `buffer_manager.go`
- ML not receiving batches
  - check `fanout.go`
- websocket live updates missing
  - check `fanout.go` and `ingest/internal/stream`
- validation rejecting records
  - check `validation/engine.go` and the validation JSON file

Also use:

- `GET /api/v1/ingestion/status`

That endpoint is the fastest way to see:

- current pressure state
- service queue usage
- shared unit usage
- sampled, dropped, and evicted counters
- registrar lifecycle state

## Good reading order for a new teammate

If you are onboarding someone, this order works well:

1. `docs/dev-run-guide.md`
2. this file
3. `ingest/main.go`
4. `docs/ingest-main-package.md`
5. `docs/ingest-ingest-package.md`
6. `docs/ingest-api-guide.md`

That order usually gives enough context before diving into deeper runtime files.

## Final takeaway

The ingest backend looks larger than it really is because the runtime is split into a few focused pieces.

The main idea is simple:

- registrar manages services
- services produce typed records
- BufferManager keeps buffering fair and bounded
- pipeline sends accepted events to the rest of the system

Once that clicks, the rest of the file layout is much easier to follow.
