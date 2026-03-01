package main

/*
Name: ingest/main.go
Description: Starts the ingest app, loads config, runs DB migrations, and keeps the Modbus loop running.
Programmer: Barrett Brown
Date Created: 2026-02-01
Dates Revised: 2026-02-28
Revision History:
- 2026-02-01, Barrett Brown: Created the file and the base structure
- 2026-02-05, Barrett Brown: Added migrations and sqlc framework
- 2026-02-15, Barrett Brown: Added standardized prologue documentation block.
- 2026-02-28, Barrett Brown: Added DB to modbus loop for use in fanout
Preconditions:
- Env vars are available (or defaults can be used).
- DB file path and migration files can be accessed.
Acceptable Input Values/Types:
- String env var values for config.
- Normal shutdown signals like Ctrl+C/SIGTERM.
Unacceptable Input Values/Types:
- Bad config values (like invalid duration or register number).
- Missing DB/migration paths.
Postconditions:
- Service runs and polls Modbus, or exits with an error.
Return Values/Types:
- main: no return value.
- runMigrations: error (nil = success, non-nil = failure).
Error/Exception Conditions:
- Config, DB, migration, or ingest startup/runtime failures.
Side Effects:
- Opens DB, runs migrations, starts polling loop, writes logs.
Invariants:
- Ingest loop only starts after config and migrations succeed.
Known Faults:
- Uses fatal exits instead of trying to recover.
*/

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/BarrettBr/eecs-582-capstone/internal/config"
	"github.com/BarrettBr/eecs-582-capstone/internal/ingest"
	"github.com/BarrettBr/eecs-582-capstone/internal/stream"
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

	// Run database migrations
	if err := runMigrations(appCfg.DB, appCfg.MigrationsPath); err != nil {
		log.Fatalf("Error running migrations: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	wsServer := stream.NewServer(stream.Config{
		Addr:               appCfg.WSAddress,
		Path:               appCfg.WSPath,
		BatchSize:          appCfg.WSBatchSize,
		BatchFlushInterval: appCfg.WSBatchFlushInterval,
	})
	// Create context that is canceled upon ctrl + c so it cancels the modbus loop
	modbusLoop := ingest.NewModbusLoop(
		appCfg.DB,
		appCfg.Queries,
		appCfg.ModbusPollInterval,
		appCfg.SourceConfigPath,
		ingest.MLFanoutConfig{
			APIURL:             appCfg.MLAPIURL,
			HTTPTimeout:        appCfg.MLHTTPTimeout,
			BatchSize:          appCfg.MLBatchSize,
			BatchFlushInterval: appCfg.MLBatchFlushInterval,
			DropOnOverload:     appCfg.MLDropOnOverload,
		},
		ingest.SQLFanoutConfig{
			BatchSize:          appCfg.SQLBatchSize,
			BatchFlushInterval: appCfg.SQLBatchFlushInterval,
			NormalSampleRate:   appCfg.SQLNormalSampleRate,
		},
		wsServer,
	)

	type faultInjectRequest struct {
		SourceName string `json:"source_name"`
	}
	wsServer.RegisterReadHandler("inject_fault", func(_ context.Context, msg stream.Message) {
		var req faultInjectRequest
		if len(msg.Data) > 0 {
			if err := json.Unmarshal(msg.Data, &req); err != nil {
				log.Printf("Fault injection decode error: %v", err)
				return
			}
		}
		if err := modbusLoop.TriggerFaultInjection(req.SourceName); err != nil {
			log.Printf("Fault injection request failed: %v", err)
			return
		}
		log.Printf("Fault injection triggered for source=%q", req.SourceName)
	})

	go func() {
		if err := wsServer.Start(ctx); err != nil {
			log.Printf("Websocket server error: %v", err)
		}
	}()

	log.Printf("Modbus service ready. Poll interval: %s", appCfg.ModbusPollInterval)
	log.Printf("Websocket stream ready at %s", wsServer.URL())
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
