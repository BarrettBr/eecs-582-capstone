package config

/*
Name: ingest/internal/config/sqlite.go
Description: Opens the SQLite database file and applies the requested runtime tuning pragmas.
Programmer: Barrett Brown
Date Created: 2026-03-07
Dates Revised: 2026-03-13
Revision History:
- 2026-03-07, Barrett Brown: Added standardized prologue documentation block.
- 2026-03-13, Barrett Brown: Added clearer SQLite setup comments.
Preconditions:
- SQLite path and tuning settings are available from runtime config.
Acceptable Input Values/Types:
- Valid SQLite file paths and supported synchronous modes.
Unacceptable Input Values/Types:
- Unsupported synchronous mode strings or inaccessible file paths.
Postconditions:
- Returns a reachable SQLite database handle with tuning applied.
Return Values/Types:
- openSQLite: (*sql.DB, error)
- Helper functions return nil or an error.
Error/Exception Conditions:
- File creation failures, open failures, ping failures, and pragma failures.
Side Effects:
- Creates the database directory and file when needed.
- Opens a live SQLite connection.
Invariants:
- SQLite tuning runs only after a successful open and ping.
Known Faults:
- Tuning still happens synchronously during startup.
*/

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

// description: Opens the configured SQLite database and applies startup tuning.
// input: SQLiteConfig with file path and tuning values.
// output: Returns an open *sql.DB or an error.
func openSQLite(cfg SQLiteConfig) (*sql.DB, error) {
	if err := ensureSQLiteFile(cfg.Path); err != nil {
		return nil, fmt.Errorf("ensure sqlite db file: %w", err)
	}

	db, err := sql.Open("sqlite", cfg.Path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db: %w", err)
	}

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite db: %w", err)
	}

	if err := applySQLiteTuning(db, cfg.EnableWAL, cfg.Synchronous); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}

// description: Makes sure the SQLite file and parent directory exist.
// input: dbPath for the SQLite database file.
// output: Returns nil when the file exists and is ready to open.
func ensureSQLiteFile(dbPath string) error {
	dir := filepath.Dir(dbPath)
	if dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", dir, err)
		}
	}

	if _, err := os.Stat(dbPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat %s: %w", dbPath, err)
	}

	f, err := os.OpenFile(dbPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("create sqlite file %s: %w", dbPath, err)
	}
	return f.Close()
}

// description: Applies journal mode and synchronous settings to SQLite.
// input: open database handle, WAL flag, and synchronous mode string.
// output: Returns nil when the requested pragmas are applied.
func applySQLiteTuning(db *sql.DB, enableWAL bool, synchronous string) error {
	mode := strings.ToUpper(strings.TrimSpace(synchronous))
	switch mode {
	case "OFF", "NORMAL", "FULL", "EXTRA":
	default:
		return fmt.Errorf("SQLITE_SYNCHRONOUS must be one of OFF, NORMAL, FULL, EXTRA")
	}

	if enableWAL {
		if _, err := db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
			return fmt.Errorf("enable sqlite WAL mode: %w", err)
		}
	}
	if _, err := db.Exec(fmt.Sprintf("PRAGMA synchronous=%s;", mode)); err != nil {
		return fmt.Errorf("set sqlite synchronous mode: %w", err)
	}

	return nil
}
