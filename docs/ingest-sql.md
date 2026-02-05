# Ingest: Migrations + sqlc Guide

## Why / What is this?

- This auto generates all the type safe code for you so you only have to focus on your cool sql statements not hooking up a million things
- Also lets you very easily revert / migrate to different versions so if something breaks it won't destroy it all
- Honestly it is very simple (Like 4 commands) to run to apply / enable the sql statements you make, this is listed at the bottom for easy reference, below is the layout / guide to this all

## Folder layout (ingest)

- `ingest/sql/schema/`
  - Goose migration files live here. Example: `001_users.sql`.
- `ingest/sql/queries/`
  - sqlc query files live here. Example: `users.sql`.
- `ingest/internal/database/`
  - Generated Go code from sqlc. You don’t edit these files directly.
- `ingest/main.go`
  - Runs goose migrations on startup and builds the sqlc query object.

## 1) Adding a new migration

Migrations are versioned SQL files that change the database schema.

Steps:

1. Create a new file in `ingest/sql/schema/` with the next number, for example:
   - `002_devices.sql`
2. Put your schema change inside the goose blocks:

```sql
-- +goose Up
CREATE TABLE devices (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- +goose Down
DROP TABLE devices;
```

Notes:

- `-- +goose Up` is applied when you run migrations.
- `-- +goose Down` is used to roll back (if you ever need to). For this it will just be the opposite of what you did in the Up
- Keep SQLite syntax (not Postgres). Example: use `TEXT`, `DATETIME`, `CURRENT_TIMESTAMP`.

## 2) Adding a new sqlc query

sqlc generates typed Go code from SQL queries so you don’t write raw SQL in Go.

Steps:

1. Create or edit a query file in `ingest/sql/queries/`.
2. Add a named query using sqlc format, for example:

```sql
-- name: CreateDevice :one
INSERT INTO devices (id, name)
VALUES (?, ?)
RETURNING id, name, created_at, updated_at;
```

Notes:

- The `-- name:` line defines the Go method name and result type. (What we call in the main.go / other file to do this)
- Use `?` placeholders for SQLite.
- `:one` means a single row is returned.
- Other options include `:many` (list), `:exec` (no rows).

## 3) Generate sqlc code

From the `ingest/` folder:

```bash
sqlc generate
```

This updates `ingest/internal/database/` with new Go methods.

## 4) How migrations and sqlc are used in code

### Migrations

`ingest/main.go` runs migrations on startup:

```go
if err := runMigrations(appCfg.DB, "sql/schema"); err != nil {
    log.Fatalf("Error running migrations: %v", err)
}
```

So whenever the service starts, it applies any new migration files.

### sqlc queries

`ingest/main.go` creates a sqlc query object once:

```go
Queries: database.New(db),
```

That `Queries` object is passed around and used in other packages, for example in the Modbus loop:

```go
modbusLoop := ingest.NewModbusLoop(appCfg.Queries, appCfg.ModbusPollInterval)
```

Inside that loop you can call generated methods like:

```go
user, err := m.queries.LookupUser(ctx, "alice")
```

## 5) Common mistakes and fixes

- “migration path does not exist”
  - Run from the `ingest/` folder so `sql/schema` is found.

- “unknown column / table / what not”
  - You added a query but didn’t add the migration (or didn’t run migrations yet).
  - This shouldn't be an issue since I added the auto-migration bit but just in case you running purposely on a lower one

- sqlc errors about syntax
  - Make sure you use SQLite syntax and `?` parameters. PostgreSQL and other might not use the same time / other bits you use

## 6) Quick checklist

When you add a new feature involving the database:

1. Add a migration in `ingest/sql/schema/`.
2. Add a query in `ingest/sql/queries/`.
3. Run `sqlc generate` from `ingest/`.
4. Use the generated Go method from `ingest/internal/database`.

That’s it.

## Resources

- [SQLC How to Guide](https://docs.sqlc.dev/en/latest/howto/select.html): Nice if looking up how to do something i.e: DELETE and it isn't listed
- [Goose](https://github.com/pressly/goose): Nice if wanting to look into the migration library more, shouldn't really need to since most is already done
