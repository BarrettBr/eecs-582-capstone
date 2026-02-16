package config

/*
Name: ingest/internal/config/config.go
Description: Loads, validates, and materializes runtime configuration and shared dependencies for the ingest service.
Programmer: Barrett Brown
Date Created: 2026-02-01
Dates Revised: 2026-02-15
Revision History:
- 2026-02-01, Barrett Brown: Created File
- 2026-02-13, Barrett Brown: Expanded file to include modbus address offset
- 2026-02-15, Barrett Brown: Added standardized prologue documentation block.
Preconditions:
- Environment variables and configured filesystem paths are readable.
- Process has permission to create database directory/file when absent.
Acceptable Input Values/Types:
- String environment variables for paths/addresses/secrets.
- Duration strings parseable by Go time.ParseDuration.
- MODBUS_MW_BASE integer in range [0, 65535].
Unacceptable Input Values/Types:
- Non-parseable duration strings.
- Register base outside [0, 65535] or non-numeric input.
- Unwritable database path or unreadable migration/validation paths.
Postconditions:
- Returns initialized Config with open DB handle and query helper, or an error.
Return Values/Types:
- Load: (*Config, error)
- Helper functions return parsed values and/or error.
Error/Exception Conditions:
- File creation/stat failures, DB open/ping failures, and environment parse errors.
Side Effects:
- Creates SQLite directories/files as needed and opens a live DB connection.
Invariants:
- Successful Load always returns non-nil DB and Queries in Config.
Known Faults:
- Config field naming/style is not fully uniform (e.g., ValidationFilePath local variable capitalization).
*/

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/BarrettBr/eecs-582-capstone/internal/database"
	_ "modernc.org/sqlite"
)

// description: Runtime configuration and shared dependencies for the ingest service.
// input: Populated by Load using environment variables and defaults.
// output: Used by main to initialize DB, Modbus polling, and validation settings.
type Config struct {
	DB                 *sql.DB
	Queries            *database.Queries
	Secret             string
	SQLitePath         string
	MigrationsPath     string
	ModbusPollInterval time.Duration
	ModbusAddress      string
	ModbusMWBase       uint16 // uint since modbus usually expects these
	ValidationFilePath string
}

// description: Builds and validates service configuration, opens SQLite, and initializes query helpers.
// input: None (reads environment variables with fallback defaults).
// output: Returns a fully initialized Config or an error.
func Load() (*Config, error) {
	sqlitePath := getEnv("SQLITE_PATH", "./data/app.db")
	migrationsPath := getEnv("MIGRATIONS_PATH", "sql/schema")
	modbusAddress := getEnv("MODBUS_ADDRESS", "127.0.0.1:1502")
	ValidationFilePath := getEnv("VALIDATION_FILE_PATH", "config/validation_rules.json")

	if err := ensureSQLiteFile(sqlitePath); err != nil {
		return nil, fmt.Errorf("Error ensuring sqlite db file: %w", err)
	}

	db, err := sql.Open("sqlite", sqlitePath)
	if err != nil {
		return nil, fmt.Errorf("Error opening sqlite db: %w", err)
	}

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("Error pinging sqlite db: %w", err)
	}

	modbusPollInterval, err := getDurationEnv("MODBUS_POLL_INTERVAL", 5*time.Millisecond)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	modbusMWBase, err := getUint16Env("MODBUS_MW_BASE", 1024)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	return &Config{
		DB:                 db,
		Queries:            database.New(db),
		Secret:             os.Getenv("SECRET"),
		SQLitePath:         sqlitePath,
		MigrationsPath:     migrationsPath,
		ModbusPollInterval: modbusPollInterval,
		ModbusAddress:      modbusAddress,
		ModbusMWBase:       modbusMWBase,
		ValidationFilePath: ValidationFilePath,
	}, nil
}

// description: Reads a string environment variable with a fallback value.
// input: key (env var name), fallback (value used when unset/empty).
// output: Returns env value if present, otherwise fallback.
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// description: Reads a duration environment variable and parses it.
// input: key (env var name), fallback (duration used when unset).
// output: Returns parsed duration or fallback; returns error on invalid duration format.
func getDurationEnv(key string, fallback time.Duration) (time.Duration, error) {
	if v := os.Getenv(key); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return 0, fmt.Errorf("Parse MODBUS_POLL_INTERVAL Error %s: %w", key, err)
		}
		return d, nil
	}
	return fallback, nil
}

// description: Reads a uint16 environment variable with range validation.
// input: key (env var name), fallback (value used when unset).
// output: Returns parsed uint16 or fallback; returns error on parse/range failure.
func getUint16Env(key string, fallback uint16) (uint16, error) {
	if v := os.Getenv(key); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return 0, fmt.Errorf("Parse %s Error: %w", key, err)
		}
		if n < 0 || n > 65535 {
			return 0, fmt.Errorf("%s must be between 0 and 65535", key)
		}
		return uint16(n), nil
	}
	return fallback, nil
}

// description: Ensures the SQLite file and its parent directory exist.
// input: dbPath (target SQLite file path).
// output: Returns nil when file is present/created, or an error on filesystem failures.
func ensureSQLiteFile(dbPath string) error {
	dir := filepath.Dir(dbPath)
	if dir != "" {
		// rwx rx rx
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("Mkdir Error %s: %w", dir, err)
		}
	}

	if _, err := os.Stat(dbPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("Stat Error %s: %w", dbPath, err)
	}

	f, err := os.OpenFile(dbPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("Error creating SQLite file %s: %w", dbPath, err)
	}
	return f.Close()
}
