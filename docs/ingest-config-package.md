# Ingest Config Package Guide

## What this package is

The config package reads runtime settings from env vars and prepares shared dependencies for the ingest service.

This is the package that turns plain strings from the environment into real typed values the rest of the service can use.

## Main flow

`config.Load()` is the main entry point.

It does this work

1. Reads env vars with defaults
2. Ensures the SQLite file exists
3. Opens the database
4. Applies SQLite performance settings
5. Parses timing, batching, and sampling settings
6. Builds the shared config struct

That config struct is then passed into the main package and used to start the rest of the service.

## What it owns

- Env parsing
- Default values
- SQLite file setup
- SQLite PRAGMA tuning
- Building the shared `Config` object

## Important sections

- `getEnv`
  - Simple string fallback helper
- `getDurationEnv`
  - Parses duration strings like `25ms`
- `getIntEnv`
  - Used for batch and sampling counts
- `getBoolEnv`
  - Used for feature switches and overload behavior
- `applySQLiteTuning`
  - Applies WAL mode and synchronous settings

## How to extend it

If you add a new runtime _knob_, add it here first.

The normal pattern is

1. Add a field to `Config`
2. Read the env var in `Load`
3. Parse and validate it
4. Store it in the returned config
5. Pass it into the package that needs it from `main.go`

## Development notes

- Keep parsing and validation here so other packages can assume they receive safe values.
- If a value is invalid, fail here instead of letting runtime logic fail later.
