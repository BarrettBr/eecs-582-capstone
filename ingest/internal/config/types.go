package config

import (
	"database/sql"
	"time"

	"github.com/BarrettBr/eecs-582-capstone/internal/database"
)

// Config contains fully initialized runtime dependencies and validated settings.
type Config struct {
	DB       *sql.DB
	Queries  *database.Queries
	Database DatabaseConfig
	Ingest   IngestConfig
	Stream   StreamConfig
	Sources  *SourceCatalog
}

// DatabaseConfig groups database-specific runtime settings.
type DatabaseConfig struct {
	MigrationsPath string
	SQLite         SQLiteConfig
}

// SQLiteConfig controls SQLite initialization and tuning.
type SQLiteConfig struct {
	Path        string
	EnableWAL   bool
	Synchronous string
}

// IngestConfig groups ingest loop and fanout settings.
type IngestConfig struct {
	PollInterval     time.Duration
	SourceConfigPath string
	MLFanout         MLFanoutConfig
	SQLFanout        SQLFanoutConfig
}

// MLFanoutConfig controls outbound ML batching behavior.
type MLFanoutConfig struct {
	APIURL             string
	HTTPTimeout        time.Duration
	BatchSize          int
	BatchFlushInterval time.Duration
	DropOnOverload     bool
}

// SQLFanoutConfig controls batched database writes.
type SQLFanoutConfig struct {
	BatchSize          int
	BatchFlushInterval time.Duration
	NormalSampleRate   int
}

// StreamConfig controls the websocket server.
type StreamConfig struct {
	Address            string
	Path               string
	APIBasePath        string
	BatchSize          int
	BatchFlushInterval time.Duration
}
