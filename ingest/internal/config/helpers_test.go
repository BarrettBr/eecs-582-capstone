package config

/*
Name: ingest/internal/config/helpers_test.go
Description: Tests config helper parsing functions and SQLite file creation behavior.
Programmer: Barrett Brown
Date Created: 2026-02-28
Dates Revised: 2026-02-28
Revision History:
- 2026-02-28, Barrett Brown: Created helper coverage tests for config parsing.
Preconditions:
- Environment variables can be set during test execution.
- Temporary directories are available for file tests.
Acceptable Input Values/Types:
- Valid and invalid env strings for parsing helpers.
- Writable temp directory paths.
Unacceptable Input Values/Types:
- Nil filesystem paths are not applicable in these tests.
Postconditions:
- Confirms helper functions return expected parsed values and errors.
Return Values/Types:
- Test functions return no value.
Error/Exception Conditions:
- Unexpected parse success for invalid values.
- File creation failures in temp directories.
Side Effects:
- Creates temp files and directories.
- Temporarily sets environment variables.
Invariants:
- Tests isolate env usage with test scoped variables.
Known Faults:
- Does not cover every possible invalid env string.
*/

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGetEnvReturnsEnvValueOrFallback(t *testing.T) {
	key := "CONFIG_TEST_GET_ENV"
	t.Setenv(key, "present")
	if got := getEnv(key, "fallback"); got != "present" {
		t.Fatalf("getEnv() = %q, want %q", got, "present")
	}

	t.Setenv(key, "")
	if got := getEnv(key, "fallback"); got != "fallback" {
		t.Fatalf("getEnv() fallback = %q, want %q", got, "fallback")
	}
}

func TestGetDurationEnv(t *testing.T) {
	key := "CONFIG_TEST_DURATION"
	t.Setenv(key, "15ms")

	got, err := getDurationEnv(key, time.Second)
	if err != nil {
		t.Fatalf("getDurationEnv() error = %v", err)
	}
	if got != 15*time.Millisecond {
		t.Fatalf("getDurationEnv() = %v, want %v", got, 15*time.Millisecond)
	}

	t.Setenv(key, "bad")
	if _, err := getDurationEnv(key, time.Second); err == nil {
		t.Fatalf("getDurationEnv() error = nil, want non-nil")
	}
}

func TestGetIntEnvAndGetBoolEnv(t *testing.T) {
	intKey := "CONFIG_TEST_INT"
	boolKey := "CONFIG_TEST_BOOL"

	t.Setenv(intKey, "42")
	gotInt, err := getIntEnv(intKey, 1)
	if err != nil {
		t.Fatalf("getIntEnv() error = %v", err)
	}
	if gotInt != 42 {
		t.Fatalf("getIntEnv() = %d, want 42", gotInt)
	}

	t.Setenv(boolKey, "true")
	gotBool, err := getBoolEnv(boolKey, false)
	if err != nil {
		t.Fatalf("getBoolEnv() error = %v", err)
	}
	if !gotBool {
		t.Fatalf("getBoolEnv() = false, want true")
	}

	t.Setenv(boolKey, "notabool")
	if _, err := getBoolEnv(boolKey, false); err == nil {
		t.Fatalf("getBoolEnv() error = nil, want non-nil")
	}
}

func TestEnsureSQLiteFileCreatesFile(t *testing.T) {
	baseDir := t.TempDir()
	dbPath := filepath.Join(baseDir, "nested", "app.db")

	if err := ensureSQLiteFile(dbPath); err != nil {
		t.Fatalf("ensureSQLiteFile() error = %v", err)
	}

	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("os.Stat() error = %v", err)
	}

	if err := ensureSQLiteFile(dbPath); err != nil {
		t.Fatalf("ensureSQLiteFile() second call error = %v", err)
	}
}
