# Ingest Database Package Guide

## What this package is

The database package is the generated SQL layer for ingest.

It is built by `sqlc` from the files in `ingest/sql/queries` and `ingest/sql/schema`. This is all generated, so don't handwrite anything here.

## Main flow

The service works with this package like this

1. Migrations define the schema
2. Query files define named SQL operations
3. `sqlc generate` builds Go code into `ingest/internal/database`
4. The rest of the app calls those generated methods

## What it owns

- Typed query methods
- Typed query parameter structs
- Typed model structs
- Transaction compatible query wrapper through `WithTx`

## Key files

- `db.go`
  - Basic query wrapper and `WithTx`
- `querier.go`
  - Generated interface for all query methods
- `models.go`
  - Generated row model types
- `devices.sql.go`
  - Generated methods for samples and anomaly persistence

## How the rest of the service uses it

- `main.go` builds `database.New(db)` once
- The ingest package stores that query object in `ModbusLoop`
- SQL fanout uses generated methods inside a transaction for batched writes

## How to extend it

To add new database behavior

1. Add or update a migration in `ingest/sql/schema`
2. Add or update a query in `ingest/sql/queries`
3. Run `sqlc generate`
4. Use the new generated method from the app

Do not hand edit files in `ingest/internal/database` because they will be overwritten.

## Development notes

- If something looks wrong here, check the SQL source files first.
- If a generated method is missing, it usually means the query file was not updated or `sqlc generate` was not run.
