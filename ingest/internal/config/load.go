package config

import (
	"time"

	"github.com/BarrettBr/eecs-582-capstone/internal/database"
)

// Load builds the runtime configuration, opens SQLite, and validates the source catalog.
func Load() (*Config, error) {
	cfg, err := loadSettings()
	if err != nil {
		return nil, err
	}

	db, err := openSQLite(cfg.Database.SQLite)
	if err != nil {
		return nil, err
	}

	sources, err := LoadSourceCatalog(cfg.Ingest.SourceConfigPath)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	cfg.DB = db
	cfg.Queries = database.New(db)
	cfg.Sources = sources

	return cfg, nil
}

func loadSettings() (*Config, error) {
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

	mlDropOnOverload, err := getBoolEnv("ML_DROP_ON_OVERLOAD", true)
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
			MLFanout: MLFanoutConfig{
				APIURL:             getEnv("ML_API_URL", ""),
				HTTPTimeout:        mlHTTPTimeout,
				BatchSize:          mlBatchSize,
				BatchFlushInterval: mlBatchFlushInterval,
				DropOnOverload:     mlDropOnOverload,
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
			BatchSize:          wsBatchSize,
			BatchFlushInterval: wsBatchFlushInterval,
		},
	}, nil
}
