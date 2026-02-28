package config

/*
Name: ingest/internal/config/config_test.go
Description: Tests SQLite behavior for valid and invalid settings.
Programmer: Barrett Brown
Date Created: 2026-02-28
Dates Revised: 2026-02-28
Revision History:
- 2026-02-28, Barrett Brown: Created test file for SQLite tuning helper coverage.
Preconditions:
- SQLite test database can be opened in memory.
Acceptable Input Values/Types:
- Valid and invalid synchronous mode strings.
- Boolean flag for WAL mode selection.
Unacceptable Input Values/Types:
- Nil database handles.
Postconditions:
- Confirms valid PRAGMA settings succeed and invalid ones fail.
Return Values/Types:
- Test helpers return *sql.DB.
- Test functions return no value.
Error/Exception Conditions:
- SQLite open failures.
- Unexpected missing errors for invalid synchronous values.
Side Effects:
- Opens in memory SQLite connections for test use.
Invariants:
- Each test uses an isolated in memory database.
Known Faults:
- Does not verify the exact SQLite PRAGMA state after success.
*/

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestApplySQLiteTuningRejectsInvalidSynchronousMode(t *testing.T) {
	db := openTestSQLite(t)

	if err := applySQLiteTuning(db, true, "bad"); err == nil {
		t.Fatalf("applySQLiteTuning() error = nil, want non-nil")
	}
}

func TestApplySQLiteTuningAcceptsValidSynchronousMode(t *testing.T) {
	db := openTestSQLite(t)

	if err := applySQLiteTuning(db, false, "normal"); err != nil {
		t.Fatalf("applySQLiteTuning() error = %v", err)
	}
}

func openTestSQLite(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	return db
}
