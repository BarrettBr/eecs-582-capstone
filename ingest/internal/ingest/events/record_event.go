package events

/*
Name: ingest/internal/ingest/events/record_event.go
Description: Defines the shared normalized event contract used by validation and downstream fanout code.
Programmer: Barrett Brown
Date Created: 2026-02-01
Dates Revised: 2026-03-14
Revision History:
- 2026-02-28, Barrett Brown: Created the generic record event contract for shared validation and fanout use.
- 2026-03-14, Barrett Brown: Removed ToRecord from the interface and added direct validation field access.
- 2026-03-14, Barrett Brown: Moved the record event contract into the events package during the ingest file structure refactor.
Preconditions:
- None
Acceptable Input Values/Types:
- Any concrete sample type that can expose routing, payload, anomaly labels, and validation field values.
Unacceptable Input Values/Types:
- None in this file directly.
Postconditions:
- Provides the shared RecordEvent interface for normalized ingest events.
Return Values/Types:
- Exported interface.
Error/Exception Conditions:
- None.
Side Effects:
- None.
Invariants:
- Implementations must provide stable field names for validation lookups.
Known Faults:
- Validation field lookup remains string keyed at the interface boundary.
*/

// RecordEvent describes the normalized event contract used across validation and fanout.
type RecordEvent interface {
	EventType() string
	Payload() any
	AnomalyLabels() []string
	ValidationValue(field string) (any, bool)
}
