package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/BarrettBr/eecs-582-capstone/internal/database"
	"github.com/BarrettBr/eecs-582-capstone/internal/ingest"
	"github.com/joho/godotenv"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

type cfg struct {
	DB                 *sql.DB
	Queries            *database.Queries
	Secret             string
	ModbusPollInterval time.Duration
}

func main() {
	// Load .env if it exists.
	_ = godotenv.Load()

	appCfg, err := newConfig()
	if err != nil {
		log.Fatalf("Error building config: %v", err)
	}
	defer appCfg.DB.Close()

	// TODO: Break out migration path into a config.go file
	if err := runMigrations(appCfg.DB, "sql/schema"); err != nil {
		log.Fatalf("Error running migrations: %v", err)
	}

	// Create context that is canceled upon ctrl + c so it cancels the modbus loop
	modbusLoop := ingest.NewModbusLoop(appCfg.Queries, appCfg.ModbusPollInterval)
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Printf("Modbus service ready. Poll interval: %s", appCfg.ModbusPollInterval)
	if err := modbusLoop.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatalf("Error running ingest loop: %v", err)
	}
}

func newConfig() (*cfg, error) {
	// SQLite Setup resouces:
    // https://www.twilio.com/en-us/blog/developers/community/use-sqlite-go
    dbPath := os.Getenv("SQLITE_PATH")
	if dbPath == "" {
		dbPath = "./data/app.db"
	}

	if err := ensureSQLiteFile(dbPath); err != nil {
		return nil, fmt.Errorf("Error ensuring sqlite db file: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("Error opening sqlite db: %w", err)
	}

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("Error pinging sqlite db: %w", err)
	}

	modbusPollInterval := 5 * time.Second
	if v := os.Getenv("MODBUS_POLL_INTERVAL"); v != "" {
		parsed, err := time.ParseDuration(v)
		if err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("Parse MODBUS_POLL_INTERVAL Error: %w", err)
		}
		modbusPollInterval = parsed
	}

	return &cfg{
		DB:                 db,
		Queries:            database.New(db),
		Secret:             os.Getenv("SECRET"),
		ModbusPollInterval: modbusPollInterval,
	}, nil
}

func ensureSQLiteFile(dbPath string) error {
	dir := filepath.Dir(dbPath)
	if dir != "" {
        // rwx rx rx
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("Mkdir error %s: %w", dir, err)
		}
	}

    // Check if file already exists
	if _, err := os.Stat(dbPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("Stat error %s: %w", dbPath, err)
	}

    // Otherwise create it
	f, err := os.OpenFile(dbPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("Error creating SQLite file %s: %w", dbPath, err)
	}
	return f.Close()
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
