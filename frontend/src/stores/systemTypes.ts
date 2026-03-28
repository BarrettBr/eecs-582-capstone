// Name: stores/systemTypes.ts
// Description: Shared type definitions for the dashboard system store and helpers.
// Programmers: Adam Berry, Barrett Brown
// Creation Date: 3/1
// Revision Dates: 3/15 Barrett Brown
// Revision Notes: 3/14 Barrett Brown split the larger system store into focused modules.
// Revision Notes: 3/15 Barrett Brown added aggregated service rate payload typing for sidebar labels.
// Preconditions: None
// Postconditions: Not Relevant
// Error Types: Not Relevant
// Invariants: Shared dashboard payload shapes stay aligned with the backend stream and API payloads.
// Known Faults: None

export interface SystemStatus {
	api: string;
	ingestion: string;
	ml: string;
}

export type ServiceEventType = "temperature" | "valve";

export interface TempEventData {
	event_type?: "temperature";
	id?: number;
	service_name?: string;
	timestamp: string;
	timestamp_ms?: number;
	display_time?: string;
	display_timestamp?: string;
	sensor_type?: string;
	sensor_number?: number;
	fan_on?: boolean;
	temperature?: number;
	heater_power?: number;
	anomalies?: string[];
}

export interface ValveEventData {
	event_type?: "valve";
	id?: number;
	service_name?: string;
	timestamp: string;
	timestamp_ms?: number;
	display_time?: string;
	display_timestamp?: string;
	sensor_type?: string;
	valve_number?: number;
	is_open?: boolean;
	flow_rate?: number;
	anomalies?: string[];
}

export interface MLAnomalyPayload {
	schema: string;
	service_name?: string;
	event_type: string;
	generated_at: string;
	generated_at_ms?: number;
	display_time?: string;
	has_anomaly: boolean;
	severity?: "info" | "low" | "medium" | "high" | string;
	labels: string[];
	confidence?: number;
	probable_cause?: string;
	recommended_action?: string;
	score?: number;
	raw_response: unknown;
}

export interface IngestionMetricsResponse {
	temp_records?: number;
	valve_records?: number;
	total_records?: number;
}

export interface ServiceCatalogEntry {
	name: string;
	alias_name?: string;
	mode: string;
	event_type: ServiceEventType;
	admitted_eps_5s?: number;
}

export interface IngestionStatusResponse {
	last_reload_at?: string;
	services?: ServiceCatalogEntry[];
}

export interface ServiceCatalogPayload {
	revision?: string;
	services?: ServiceCatalogEntry[];
}

export interface ServiceRateEntry {
	name: string;
	admitted_eps_5s?: number;
}

export interface ServiceRatesPayload {
	interval_seconds?: number;
	services?: ServiceRateEntry[];
}

export interface SubscriptionAckPayload {
	accepted_services?: string[];
	rejected_services?: string[];
	current_services?: string[];
}

export interface SubscriptionsPrunedPayload {
	removed_services?: string[];
	current_services?: string[];
	reason?: string;
}
