package config

/*
Name: ingest/internal/config/types.go
Description: Defines the main runtime config structs shared across ingest startup and wiring.
Programmer: Barrett Brown
Date Created: 2026-03-07
Dates Revised: 2026-03-14
Revision History:
- 2026-03-07, Barrett Brown: Added standardized prologue documentation block.
- 2026-03-13, Barrett Brown: Added clearer config type descriptions.
- 2026-03-14, Barrett Brown: Added source config profile selection so one catalog can support simulator and modbus presets.
Preconditions:
- Config values are populated by the config load path before use.
Acceptable Input Values/Types:
- Runtime settings for database, ingest, stream, and source catalog wiring.
Unacceptable Input Values/Types:
- Nil required dependencies after load completion.
Postconditions:
- Provides shared config types for the ingest service.
Return Values/Types:
- Struct definitions only.
Error/Exception Conditions:
- None in this file directly.
Side Effects:
- None.
Invariants:
- Config groups stay separated by subsystem.
Known Faults:
- Runtime config still mixes both settings and initialized dependencies in one top level struct.
*/

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
	SourceProfile    string
	BufferChunkSize  int
	MLFanout         MLFanoutConfig
	SQLFanout        SQLFanoutConfig
}

// MLFanoutConfig controls outbound ML batching behavior.
type MLFanoutConfig struct {
	APIURL             string
	HTTPTimeout        time.Duration
	BatchSize          int
	BatchFlushInterval time.Duration
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
