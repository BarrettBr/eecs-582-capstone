package main

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
	modbusLoop := ingest.NewModbusLoop(appCfg.Queries, appCfg.ModbusPollInterval, appCfg.ModbusAdress)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Printf("Modbus service ready. Poll interval: %s", appCfg.ModbusPollInterval)
	if err := modbusLoop.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatalf("Error running ingest loop: %v", err)
	}
}

// Runs goose migrations on sqlite file
func runMigrations(db *sql.DB, migrationsPath string) error {
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("Error setting goose dialect: %w", err)
	}
	if err := goose.Up(db, migrationsPath); err != nil {
		return fmt.Errorf("Error, goose up: %w", err)
	}
	return nil
}
