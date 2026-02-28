package ingest

/*
Name: ingest/internal/ingest/record_event.go
Description: Defines generic RecordEvent interface for use with down the line valve and nuclear data support
Programmer: Barrett Brown
Date Created: 2026-02-01
Dates Revised: 2026-02-28
Revision History:
- 2026-02-28, Barrett Brown: Created File and filled out
Preconditions:
- None
Acceptable Input Values/Types:
- None it is exported
Unacceptable Input Values/Types:
- None it is exported
Postconditions:
- Gives an exported RecordEvent for use with tempsample methods and functions
Return Values/Types:
- Exported interface
Error/Exception Conditions:
- No errors
Side Effects:
- None
Invariants:
- Nothing
Known Faults:
- Nothing
*/

// RecordEvent describes the normalized event contract used across validation and fanout.
type RecordEvent interface {
	ToRecord() map[string]any
	EventType() string
	Payload() any
	AnomalyLabels() []string
}
