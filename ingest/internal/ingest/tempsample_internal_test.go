package ingest

/*
Name: ingest/internal/ingest/tempsample_internal_test.go
Description: Tests TempSample helper methods and the UTC timestamp helper.
Programmer: Barrett Brown
Date Created: 2026-02-28
Dates Revised: 2026-02-28
Revision History:
- 2026-02-28, Barrett Brown: Created helper tests for TempSample behavior.
Preconditions:
- System time is available for timestamp generation.
Acceptable Input Values/Types:
- TempSample values with normal numeric fields.
Unacceptable Input Values/Types:
- No special invalid inputs are required for these helper tests.
Postconditions:
- Confirms payload and timestamp helpers behave as expected.
Return Values/Types:
- Test functions return no value.
Error/Exception Conditions:
- RFC3339Nano parse failures for generated timestamps.
Side Effects:
- Reads current system time.
Invariants:
- nowUTC should always return a UTC formatted timestamp.
Known Faults:
- Does not check exact timestamp precision.
*/

import (
	"testing"
	"time"
)

func TestTempSamplePayloadReturnsValueCopy(t *testing.T) {
	sample := TempSample{Temperature: 72.5}

	payload, ok := sample.Payload().(TempSample)
	if !ok {
		t.Fatalf("Payload() type assertion failed")
	}
	if payload.Temperature != sample.Temperature {
		t.Fatalf("Payload().Temperature = %v, want %v", payload.Temperature, sample.Temperature)
	}
}

func TestNowUTCUsesRFC3339Nano(t *testing.T) {
	got := nowUTC()
	parsed, err := time.Parse(time.RFC3339Nano, got)
	if err != nil {
		t.Fatalf("time.Parse() error = %v", err)
	}
	if parsed.Location() != time.UTC {
		t.Fatalf("nowUTC() location = %v, want UTC", parsed.Location())
	}
}
