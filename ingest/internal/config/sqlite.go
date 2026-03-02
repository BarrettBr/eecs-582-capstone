package config

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

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
