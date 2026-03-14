package config

/*
Name: ingest/internal/config/load.go
Description: Builds the full ingest runtime config, opens SQLite, and loads the source catalog.
Programmer: Barrett Brown
Date Created: 2026-03-07
Dates Revised: 2026-03-14
Revision History:
- 2026-03-07, Barrett Brown: Added standardized prologue documentation block.
- 2026-03-13, Barrett Brown: Added clearer config build comments and chunk size validation notes.
- 2026-03-14, Barrett Brown: Added source profile loading so one source catalog file can support multiple local run presets.
Preconditions:
- Environment variables and source config path are available.
- SQLite settings and source catalog values are valid.
Acceptable Input Values/Types:
- Runtime env values and a readable source catalog file.
Unacceptable Input Values/Types:
- Invalid durations, invalid chunk sizes, or unreadable config files.
Postconditions:
- Returns a fully initialized Config with database and query dependencies.
Return Values/Types:
- Load: (*Config, error)
- Helper loaders return typed config values or errors.
Error/Exception Conditions:
- Config parse failures, SQLite open failures, and source catalog errors.
Side Effects:
- Opens the database and reads the source catalog file.
Invariants:
- Config is not returned until database and source catalog initialization both succeed.
Known Faults:
- Load still performs startup work synchronously on the main path.
*/

import (
	"fmt"
	"strings"
	"time"

	"github.com/BarrettBr/eecs-582-capstone/internal/database"
)

// description: Builds the full runtime config and initializes database backed dependencies.
// input: None, reads env settings and source config from disk.
// output: Returns a ready Config or an error if startup config is invalid.
func Load() (*Config, error) {
	cfg, err := loadSettings()
	if err != nil {
		return nil, err
	}

	db, err := openSQLite(cfg.Database.SQLite)
	if err != nil {
		return nil, err
	}

	sources, err := LoadSourceCatalogWithProfile(cfg.Ingest.SourceConfigPath, cfg.Ingest.SourceProfile)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	cfg.DB = db
	cfg.Queries = database.New(db)
	cfg.Sources = sources

	return cfg, nil
}

// description: Reads process env values and assembles the in memory Config settings.
// input: None, reads process environment variables and defaults.
// output: Returns a Config with runtime settings but no opened database handles.
func loadSettings() (*Config, error) {
	bufferChunkSize, err := loadBufferChunkSize()
	if err != nil {
		return nil, err
	}

	sqliteEnableWAL, err := getBoolEnv("SQLITE_ENABLE_WAL", true)
	if err != nil {
		return nil, err
	}

	pollInterval, err := getDurationEnv("MODBUS_POLL_INTERVAL", 5*time.Millisecond)
	if err != nil {
		return nil, err
	}

	mlHTTPTimeout, err := getDurationEnv("ML_HTTP_TIMEOUT", 500*time.Millisecond)
	if err != nil {
		return nil, err
	}

	mlBatchSize, err := getPositiveIntEnv("ML_BATCH_SIZE", 32)
	if err != nil {
		return nil, err
	}

	mlBatchFlushInterval, err := getPositiveDurationEnv("ML_BATCH_FLUSH_INTERVAL", 25*time.Millisecond)
	if err != nil {
		return nil, err
	}

	wsBatchSize, err := getPositiveIntEnv("WS_BATCH_SIZE", 16)
	if err != nil {
		return nil, err
	}

	wsBatchFlushInterval, err := getPositiveDurationEnv("WS_BATCH_FLUSH_INTERVAL", 50*time.Millisecond)
	if err != nil {
		return nil, err
	}

	sqlBatchSize, err := getPositiveIntEnv("SQL_BATCH_SIZE", 256)
	if err != nil {
		return nil, err
	}

	sqlBatchFlushInterval, err := getPositiveDurationEnv("SQL_BATCH_FLUSH_INTERVAL", 25*time.Millisecond)
	if err != nil {
		return nil, err
	}

	sqlNormalSampleRate, err := getPositiveIntEnv("SQL_NORMAL_SAMPLE_RATE", 1)
	if err != nil {
		return nil, err
	}

	apiBasePath := strings.TrimSpace(getEnv("API_BASE_PATH", "/api/v1"))
	if apiBasePath == "" {
		apiBasePath = "/api/v1"
	}
	if !strings.HasPrefix(apiBasePath, "/") {
		apiBasePath = "/" + apiBasePath
	}
	apiBasePath = strings.TrimRight(apiBasePath, "/")
	if apiBasePath == "" {
		apiBasePath = "/"
	}

	return &Config{
		Database: DatabaseConfig{
			MigrationsPath: getEnv("MIGRATIONS_PATH", "sql/schema"),
			SQLite: SQLiteConfig{
				Path:        getEnv("SQLITE_PATH", "./data/app.db"),
				EnableWAL:   sqliteEnableWAL,
				Synchronous: getEnv("SQLITE_SYNCHRONOUS", "NORMAL"),
			},
		},
		Ingest: IngestConfig{
			PollInterval:     pollInterval,
			SourceConfigPath: getEnv("SOURCE_CONFIG_PATH", "config/sources.json"),
			SourceProfile:    strings.TrimSpace(getEnv("SOURCE_CONFIG_PROFILE", "")),
			BufferChunkSize:  bufferChunkSize,
			MLFanout: MLFanoutConfig{
				APIURL:             getEnv("ML_API_URL", ""),
				HTTPTimeout:        mlHTTPTimeout,
				BatchSize:          mlBatchSize,
				BatchFlushInterval: mlBatchFlushInterval,
			},
			SQLFanout: SQLFanoutConfig{
				BatchSize:          sqlBatchSize,
				BatchFlushInterval: sqlBatchFlushInterval,
				NormalSampleRate:   sqlNormalSampleRate,
			},
		},
		Stream: StreamConfig{
			Address:            getEnv("WS_ADDRESS", "127.0.0.1:8080"),
			Path:               getEnv("WS_PATH", "/ws"),
			APIBasePath:        apiBasePath,
			BatchSize:          wsBatchSize,
			BatchFlushInterval: wsBatchFlushInterval,
		},
	}, nil
}

// description: Validates the optional dev only buffer chunk size env override.
// input: BUFFER_CHUNK_SIZE env var with one allowed integer value.
// output: Returns a supported chunk size or an error.
func loadBufferChunkSize() (int, error) {
	value, err := getPositiveIntEnv("BUFFER_CHUNK_SIZE", 32)
	if err != nil {
		return 0, err
	}

	switch value {
	case 16, 32, 64, 128:
		return value, nil
	default:
		return 0, fmt.Errorf("BUFFER_CHUNK_SIZE must be one of 16, 32, 64, 128")
	}
}
