# Dev Run Guide

This project supports two primary local run modes:

- `Simulator mode`: no PLC/OpenPLC required. Ingest generates temperature data in-process.
- `Modbus mode`: ingest reads from a real Modbus TCP source such as OpenPLC.

If you are new to the ingest backend itself, read `docs/ingest-quick-guide.md` after this file for a faster codebase map.

## Prerequisites

- Go installed and available as `go`
- Python 3 installed and available as `python3`
- Node.js installed (see `frontend/package.json` for the supported versions)
- Dependencies installed once via `make deps`

## One-Command Launch

From the repository root:

- `make dev`
  - Starts ingest in simulator mode
  - Starts the ML API on `http://127.0.0.1:8000`
  - Starts the Vite frontend dev server on the host network

- `make dev-modbus`
  - Starts ingest using the Modbus profile from the checked-in source configuration
  - Starts the ML API
  - Starts the frontend dev server

Use `Ctrl+C` to stop all three processes started by the Makefile target.

## Simulator Mode

Simulator mode is the default local development path.

Configuration:

- Source config file: `ingest/config/sources.json`
- Enabled source: `temp_dev`
- Disabled sources: `temp_plc`, `valve_plc`
- Default simulator interval: `100ms`

Run it:

```bash
make dev
```

What it does:

- Ingest loads `config/sources.json`
- The registrar watches that file and hot reloads source changes
- A built-in simulator emits temperature records on a timer
- Validation uses `config/validation/temperature.json`
- Events are written to SQLite, streamed over websocket, and sent to the ML API
- The dashboard includes an `Inject Fault` button that forces a short burst of anomalous simulator readings

Useful manual commands:

```bash
cd ingest && SOURCE_CONFIG_PATH=config/sources.json go run .
./.venv/bin/python ml/app/main.py --serve
cd frontend && npm run dev -- --host
```

## Modbus / OpenPLC Mode

Modbus mode uses the same checked-in source config file with the `modbus` profile selected.

That profile:

- disables `temp_dev`
- enables `temp_plc`
- leaves `valve_plc` disabled until you choose to add it

Run it:

```bash
make dev-modbus
```

If you want valve data as well:

1. Edit `ingest/config/sources.json`
2. Add `"valve_plc"` to `profiles.modbus.enabled_sources`
3. Adjust the Modbus address/register values to match your PLC register map
4. Run `make dev-modbus`

The default Modbus assumptions are:

- Temperature source:
  - base register `1024`
  - register count `6`
- Valve source:
  - base register `1100`
  - register count `5`

If your PLC uses different addresses or layouts, update the relevant source entry in the JSON file.

## Source Config Files

There is one checked-in source config file:

- `ingest/config/sources.json`
  - simulator-first local dev config with optional named profiles such as `modbus`

The ingest service reads the file specified by `SOURCE_CONFIG_PATH`.

If `SOURCE_CONFIG_PROFILE` is set, ingest applies that named profile after loading the file.

Examples:

- no profile
  - uses the base `enabled` flags in `sources.json`
- `SOURCE_CONFIG_PROFILE=modbus`
  - applies the `modbus` profile from `sources.json`

`sources.json` now has two major sections:

- `runtime.buffering`
  - global shared burst capacity in units
  - pressure thresholds
  - sampling trigger threshold
- `sources[]`
  - source identity and adapter settings
  - `flow_control` for reserved units, overload policy, and backpressure controls
- `profiles`
  - optional named source enablement presets such as `modbus`

When the file changes at runtime, the registrar will:

- start newly enabled sources
- stop removed sources
- restart sources whose config changed
- keep the last good catalog active if the new file is invalid

Examples:

```bash
cd ingest && SOURCE_CONFIG_PATH=config/sources.json go run .
cd ingest && SOURCE_CONFIG_PATH=config/sources.json SOURCE_CONFIG_PROFILE=modbus go run .
```

## Validation Files

Validation is now per event type:

- `ingest/config/validation/temperature.json`
- `ingest/config/validation/valve.json`

Each source entry points at the validation file it should use.

## ML API Contract

The ML service now accepts typed batches:

```json
{
  "event_type": "temperature",
  "samples": []
}
```

The same endpoint can also accept:

```json
{
  "event_type": "valve",
  "samples": []
}
```

Ingest groups outgoing ML batches by `event_type` before sending them.

## Ingestion Status API

The ingest API now exposes registrar and BufferManager state at:

```text
GET /api/v1/ingestion/status
```

The response includes:

- current pressure state
- total buffered units versus total capacity units
- shared units usage
- last reload time and last reload error
- one status object per service with buffer occupancy and drop or sampling counters

This endpoint is the quickest way to confirm hot reloads, pressure transitions, and service level degradation behavior.

## Overload Behavior

This guide only keeps the operational summary here.

Important runtime behavior:

- pressure states are `normal`, `elevated`, `high`, and `critical`
- `elevated` only raises observability signals
- backpressure capable services reduce poll rate only in `high` and `critical`
- non backpressure services can enter fixed rate sampling only when pressure is high enough, shared units are stressed, and that service is borrowing shared capacity
- `drop_oldest` only evicts the same service's buffered events

`SQL_NORMAL_SAMPLE_RATE` still controls downstream SQL persistence of non anomalous temperature records. It is separate from overload sampling.

If you want the full ingest flow and package map, read `docs/ingest-quick-guide.md`.

## Troubleshooting

- If `make dev` fails immediately in the frontend:
  - Run `make deps` first

- If `make dev-modbus` fails in ingest:
  - Confirm your Modbus TCP server is reachable
  - Confirm the `address`, `mw_base`, and `register_count` values in `ingest/config/sources.json`

- If the frontend connects but shows no new events:
  - Check that ingest is running
  - Check that the selected source config has at least one enabled source
  - In Modbus mode, verify your PLC is actually updating the configured registers

- If a source config edit does not seem to apply:
  - Call `GET /api/v1/ingestion/status`
  - Check `last_reload_error`
  - Confirm the edited file is the one pointed to by `SOURCE_CONFIG_PATH`
