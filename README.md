# eecs-582-capstone

Note to Grader: School documents are in `docs/School`.

An AI-assisted industrial process monitoring and control demonstrator.

## Quick Tour

- `docs/` Project notes and documentation. See `docs/School` for course documents.
- `ingest/` Data ingestion pipeline and SQL schema/migrations (View `docs/ingest-sql.md` for a guide on SQL with Go).
- `ml/` Model training and artifacts.
- `frontend/` Web app frontend and static assets.

If you are trying to understand the ingest backend quickly, start with:

- `docs/ingest-quick-guide.md`
- `docs/dev-run-guide.md`

## Running Locally

Local run instructions for both simulator mode and real Modbus/OpenPLC mode are in:

- `docs/dev-run-guide.md`
- SQL Check: `cd ingest && sqlite3 data/app.db "select count(*) from temp_samples;"` will show a higher number than before. Delete app.db to reset this from 0 if wanting to clear any previous data

## Quick Start

Install dependencies once:

```bash
make deps
```

`make deps` installs frontend packages and Python ML dependencies into a repo-local `.venv`.

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

## How To Start Docker

Copy the example env file once (Or copy paste on windows):

```bash
cp .env.example .env
```

Start the full stack (Can do this through docker engine on windows):

```bash
sudo docker compose up -d --build
```

Check service status and logs:

```bash
sudo docker compose ps
sudo docker compose logs --tail=100 ingest frontend ml
```

Stop everything:

```bash
sudo docker compose down
```

Open the apps:

- Frontend: `http://localhost:5173`
- Ingest API ping: `http://localhost:8080/api/v1/ping`

If your machine uses legacy Compose instead of `docker compose`, use `docker-compose` with the same commands.

## References

- [Autonomy, OpenPLC downloads](https://autonomylogic.com/runtime)
- [Docker Engine install docs](https://docs.docker.com/engine/install/)
- [Docker Compose install docs](https://docs.docker.com/compose/install/)
- [Podman Compose](https://github.com/containers/podman-compose)
