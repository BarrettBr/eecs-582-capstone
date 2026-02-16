package main

/*
Name: ingest/main.go
Description: Entrypoint for the ingest service; loads configuration, applies migrations, and runs the Modbus ingest loop until shutdown.
Programmer: Barrett Brown
Date Created: 2026-02-01
Dates Revised: 2026-02-15
Revision History:
- 2026-02-01, Barrett Brown: Created the file and the base structure
- 2026-02-05, Barrett Brown: Added migrations and sqlc framework
- 2026-02-15, Barrett Brown: Added standardized prologue documentation block.
Preconditions:
- Runtime environment can provide required configuration values and filesystem access.
- SQLite driver and migration files are available.
Acceptable Input Values/Types:
- Environment variables (string values) for config keys; missing values use documented defaults.
- OS interrupt/termination signals for graceful shutdown.
Unacceptable Input Values/Types:
- Invalid config values (for example malformed duration or invalid register base) causing startup failure.
- Missing or unreadable migration path/database path.
Postconditions:
- Service either runs ingest loop successfully or exits with a fatal startup/runtime error.
Return Values/Types:
- main: no return value.
- runMigrations: error (nil on success; non-nil on dialect/migration failures).
Error/Exception Conditions:
- Config load failures, DB open/ping failures, migration failures, or ingest loop fatal errors.
Side Effects:
- Opens database connection, executes schema migrations, starts network polling loop, writes logs.
Invariants:
- Ingest loop only starts after successful configuration and migration setup.
Known Faults:
- Fatal logging exits process immediately instead of attempting recovery.
*/

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/BarrettBr/eecs-582-capstone/internal/config"
	"github.com/BarrettBr/eecs-582-capstone/internal/ingest"
	"github.com/joho/godotenv"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

// description: Bootstraps config, runs DB migrations, and starts the ingest loop until shutdown.
// input: None (reads runtime configuration from environment and defaults).
// output: None (process exits on fatal startup/runtime errors).
func main() {
	// Load .env if it exists.
	_ = godotenv.Load()

	appCfg, err := config.Load()
	if err != nil {
		log.Fatalf("Error building config: %v", err)
	}
	defer appCfg.DB.Close()

	if err := runMigrations(appCfg.DB, appCfg.MigrationsPath); err != nil {
		log.Fatalf("Error running migrations: %v", err)
	}

	// Create context that is canceled upon ctrl + c so it cancels the modbus loop
	modbusLoop := ingest.NewModbusLoop(
		appCfg.Queries,
		appCfg.ModbusPollInterval,
		appCfg.ModbusAddress,
		appCfg.ModbusMWBase,
		appCfg.ValidationFilePath,
	)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Printf("Modbus service ready. Poll interval: %s", appCfg.ModbusPollInterval)
	if err := modbusLoop.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatalf("Error running ingest loop: %v", err)
	}
}

// description: Applies goose migrations to the configured SQLite database.
// input: db (open SQL connection), migrationsPath (filesystem path to migration files).
// output: Returns nil on success, or an error if dialect setup/migration fails.
func runMigrations(db *sql.DB, migrationsPath string) error {
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("Error setting goose dialect: %w", err)
	}
	if err := goose.Up(db, migrationsPath); err != nil {
		return fmt.Errorf("Error, goose up: %w", err)
	}
	return nil
}
