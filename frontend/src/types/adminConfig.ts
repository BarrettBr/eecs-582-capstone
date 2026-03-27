export type ServiceEventType = "temperature" | "valve";
export type ServiceMode = "simulator" | "modbus";

export interface AdminBackpressurePolicy {
	high_interval_multiplier: number;
	critical_interval_multiplier: number;
}

export interface AdminFlowControlConfig {
	reserved_units: number;
	adaptive_enabled: boolean;
	supports_backpressure: boolean;
	overload_policy: string;
	sampling_every_n: number;
	backpressure: AdminBackpressurePolicy;
}

export interface AdminModbusSourceSettings {
	address: string;
	mw_base: number;
	register_count: number;
	slave_id: number;
	interval?: string;
}

export interface AdminSimulatorSourceSettings {
	interval: string;
	seed: number;
	profile: string;
}

export interface AdminSourceDefinition {
	name: string;
	alias_name?: string;
	event_type: ServiceEventType;
	mode: ServiceMode;
	enabled: boolean;
	validation_file: string;
	sql_target: string;
	ml_enabled: boolean;
	flow_control: AdminFlowControlConfig;
	modbus?: AdminModbusSourceSettings;
	simulator?: AdminSimulatorSourceSettings;
}

export interface AdminPressureThresholds {
	elevated_enter: number;
	elevated_exit: number;
	high_enter: number;
	high_exit: number;
	critical_enter: number;
	critical_exit: number;
}

export interface AdminBufferingConfig {
	shared_units: number;
	sampling_shared_threshold: number;
	pressure: AdminPressureThresholds;
}

export interface AdminSourceRuntimeConfig {
	buffering: AdminBufferingConfig;
}

export interface AdminSourceProfile {
	enabled_sources: string[];
}

export interface AdminSourceCatalog {
	runtime: AdminSourceRuntimeConfig;
	profiles?: Record<string, AdminSourceProfile>;
	sources: AdminSourceDefinition[];
}

export interface AdminBounds {
	min?: number;
	max?: number;
}

export interface AdminAnomalyRule {
	field: string;
	op: string;
	value: number;
	label: string;
}

export interface AdminValidationSpec {
	required_fields: string[];
	bounds: Record<string, AdminBounds>;
	anomaly_rules: AdminAnomalyRule[];
}

export interface AdminConfigDocument {
	source_config_path: string;
	source_profile?: string;
	catalog: AdminSourceCatalog;
	validation_specs: Record<string, AdminValidationSpec>;
}
