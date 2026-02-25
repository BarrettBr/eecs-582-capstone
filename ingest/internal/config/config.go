package config

/*
Name: ingest/internal/config/config.go
Description: Reads app settings from env vars, sets defaults, and opens the SQLite DB.
Programmer: Barrett Brown
Date Created: 2026-02-01
Dates Revised: 2026-02-15
Revision History:
- 2026-02-01, Barrett Brown: Created File
- 2026-02-13, Barrett Brown: Expanded file to include modbus address offset
- 2026-02-15, Barrett Brown: Added standardized prologue documentation block.
Preconditions:
- Env vars can be read.
- App can read/create the DB path.
Acceptable Input Values/Types:
- String env vars for paths and addresses.
- Duration string for MODBUS_POLL_INTERVAL.
- MODBUS_MW_BASE number from 0 to 65535.
Unacceptable Input Values/Types:
- Bad duration string.
- MODBUS_MW_BASE out of range or not a number.
- DB/migration/validation paths that do not exist or cannot be read.
Postconditions:
- Returns a filled Config with an open DB, or returns an error.
Return Values/Types:
- Load: (*Config, error)
- Helper functions return parsed values or errors.
Error/Exception Conditions:
- File create/read errors, DB open/ping errors, or parse errors.
Side Effects:
- May create folders/files for SQLite and opens a DB connection.
Invariants:
- If Load succeeds, DB and Queries are always set.
Known Faults:
- Naming style is not fully consistent in a few places.
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
	DB                   *sql.DB
	Queries              *database.Queries
	Secret               string
	SQLitePath           string
	MigrationsPath       string
	ModbusPollInterval   time.Duration
	ModbusAddress        string
	ModbusMWBase         uint16 // uint since modbus usually expects these
	ValidationFilePath   string
	MLAPIURL             string
	MLHTTPTimeout        time.Duration
	MLBatchSize          int
	MLBatchFlushInterval time.Duration
	MLDropOnOverload     bool
}

// description: Builds and validates service configuration, opens SQLite, and initializes query helpers.
// input: None (reads environment variables with fallback defaults).
// output: Returns a fully initialized Config or an error.
func Load() (*Config, error) {
	// Load in enviroment variables and if not present give it a fallback value
	sqlitePath := getEnv("SQLITE_PATH", "./data/app.db")
	migrationsPath := getEnv("MIGRATIONS_PATH", "sql/schema")
	modbusAddress := getEnv("MODBUS_ADDRESS", "127.0.0.1:1502")
	ValidationFilePath := getEnv("VALIDATION_FILE_PATH", "config/validation_rules.json")
	mlAPIURL := getEnv("ML_API_URL", "")

	// Make sure the env you loaded for sqlite is real and can connect properly
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

	// Make sure a poll interval exists if not give it a fallback and parse the duration of it
	modbusPollInterval, err := getDurationEnv("MODBUS_POLL_INTERVAL", 5*time.Millisecond)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	// Get the offset value and ensure it is proper
	modbusMWBase, err := getUint16Env("MODBUS_MW_BASE", 1024)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	mlHTTPTimeout, err := getDurationEnv("ML_HTTP_TIMEOUT", 500*time.Millisecond)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	mlBatchFlushInterval, err := getDurationEnv("ML_BATCH_FLUSH_INTERVAL", 25*time.Millisecond)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	mlBatchSize, err := getIntEnv("ML_BATCH_SIZE", 32)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	if mlBatchSize <= 0 {
		_ = db.Close()
		return nil, fmt.Errorf("ML_BATCH_SIZE must be > 0")
	}

	mlDropOnOverload, err := getBoolEnv("ML_DROP_ON_OVERLOAD", true)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	// Return a config item
	return &Config{
		DB:                   db,
		Queries:              database.New(db),
		Secret:               os.Getenv("SECRET"),
		SQLitePath:           sqlitePath,
		MigrationsPath:       migrationsPath,
		ModbusPollInterval:   modbusPollInterval,
		ModbusAddress:        modbusAddress,
		ModbusMWBase:         modbusMWBase,
		ValidationFilePath:   ValidationFilePath,
		MLAPIURL:             mlAPIURL,
		MLHTTPTimeout:        mlHTTPTimeout,
		MLBatchSize:          mlBatchSize,
		MLBatchFlushInterval: mlBatchFlushInterval,
		MLDropOnOverload:     mlDropOnOverload,
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

// description: Reads a positive/negative int environment variable with fallback.
// input: key (env var name), fallback (value used when unset).
// output: Returns parsed int or fallback; returns error on parse failure.
func getIntEnv(key string, fallback int) (int, error) {
	if v := os.Getenv(key); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return 0, fmt.Errorf("Parse %s Error: %w", key, err)
		}
		return n, nil
	}
	return fallback, nil
}

// description: Reads a boolean environment variable with fallback.
// input: key (env var name), fallback (value used when unset).
// output: Returns parsed bool or fallback; returns error on invalid values.
func getBoolEnv(key string, fallback bool) (bool, error) {
	if v := os.Getenv(key); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return false, fmt.Errorf("Parse %s Error: %w", key, err)
		}
		return b, nil
	}
	return fallback, nil
}

// description: Ensures the SQLite file and its parent directory exist.
// input: dbPath (target SQLite file path).
// output: Returns nil when file is present/created, or an error on filesystem failures.
func ensureSQLiteFile(dbPath string) error {
	// Load in the directory and make it if it doesn't exist
	dir := filepath.Dir(dbPath)
	if dir != "" {
		// rwx rx rx
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("Mkdir Error %s: %w", dir, err)
		}
	}

	// Double check file exists
	if _, err := os.Stat(dbPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("Stat Error %s: %w", dbPath, err)
	}

	// Open the file if it exists
	f, err := os.OpenFile(dbPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("Error creating SQLite file %s: %w", dbPath, err)
	}
	return f.Close()
}
