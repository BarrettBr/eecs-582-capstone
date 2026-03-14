package api

/*
Name: ingest/internal/api/history_test.go
Description: Tests history query selection and helper behavior directly.
Programmer: Barrett Brown
Date Created: 2026-03-14
Dates Revised: 2026-03-14
Revision History:
- 2026-03-14, Barrett Brown: Added direct history helper coverage tests.
Preconditions:
- History helper functions are available in the api package.
Acceptable Input Values/Types:
- Valid endpoint, kind, and window combinations plus anomaly label strings.
Unacceptable Input Values/Types:
- Unsupported kinds and windows.
Postconditions:
- Confirms selection helpers and anomaly splitting behave as expected.
Return Values/Types:
- Test functions return no value.
Error/Exception Conditions:
- Unexpected selection data or missing errors fail the tests.
Side Effects:
- None.
Invariants:
- buildQuerySelection should derive consistent table and range values.
Known Faults:
- Does not cover database query execution paths directly.
*/

import "testing"

func TestBuildQuerySelectionForTempHour(t *testing.T) {
	selection, err := buildQuerySelection("history", "temp", "hour")
	if err != nil {
		t.Fatalf("buildQuerySelection() error = %v", err)
	}
	if selection.Table != "temp_samples" {
		t.Fatalf("Table = %q, want %q", selection.Table, "temp_samples")
	}
	if selection.GroupBy != "minute" || selection.Range != "1h" || selection.Seconds != 3600 {
		t.Fatalf("selection = %+v, want minute / 1h / 3600", selection)
	}
}

func TestBuildQuerySelectionRejectsBadInput(t *testing.T) {
	if _, err := buildQuerySelection("history", "bad", "hour"); err == nil {
		t.Fatalf("buildQuerySelection() bad kind error = nil, want non-nil")
	}
	if _, err := buildQuerySelection("report", "temp", "month"); err == nil {
		t.Fatalf("buildQuerySelection() bad window error = nil, want non-nil")
	}
}

func TestSplitAnomalies(t *testing.T) {
	if got := splitAnomalies(""); len(got) != 0 {
		t.Fatalf("splitAnomalies(empty) = %v, want []", got)
	}
	got := splitAnomalies("a,b,c")
	if len(got) != 3 || got[0] != "a" || got[2] != "c" {
		t.Fatalf("splitAnomalies() = %v, want [a b c]", got)
	}
}
