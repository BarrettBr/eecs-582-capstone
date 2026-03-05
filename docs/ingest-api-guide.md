# Ingest API Guide

This guide explains where to add HTTP API endpoints in ingest while keeping websocket behavior unchanged.

## Current Routing

- Websocket endpoint: `/ws`
- Versioned API base path: `/api/v1` (configurable via `API_BASE_PATH`)
- Current API endpoint: `GET /api/v1/ping`
- Dynamic history endpoint: `GET /api/v1/history/{kind}/{window}`
- Dynamic report endpoint: `GET /api/v1/report/{kind}/{window}`

## Where to Add Endpoints

Add routes in:

- `ingest/internal/api/routes.go`
- Function: `RegisterRoutes(registrar HTTPRegistrar, cfg Config)`

`main.go` should only call this package:

```go
apiRegistration := api.RegisterRoutes(wsServer, api.Config{
	BasePath: appCfg.Stream.APIBasePath,
})
```

## History and Report Routing

The route table in `ingest/internal/api/routes.go` registers:

```go
"GET " + path.Join(apiCfg.basePath, "history", "{kind}", "{window}"): apiCfg.historyHandler
"GET " + path.Join(apiCfg.basePath, "report", "{kind}", "{window}"):  apiCfg.reportHandler
```

`historyHandler` and `reportHandler` perform the main switching logic using:

- kind: `temp`, `valve`
- window: `hour`, `day`, `week`

They use a two-step switch in `ingest/internal/api/history.go`:

1. switch on `kind` to choose service type
2. switch on `window` to choose range

That fills the `querySelection` struct that a function runs.

Examples:

- `http://127.0.0.1:8080/api/v1/history/temp/hour`
- `http://127.0.0.1:8080/api/v1/report/valve/week`

## Extensible Pattern

The API package uses:

- key: method-aware ServeMux pattern (`GET /api/v1/...`, `POST /api/v1/...`)
- value: handler method on `apiConfig`
- per-endpoint switch logic in `history.go` to build `querySelection`

As routes grow, split files by concern inside `ingest/internal/api`:

- `health.go` for health/ping
- `history.go` for history/query endpoints
- `admin.go` for admin-only endpoints

Keep the base path in config (`API_BASE_PATH`) so versioning stays centralized for future `/api/v2`.

## Adding A New Kind (Example: `nuclear`)

To add `nuclear` with minimal repetition:

1. Add one case in the `kind` switch in `buildQuerySelection(...)`:

- set the data target fields for `nuclear`

2. Keep using existing `window` switch unless you need a new time bucket option.

No route changes are required because the dynamic path already supports new kinds.

## Config

Relevant environment variables:

- `WS_PATH=/ws`
- `API_BASE_PATH=/api/v1`

Update `.env.example` and deployment env (for example `docker-compose.yml`) when changing API version paths.
