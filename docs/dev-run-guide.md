# Dev Run Guide

This project supports two primary local run modes:

- `Simulator mode`: no PLC/OpenPLC required. Ingest generates temperature data in-process.
- `Modbus mode`: ingest reads from a real Modbus TCP source such as OpenPLC.

## Prerequisites

- Go installed and available as `go`
- Python 3 installed and available as `python3`
- Node.js installed (see `web/package.json` for the supported versions)
- Web dependencies installed once via `make deps` or `cd web && npm install`

## One-Command Launch

From the repository root:

- `make dev`
  - Starts ingest in simulator mode
  - Starts the ML API on `http://127.0.0.1:8000`
  - Starts the Vite frontend dev server on the host network

- `make dev-modbus`
  - Starts ingest using the checked-in Modbus source configuration
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
- A built-in simulator emits temperature records on a timer
- Validation uses `config/validation/temperature.json`
- Events are written to SQLite, streamed over websocket, and sent to the ML API
- The dashboard includes an `Inject Fault` button that forces a short burst of anomalous simulator readings

Useful manual commands:

```bash
cd ingest && SOURCE_CONFIG_PATH=config/sources.json go run .
python3 ml/app/main.py --serve
cd web && npm run dev -- --host
```

## Modbus / OpenPLC Mode

Modbus mode uses the dedicated checked-in config:

- Source config file: `ingest/config/sources.modbus.json`

By default this file:

- Disables `temp_dev`
- Enables `temp_plc`
- Leaves `valve_plc` disabled until you want to turn it on

Run it:

```bash
make dev-modbus
```

If you want valve data as well:

1. Edit `ingest/config/sources.modbus.json`
2. Set `valve_plc.enabled` to `true`
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

There are two checked-in source config files:

- `ingest/config/sources.json`
  - simulator-first local dev config
- `ingest/config/sources.modbus.json`
  - Modbus-first config for real PLC/OpenPLC use

The ingest service reads the file specified by `SOURCE_CONFIG_PATH`.

Examples:

```bash
cd ingest && SOURCE_CONFIG_PATH=config/sources.json go run .
cd ingest && SOURCE_CONFIG_PATH=config/sources.modbus.json go run .
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

## Troubleshooting

- If `make dev` fails immediately in the frontend:
  - Run `make deps` first

- If `make dev-modbus` fails in ingest:
  - Confirm your Modbus TCP server is reachable
  - Confirm the `address`, `mw_base`, and `register_count` values in `ingest/config/sources.modbus.json`

- If the frontend connects but shows no new events:
  - Check that ingest is running
  - Check that the selected source config has at least one enabled source
  - In Modbus mode, verify your PLC is actually updating the configured registers
