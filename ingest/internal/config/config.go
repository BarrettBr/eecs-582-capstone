package config

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/BarrettBr/eecs-582-capstone/internal/database"
	_ "modernc.org/sqlite"
)

type Config struct {
	DB                 *sql.DB
	Queries            *database.Queries
	Secret             string
	SQLitePath         string
	MigrationsPath     string
	ModbusPollInterval time.Duration
    ModbusAdress       string
}

// Creates and returns a config struct using fallbacks if envs not found
func Load() (*Config, error) {
	sqlitePath := getEnv("SQLITE_PATH", "./data/app.db")
	migrationsPath := getEnv("MIGRATIONS_PATH", "sql/schema")
    modbusAddress := getEnv("MODBUS_ADDRESS", "localhost:502")

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

	modbusPollInterval, err := getDurationEnv("MODBUS_POLL_INTERVAL", 5*time.Second)
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
        ModbusAdress:       modbusAddress,
	}, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

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
