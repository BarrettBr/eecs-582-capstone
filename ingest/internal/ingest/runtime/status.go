package runtime

/*
Name: ingest/internal/ingest/runtime/status.go
Description: Defines the status payloads exposed by the registrar and BufferManager status endpoint.
Programmer: Barrett Brown
Date Created: 2026-03-07
Dates Revised: 2026-03-15
Revision History:
- 2026-03-07, Barrett Brown: Added standardized prologue documentation block.
- 2026-03-13, Barrett Brown: Added clearer status type documentation.
- 2026-03-15, Barrett Brown: Added admitted event totals for low-overhead service rate reporting.
Preconditions:
- Runtime status fields are populated by registrar and BufferManager snapshots.
Acceptable Input Values/Types:
- Lifecycle, buffering, and pressure data from the ingest runtime.
Unacceptable Input Values/Types:
- None in this file directly.
Postconditions:
- Provides stable JSON status structs for the API layer.
Return Values/Types:
- Struct definitions only.
Error/Exception Conditions:
- None.
Side Effects:
- None.
Invariants:
- Status types stay aligned with the ingest status endpoint payload.
Known Faults:
- Snapshot structs still mix both service and system level status fields in one package.
*/

import "time"

type ServiceStatus struct {
	Name                 string          `json:"name"`
	AliasName            string          `json:"alias_name,omitempty"`
	InstanceName         string          `json:"instance_name,omitempty"`
	MachineID            string          `json:"machine_id,omitempty"`
	MachineName          string          `json:"machine_name,omitempty"`
	Mode                 string          `json:"mode"`
	EventType            string          `json:"event_type"`
	LifecycleState       string          `json:"lifecycle_state"`
	Connected            bool            `json:"connected"`
	MachineCount         int             `json:"machine_count"`
	RecentAlertCount     int             `json:"recent_alert_count"`
	LastAnomalyAt        string          `json:"last_anomaly_at,omitempty"`
	ReservedUnits        int             `json:"reserved_units"`
	BufferedUnits        int             `json:"buffered_units"`
	QueueDepth           int             `json:"queue_depth"`
	AdmittedEvents       uint64          `json:"admitted_events"`
	AdaptiveEnabled      bool            `json:"adaptive_enabled"`
	SupportsBackpressure bool            `json:"supports_backpressure"`
	RateReduced          bool            `json:"rate_reduced"`
	Sampling             bool            `json:"sampling"`
	CurrentInterval      time.Duration   `json:"current_interval"`
	DroppedEvents        uint64          `json:"dropped_events"`
	EvictedEvents        uint64          `json:"evicted_events"`
	SampledEvents        uint64          `json:"sampled_events"`
	LastError            string          `json:"last_error,omitempty"`
	Machines             []MachineStatus `json:"machines,omitempty"`
}

type MachineStatus struct {
	ID             string `json:"id"`
	Name           string `json:"name,omitempty"`
	ServiceName    string `json:"service_name"`
	InstanceName   string `json:"instance_name"`
	LifecycleState string `json:"lifecycle_state"`
	Connected      bool   `json:"connected"`
	RecentAlerts   int    `json:"recent_alerts"`
	AdmittedEvents uint64 `json:"admitted_events"`
	LastAnomalyAt  string `json:"last_anomaly_at,omitempty"`
}

type SystemStatusSnapshot struct {
	PressureState       PressureState   `json:"pressure_state"`
	TotalBufferedUnits  int             `json:"total_buffered_units"`
	TotalCapacityUnits  int             `json:"total_capacity_units"`
	SharedUnitsUsed     int             `json:"shared_units_used"`
	SharedUnitsCapacity int             `json:"shared_units_capacity"`
	LastReloadAt        string          `json:"last_reload_at,omitempty"`
	LastReloadError     string          `json:"last_reload_error,omitempty"`
	Services            []ServiceStatus `json:"services"`
}
