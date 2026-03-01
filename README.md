# eecs-582-capstone

Note to Grader: School documents are in `docs/School`.

An AI-assisted industrial process monitoring and control demonstrator.

## Quick Tour

- `docs/` Project notes and documentation. See `docs/School` for course documents.
- `ingest/` Data ingestion pipeline and SQL schema/migrations (View `docs/ingest-sql.md` for a guide on SQL with Go).
- `ml/` Model training and artifacts.
- `web/` Web app frontend and static assets.

## Running Locally

Local run instructions for both simulator mode and real Modbus/OpenPLC mode are in:

- `docs/dev-run-guide.md`
- SQL Check: `cd ingest && sqlite3 data/app.db "select count(*) from temp_samples;"` will show a higher number than before. Delete app.db to reset this from 0 if wanting to clear any previous data

## Quick Start

Install the frontend dependencies once:

```bash
make deps
```

Run the full local stack in simulator mode:

```bash
make dev
```

That starts:

- ingest (using `ingest/config/sources.json`)
- the ML API
- the frontend dev server

To run against a real Modbus/OpenPLC source instead:

```bash
make dev-modbus
```

## OpenPLC Reference

Detailed OpenPLC Runtime v4 setup notes and troubleshooting are in:

- `docs/openplc-reference.md`

## Resources

- [Autonomy, OpenPLC downloads](https://autonomylogic.com/runtime)
