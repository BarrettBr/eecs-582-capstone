<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import Button from "primevue/button";
import { useSystemStore } from "@/stores/system";
import { getDefaultIngestAPIBaseURL } from "@/utils/wsHelper";
import type {
	AdminAnomalyRule,
	AdminBounds,
	AdminConfigDocument,
	AdminFlowControlConfig,
	AdminModbusSourceSettings,
	AdminSimulatorSourceSettings,
	AdminSourceDefinition,
	AdminValidationSpec,
	ServiceEventType,
	ServiceMode,
} from "@/types/adminConfig";

type AdminTab = "overview" | "services" | "rules" | "runtime" | "profiles";

const systemStore = useSystemStore();
const adminConfigURL = `${getDefaultIngestAPIBaseURL()}/admin/config`;

const loading = ref(true);
const saving = ref(false);
const errorMessage = ref("");
const successMessage = ref("");
const baselineSignature = ref("");
const draft = ref<AdminConfigDocument | null>(null);

const activeTab = ref<AdminTab>("overview");
const selectedServiceName = ref("");
const selectedProfileName = ref("");
const selectedRulePath = ref("");
const adminTabs: Array<{ id: AdminTab; label: string; helper: string }> = [
	{
		id: "overview",
		label: "Overview",
		helper: "Status and quick edits",
	},
	{
		id: "services",
		label: "Services",
		helper: "Source setup",
	},
	{
		id: "rules",
		label: "Rules",
		helper: "Validation logic",
	},
	{
		id: "runtime",
		label: "Runtime",
		helper: "Buffering and pressure",
	},
	{
		id: "profiles",
		label: "Profiles",
		helper: "Assignments",
	},
];

const services = computed(() => draft.value?.catalog.sources ?? []);
const serviceCount = computed(() => services.value.length);
const rulePaths = computed(() =>
	Object.keys(draft.value?.validation_specs ?? {}).sort((left, right) =>
		left.localeCompare(right),
	),
);
const ruleCount = computed(() => rulePaths.value.length);
const profileCount = computed(() => profileEntries.value.length);
const profileEntries = computed(() =>
	Object.entries(draft.value?.catalog.profiles ?? {}).sort(([left], [right]) =>
		left.localeCompare(right),
	),
);
const serviceOptions = computed(() =>
	services.value
		.slice()
		.sort((left, right) =>
			displayServiceName(left).localeCompare(displayServiceName(right)),
		),
);
const ruleOptions = computed(() => rulePaths.value.slice());
const profileOptions = computed(() =>
	profileEntries.value.map(([profileName]) => profileName),
);
const selectedProfileEntry = computed(() => {
	const match = profileEntries.value.find(
		([profileName]) => profileName === selectedProfileName.value,
	);
	return match ?? null;
});
const summaryCards = computed(() => {
	if (!draft.value) {
		return [] as Array<{ label: string; value: string; detail: string }>;
	}

	const totalProfileAssignments = profileEntries.value.reduce(
		(total, [, profile]) => total + profile.enabled_sources.length,
		0,
	);
	const assignedRuleReferences = services.value.filter(
		(service) => service.validation_file.trim() !== "",
	).length;

	return [
		{
			label: "Shared Units",
			value: formatNumericValue(draft.value.catalog.runtime.buffering.shared_units),
			detail: `Sampling at ${formatNumericValue(
				draft.value.catalog.runtime.buffering.sampling_shared_threshold,
			)}`,
		},
		{
			label: "Profiles",
			value: formatNumericValue(profileEntries.value.length),
			detail: `${formatNumericValue(totalProfileAssignments)} service links`,
		},
		{
			label: "Rule Files",
			value: formatNumericValue(ruleCount.value),
			detail: `${formatNumericValue(assignedRuleReferences)} assignments`,
		},
	];
});
const runtimeBufferFields = computed(() => {
	const buffering = draft.value?.catalog.runtime.buffering;
	if (!buffering) {
		return [] as Array<{ key: string; label: string; value: number }>;
	}

	return Object.entries(buffering)
		.filter(([key, value]) => key !== "pressure" && typeof value === "number")
		.map(([key, value]) => ({
			key,
			label: formatConfigLabel(key),
			value,
		}));
});
const runtimePressureSections = computed(() => {
	const pressure = draft.value?.catalog.runtime.buffering.pressure;
	if (!pressure) {
		return [] as Array<{
			id: string;
			label: string;
			fields: Array<{ key: string; label: string; value: number }>;
		}>;
	}

	const grouped = new Map<
		string,
		Array<{ key: string; label: string; value: number }>
	>();
	for (const [key, value] of Object.entries(pressure)) {
		if (typeof value !== "number") {
			continue;
		}
		const parts = key.split("_");
		const groupKey = parts[0] ?? key;
		const suffix = parts.slice(1).join("_");
		if (!grouped.has(groupKey)) {
			grouped.set(groupKey, []);
		}
		grouped.get(groupKey)?.push({
			key,
			label: formatConfigLabel(suffix),
			value,
		});
	}

	return Array.from(grouped.entries())
		.sort(([left], [right]) => left.localeCompare(right))
		.map(([groupKey, fields]) => ({
			id: groupKey,
			label: formatConfigLabel(groupKey),
			fields: fields.sort((left, right) => left.label.localeCompare(right.label)),
		}));
});

const selectedService = computed<AdminSourceDefinition | null>(() => {
	const match = services.value.find(
		(service) => service.name === selectedServiceName.value,
	);
	return match ?? null;
});

const selectedRuleSpec = computed<AdminValidationSpec | null>(() => {
	if (!draft.value || selectedRulePath.value === "") {
		return null;
	}
	return draft.value.validation_specs[selectedRulePath.value] ?? null;
});

const selectedRuleUsageCount = computed(() =>
	selectedRulePath.value === "" ? 0 : ruleUsageCount(selectedRulePath.value),
);
const selectedProfileServiceCount = computed(() =>
	selectedProfileEntry.value?.[1].enabled_sources.length ?? 0,
);

const selectedRuleBounds = computed(() => {
	if (!selectedRuleSpec.value) {
		return [] as Array<{ field: string; bounds: AdminBounds }>;
	}
	return Object.keys(selectedRuleSpec.value.bounds)
		.sort((left, right) => left.localeCompare(right))
		.map((field) => ({
			field,
			bounds: selectedRuleSpec.value!.bounds[field] ?? {},
		}));
});

const hasUnsavedChanges = computed(() => {
	if (!draft.value) {
		return false;
	}
	return JSON.stringify(draft.value) !== baselineSignature.value;
});

onMounted(() => {
	void loadAdminConfig();
});

async function loadAdminConfig(): Promise<void> {
	loading.value = true;
	errorMessage.value = "";
	successMessage.value = "";

	try {
		const response = await fetch(adminConfigURL);
		if (!response.ok) {
			throw new Error(await readErrorMessage(response));
		}

		const payload = (await response.json()) as AdminConfigDocument;
		const normalized = normalizeDocument(payload);
		draft.value = normalized;
		baselineSignature.value = JSON.stringify(normalized);
		repairSelections();
	} catch (error) {
		errorMessage.value =
			error instanceof Error ? error.message : "Failed to load config.";
	} finally {
		loading.value = false;
	}
}

async function saveAdminConfig(): Promise<void> {
	if (!draft.value) {
		return;
	}

	saving.value = true;
	errorMessage.value = "";
	successMessage.value = "";

	try {
		const response = await fetch(adminConfigURL, {
			method: "PUT",
			headers: {
				"Content-Type": "application/json",
			},
			body: JSON.stringify(draft.value),
		});
		if (!response.ok) {
			throw new Error(await readErrorMessage(response));
		}

		const payload = (await response.json()) as AdminConfigDocument;
		const normalized = normalizeDocument(payload);
		draft.value = normalized;
		baselineSignature.value = JSON.stringify(normalized);
		repairSelections();
		successMessage.value = `Saved ${new Date().toLocaleTimeString()}`;
		await systemStore.loadAvailableServices();
	} catch (error) {
		errorMessage.value =
			error instanceof Error ? error.message : "Failed to save config.";
	} finally {
		saving.value = false;
	}
}

function repairSelections(): void {
	if (!draft.value) {
		selectedServiceName.value = "";
		selectedRulePath.value = "";
		return;
	}

	if (
		selectedServiceName.value === "" ||
		!draft.value.catalog.sources.some(
			(service) => service.name === selectedServiceName.value,
		)
	) {
		selectedServiceName.value = draft.value.catalog.sources[0]?.name ?? "";
	}

	if (
		selectedProfileName.value === "" ||
		!profileEntries.value.some(
			([profileName]) => profileName === selectedProfileName.value,
		)
	) {
		selectedProfileName.value = profileEntries.value[0]?.[0] ?? "";
	}

	if (
		selectedRulePath.value === "" ||
		!(selectedRulePath.value in draft.value.validation_specs)
	) {
		selectedRulePath.value = Object.keys(draft.value.validation_specs).sort(
			(left, right) => left.localeCompare(right),
		)[0] ?? "";
	}
}

function normalizeDocument(doc: AdminConfigDocument): AdminConfigDocument {
	const normalized = structuredClone(doc);
	normalized.catalog.profiles ??= {};
	normalized.catalog.sources ??= [];
	normalized.validation_specs ??= {};

	normalized.catalog.sources = normalized.catalog.sources.map((source, index) =>
		normalizeSource(source, index),
	);

	const normalizedSpecs: Record<string, AdminValidationSpec> = {};
	for (const [path, spec] of Object.entries(normalized.validation_specs)) {
		normalizedSpecs[normalizePath(path)] = normalizeValidationSpec(spec);
	}
	normalized.validation_specs = normalizedSpecs;

	return normalized;
}

function normalizeSource(
	source: AdminSourceDefinition,
	index: number,
): AdminSourceDefinition {
	const flowControl = {
		...createDefaultFlowControl(),
		...source.flow_control,
		backpressure: {
			...createDefaultFlowControl().backpressure,
			...source.flow_control?.backpressure,
		},
	};

	const normalized: AdminSourceDefinition = {
		...source,
		alias_name: source.alias_name ?? "",
		sql_target: source.sql_target || defaultSQLTarget(source.event_type),
		validation_file: normalizePath(source.validation_file),
		flow_control: flowControl,
	};

	if (normalized.mode === "simulator") {
		normalized.simulator = {
			...createDefaultSimulatorSettings(normalized.event_type, index),
			...source.simulator,
		};
		normalized.modbus = undefined;
	} else {
		normalized.modbus = {
			...createDefaultModbusSettings(index),
			...source.modbus,
		};
		normalized.simulator = undefined;
	}

	return normalized;
}

function normalizeValidationSpec(spec: AdminValidationSpec): AdminValidationSpec {
	return {
		required_fields: [...(spec.required_fields ?? [])],
		bounds: { ...(spec.bounds ?? {}) },
		anomaly_rules: [...(spec.anomaly_rules ?? [])],
	};
}

function defaultSQLTarget(eventType: ServiceEventType): string {
	return eventType === "valve" ? "valve_samples" : "temp_samples";
}

function defaultSimulatorProfile(eventType: ServiceEventType): string {
	return eventType === "valve" ? "valve_default" : "temperature_default";
}

function createDefaultFlowControl(): AdminFlowControlConfig {
	return {
		reserved_units: 64,
		adaptive_enabled: true,
		supports_backpressure: true,
		overload_policy: "drop_oldest",
		sampling_every_n: 4,
		backpressure: {
			high_interval_multiplier: 2,
			critical_interval_multiplier: 4,
		},
	};
}

function createDefaultModbusSettings(index: number): AdminModbusSourceSettings {
	return {
		address: "127.0.0.1:1502",
		mw_base: 1024 + index * 100,
		register_count: 6,
		slave_id: 1,
		interval: "",
	};
}

function createDefaultSimulatorSettings(
	eventType: ServiceEventType,
	index: number,
): AdminSimulatorSourceSettings {
	return {
		interval: "100ms",
		seed: 42 + index,
		profile: defaultSimulatorProfile(eventType),
	};
}

function createDefaultValidationSpec(
	eventType: ServiceEventType,
): AdminValidationSpec {
	if (eventType === "valve") {
		return {
			required_fields: [
				"id",
				"timestamp",
				"sensor_type",
				"valve_number",
				"is_open",
				"flow_rate",
			],
			bounds: {
				valve_number: {
					min: 0,
					max: 200,
				},
				flow_rate: {
					min: 0,
					max: 1000,
				},
			},
			anomaly_rules: [
				{
					field: "flow_rate",
					op: ">",
					value: 500,
					label: "valve_flow_high",
				},
			],
		};
	}

	return {
		required_fields: [
			"id",
			"timestamp",
			"sensor_type",
			"sensor_number",
			"temperature",
			"heater_power",
		],
		bounds: {
			temperature: {
				min: -100,
				max: 200,
			},
			heater_power: {
				min: 0,
				max: 100,
			},
		},
		anomaly_rules: [
			{
				field: "temperature",
				op: ">",
				value: 80,
				label: "temperature_high",
			},
		],
	};
}

function displayServiceName(service: AdminSourceDefinition): string {
	return service.alias_name?.trim() || service.name;
}

function formatConfigLabel(value: string): string {
	return value
		.replace(/_/g, " ")
		.trim()
		.replace(/\b\w/g, (char) => char.toUpperCase());
}

function formatNumericValue(value: number): string {
	return Number.isInteger(value) ? String(value) : String(value);
}

function normalizePath(path: string): string {
	return path.trim().replace(/\\/g, "/");
}

function updateRuntimeBufferField(key: string, event: Event): void {
	if (!draft.value) {
		return;
	}
	const input = event.target as HTMLInputElement;
	const parsed = Number(input.value);
	if (Number.isFinite(parsed)) {
		const buffering = draft.value.catalog.runtime.buffering as Record<
			string,
			unknown
		>;
		if (typeof buffering[key] === "number") {
			buffering[key] = parsed;
		}
	}
}

function updateRuntimePressureField(key: string, event: Event): void {
	if (!draft.value) {
		return;
	}
	const input = event.target as HTMLInputElement;
	const parsed = Number(input.value);
	if (Number.isFinite(parsed)) {
		(draft.value.catalog.runtime.buffering.pressure as Record<string, number>)[key] =
			parsed;
	}
}

function inferRuleEventType(spec?: AdminValidationSpec): ServiceEventType {
	const requiredFields = new Set(spec?.required_fields ?? []);
	if (
		requiredFields.has("flow_rate") ||
		requiredFields.has("valve_number") ||
		requiredFields.has("is_open")
	) {
		return "valve";
	}
	return "temperature";
}

function matchingRulePaths(eventType: ServiceEventType): string[] {
	return rulePaths.value.filter(
		(path) => inferRuleEventType(draft.value?.validation_specs[path]) === eventType,
	);
}

function ensureRuleFileForEventType(eventType: ServiceEventType): string {
	if (!draft.value) {
		return "";
	}

	const existing = matchingRulePaths(eventType)[0];
	if (existing) {
		return existing;
	}

	const nextPath = createUniqueRulePath(eventType);
	draft.value.validation_specs[nextPath] = createDefaultValidationSpec(eventType);
	selectedRulePath.value = nextPath;
	return nextPath;
}

function createUniqueServiceName(prefix: string): string {
	const existingNames = new Set(services.value.map((service) => service.name));
	let index = 1;
	let candidate = `${prefix}_${index}`;
	while (existingNames.has(candidate)) {
		index += 1;
		candidate = `${prefix}_${index}`;
	}
	return candidate;
}

function createUniqueRulePath(eventType: ServiceEventType): string {
	const existing = new Set(rulePaths.value);
	let index = 1;
	let candidate = normalizePath(`config/validation/${eventType}.json`);
	while (existing.has(candidate)) {
		index += 1;
		candidate = normalizePath(`config/validation/${eventType}-${index}.json`);
	}
	return candidate;
}

function addService(): void {
	if (!draft.value) {
		return;
	}

	const eventType = selectedService.value?.event_type ?? "temperature";
	const serviceIndex = draft.value.catalog.sources.length;
	const validationFile = ensureRuleFileForEventType(eventType);
	const name = createUniqueServiceName(eventType === "valve" ? "valve_service" : "temp_service");

	draft.value.catalog.sources.push({
		name,
		alias_name: eventType === "valve" ? "New Valve Service" : "New Temperature Service",
		event_type: eventType,
		mode: "simulator",
		enabled: true,
		validation_file: validationFile,
		sql_target: defaultSQLTarget(eventType),
		ml_enabled: true,
		flow_control: createDefaultFlowControl(),
		simulator: createDefaultSimulatorSettings(eventType, serviceIndex),
	});

	selectedServiceName.value = name;
	successMessage.value = "";
	errorMessage.value = "";
}

function removeSelectedService(): void {
	if (!draft.value || !selectedService.value) {
		return;
	}

	const removedName = selectedService.value.name;
	draft.value.catalog.sources = draft.value.catalog.sources.filter(
		(service) => service.name !== removedName,
	);

	for (const profile of Object.values(draft.value.catalog.profiles ?? {})) {
		profile.enabled_sources = profile.enabled_sources.filter(
			(name) => name !== removedName,
		);
	}

	repairSelections();
}

function updateSelectedServiceName(event: Event): void {
	if (!draft.value || !selectedService.value) {
		return;
	}

	const input = event.target as HTMLInputElement;
	const nextName = input.value.trim();
	if (nextName === "" || nextName === selectedService.value.name) {
		return;
	}
	if (services.value.some((service) => service.name === nextName)) {
		errorMessage.value = `Service name "${nextName}" already exists.`;
		return;
	}

	const previousName = selectedService.value.name;
	selectedService.value.name = nextName;
	selectedServiceName.value = nextName;
	for (const profile of Object.values(draft.value.catalog.profiles ?? {})) {
		profile.enabled_sources = profile.enabled_sources.map((name) =>
			name === previousName ? nextName : name,
		);
	}
	errorMessage.value = "";
}

function setSelectedServiceEventType(event: Event): void {
	if (!selectedService.value) {
		return;
	}

	const target = event.target as HTMLSelectElement;
	const nextEventType = target.value as ServiceEventType;
	selectedService.value.event_type = nextEventType;
	selectedService.value.sql_target = defaultSQLTarget(nextEventType);
	selectedService.value.validation_file = ensureRuleFileForEventType(nextEventType);
	if (selectedService.value.mode === "simulator" && selectedService.value.simulator) {
		selectedService.value.simulator.profile =
			defaultSimulatorProfile(nextEventType);
	}
}

function setSelectedServiceMode(event: Event): void {
	if (!selectedService.value) {
		return;
	}

	const target = event.target as HTMLSelectElement;
	const nextMode = target.value as ServiceMode;
	selectedService.value.mode = nextMode;
	if (nextMode === "simulator") {
		selectedService.value.simulator = createDefaultSimulatorSettings(
			selectedService.value.event_type,
			services.value.findIndex(
				(service) => service.name === selectedService.value?.name,
			),
		);
		selectedService.value.modbus = undefined;
		return;
	}

	selectedService.value.modbus = createDefaultModbusSettings(
		services.value.findIndex(
			(service) => service.name === selectedService.value?.name,
		),
	);
	selectedService.value.modbus.register_count =
		selectedService.value.event_type === "valve" ? 5 : 6;
	selectedService.value.simulator = undefined;
}

function addProfile(): void {
	if (!draft.value) {
		return;
	}

	const existing = new Set(Object.keys(draft.value.catalog.profiles ?? {}));
	let index = 1;
	let candidate = `profile_${index}`;
	while (existing.has(candidate)) {
		index += 1;
		candidate = `profile_${index}`;
	}

	draft.value.catalog.profiles ??= {};
	draft.value.catalog.profiles[candidate] = {
		enabled_sources: [],
	};
	selectedProfileName.value = candidate;
}

function renameProfile(previousName: string, event: Event): void {
	if (!draft.value || !draft.value.catalog.profiles) {
		return;
	}

	const input = event.target as HTMLInputElement;
	const nextName = input.value.trim();
	if (nextName === "" || nextName === previousName) {
		return;
	}
	if (draft.value.catalog.profiles[nextName]) {
		errorMessage.value = `Profile "${nextName}" already exists.`;
		return;
	}

	const nextProfiles: Record<string, { enabled_sources: string[] }> = {};
	for (const [name, profile] of Object.entries(draft.value.catalog.profiles)) {
		nextProfiles[name === previousName ? nextName : name] = profile;
	}
	draft.value.catalog.profiles = nextProfiles;
	if (selectedProfileName.value === previousName) {
		selectedProfileName.value = nextName;
	}
	errorMessage.value = "";
}

function removeProfile(name: string): void {
	if (!draft.value?.catalog.profiles) {
		return;
	}
	delete draft.value.catalog.profiles[name];
}

function toggleProfileService(
	profileName: string,
	serviceName: string,
	checked: boolean,
): void {
	const profile = draft.value?.catalog.profiles?.[profileName];
	if (!profile) {
		return;
	}

	const nextSet = new Set(profile.enabled_sources);
	if (checked) {
		nextSet.add(serviceName);
	} else {
		nextSet.delete(serviceName);
	}
	profile.enabled_sources = Array.from(nextSet).sort((left, right) =>
		left.localeCompare(right),
	);
}

function addRuleFile(): void {
	if (!draft.value) {
		return;
	}

	const eventType = selectedService.value?.event_type ?? "temperature";
	const nextPath = createUniqueRulePath(eventType);
	draft.value.validation_specs[nextPath] = createDefaultValidationSpec(eventType);
	selectedRulePath.value = nextPath;
	errorMessage.value = "";
	successMessage.value = "";
}

function renameSelectedRulePath(event: Event): void {
	if (!draft.value || !selectedRuleSpec.value || selectedRulePath.value === "") {
		return;
	}

	const input = event.target as HTMLInputElement;
	const nextPath = normalizePath(input.value);
	if (nextPath === "" || nextPath === selectedRulePath.value) {
		return;
	}
	if (draft.value.validation_specs[nextPath]) {
		errorMessage.value = `Rule file "${nextPath}" already exists.`;
		return;
	}

	const previousPath = selectedRulePath.value;
	const spec = draft.value.validation_specs[previousPath];
	if (!spec) {
		return;
	}
	delete draft.value.validation_specs[previousPath];
	draft.value.validation_specs[nextPath] = spec;
	for (const service of draft.value.catalog.sources) {
		if (service.validation_file === previousPath) {
			service.validation_file = nextPath;
		}
	}
	selectedRulePath.value = nextPath;
	errorMessage.value = "";
}

function removeSelectedRule(): void {
	if (!draft.value || selectedRulePath.value === "") {
		return;
	}
	if (selectedRuleUsageCount.value > 0) {
		errorMessage.value =
			"Rule files must be unassigned from services before removal.";
		return;
	}

	delete draft.value.validation_specs[selectedRulePath.value];
	repairSelections();
}

function ruleUsageCount(path: string): number {
	return services.value.filter((service) => service.validation_file === path).length;
}

function updateRequiredField(index: number, event: Event): void {
	if (!selectedRuleSpec.value) {
		return;
	}
	const input = event.target as HTMLInputElement;
	selectedRuleSpec.value.required_fields[index] = input.value;
}

function addRequiredField(): void {
	if (!selectedRuleSpec.value) {
		return;
	}
	selectedRuleSpec.value.required_fields.push("");
}

function removeRequiredField(index: number): void {
	selectedRuleSpec.value?.required_fields.splice(index, 1);
}

function addBoundRow(): void {
	if (!selectedRuleSpec.value) {
		return;
	}

	let index = 1;
	let candidate = `field_${index}`;
	while (selectedRuleSpec.value.bounds[candidate]) {
		index += 1;
		candidate = `field_${index}`;
	}
	selectedRuleSpec.value.bounds[candidate] = {};
}

function renameBoundField(previousField: string, event: Event): void {
	if (!selectedRuleSpec.value) {
		return;
	}

	const input = event.target as HTMLInputElement;
	const nextField = input.value.trim();
	if (nextField === "" || nextField === previousField) {
		return;
	}
	if (selectedRuleSpec.value.bounds[nextField]) {
		errorMessage.value = `Bounds field "${nextField}" already exists.`;
		return;
	}

	const currentBounds = selectedRuleSpec.value.bounds[previousField];
	if (!currentBounds) {
		return;
	}
	selectedRuleSpec.value.bounds[nextField] = currentBounds;
	delete selectedRuleSpec.value.bounds[previousField];
	errorMessage.value = "";
}

function removeBoundField(field: string): void {
	if (!selectedRuleSpec.value) {
		return;
	}
	delete selectedRuleSpec.value.bounds[field];
}

function updateBoundLimit(
	field: string,
	limit: "min" | "max",
	event: Event,
): void {
	if (!selectedRuleSpec.value) {
		return;
	}

	const input = event.target as HTMLInputElement;
	const rawValue = input.value.trim();
	selectedRuleSpec.value.bounds[field] ??= {};
	if (rawValue === "") {
		delete selectedRuleSpec.value.bounds[field][limit];
		return;
	}

	const parsed = Number(rawValue);
	if (Number.isFinite(parsed)) {
		selectedRuleSpec.value.bounds[field][limit] = parsed;
	}
}

function addAnomalyRule(): void {
	if (!selectedRuleSpec.value) {
		return;
	}
	selectedRuleSpec.value.anomaly_rules.push({
		field: "",
		op: ">",
		value: 0,
		label: "",
	});
}

function removeAnomalyRule(index: number): void {
	selectedRuleSpec.value?.anomaly_rules.splice(index, 1);
}

async function readErrorMessage(response: Response): Promise<string> {
	try {
		const payload = (await response.json()) as { error?: string };
		if (payload?.error) {
			return payload.error;
		}
	} catch {
		// fall through to status text
	}
	return `Request failed with status ${response.status}`;
}

function anomalyRuleKey(rule: AdminAnomalyRule, index: number): string {
	return `${rule.field}-${rule.label}-${index}`;
}
</script>

<template>
	<main class="admin-shell">
		<section class="admin-hero">
			<div>
				<p class="eyebrow">Admin Control Panel</p>
				<h1>System Configuration</h1>
				<p class="hero-copy">
					Edit services, rule files, and runtime settings from one control
					panel.
				</p>
			</div>
		</section>

		<section class="action-bar">
			<div class="status-row">
				<span class="status-pill" :class="{ dirty: hasUnsavedChanges }">
					{{ hasUnsavedChanges ? "Unsaved changes" : "In sync" }}
				</span>
				<span class="status-pill status-pill-muted">
					{{ serviceCount }} services
				</span>
				<span class="status-pill status-pill-muted">
					{{ ruleCount }} rule files
				</span>
				<span v-if="successMessage" class="status-pill status-pill-success">
					{{ successMessage }}
				</span>
			</div>

			<div class="action-row">
				<Button
					label="Reload From Disk"
					icon="pi pi-refresh"
					severity="secondary"
					outlined
					@click="loadAdminConfig"
					:disabled="loading || saving"
				/>
				<Button
					label="Save Changes"
					icon="pi pi-save"
					@click="saveAdminConfig"
					:loading="saving"
					:disabled="loading || !draft"
				/>
			</div>
		</section>

		<section v-if="errorMessage" class="feedback-panel feedback-panel-error">
			<p>{{ errorMessage }}</p>
		</section>

		<section v-if="loading" class="feedback-panel">
			<p>Loading configuration from the ingest API...</p>
		</section>

		<template v-else-if="draft">
			<section class="workspace-tabs" role="tablist" aria-label="Admin sections">
				<button
					v-for="tab in adminTabs"
					:key="tab.id"
					type="button"
					class="workspace-tab"
					:class="{ active: activeTab === tab.id }"
					@click="activeTab = tab.id"
				>
					<strong>{{ tab.label }}</strong>
					<small>{{ tab.helper }}</small>
				</button>
			</section>

			<section v-if="activeTab === 'overview'" class="overview-shell">
				<article class="editor-panel">
					<div class="panel-header">
						<div>
							<p class="panel-eyebrow">Overview</p>
							<h2>Configuration Snapshot</h2>
						</div>
					</div>

					<div class="summary-grid">
						<article
							v-for="card in summaryCards"
							:key="card.label"
							class="summary-card"
						>
							<span class="summary-label">{{ card.label }}</span>
							<strong>{{ card.value }}</strong>
							<p>{{ card.detail }}</p>
						</article>
					</div>
				</article>

				<article class="editor-panel">
					<div class="panel-header">
						<div>
							<p class="panel-eyebrow">Overview</p>
							<h2>Quick Edits</h2>
						</div>
					</div>

					<div class="field-grid">
						<label
							v-for="field in runtimeBufferFields"
							:key="field.key"
							class="field"
						>
							<span>{{ field.label }}</span>
							<input
								:value="field.value"
								type="number"
								step="0.01"
								:min="field.key === 'shared_units' ? 1 : 0.01"
								:max="
									field.key === 'sampling_shared_threshold' ? 0.99 : undefined
								"
								@input="updateRuntimeBufferField(field.key, $event)"
							/>
						</label>
					</div>

					<div class="overview-notes">
						<div class="overview-note">
							<span class="summary-label">Selected Service</span>
							<strong>{{ selectedService ? displayServiceName(selectedService) : "None" }}</strong>
						</div>
						<div class="overview-note">
							<span class="summary-label">Selected Rule</span>
							<strong>{{ selectedRulePath || "None" }}</strong>
						</div>
						<div class="overview-note">
							<span class="summary-label">Profiles</span>
							<strong>{{ profileCount }}</strong>
						</div>
					</div>
				</article>
			</section>

			<section v-else-if="activeTab === 'services'">
				<article class="editor-panel">
					<div v-if="selectedService" class="detail-stack">
						<div class="panel-header editor-panel-header">
							<div>
								<p class="panel-eyebrow">Services</p>
								<h2>Service Settings</h2>
							</div>
							<div class="editor-panel-actions">
								<select v-model="selectedServiceName" class="selector-control">
									<option
										v-for="service in serviceOptions"
										:key="service.name"
										:value="service.name"
									>
										{{ displayServiceName(service) }}
									</option>
								</select>
								<Button
									label="Add Service"
									icon="pi pi-plus"
									severity="secondary"
									outlined
									@click="addService"
								/>
								<Button
									label="Remove Service"
									icon="pi pi-trash"
									severity="danger"
									outlined
									@click="removeSelectedService"
								/>
							</div>
						</div>

						<div class="panel-subheader">
							<div>
								<h2>{{ displayServiceName(selectedService) }}</h2>
								<p class="panel-subcopy">
									{{ selectedService.name }} · {{ selectedService.event_type }} ·
									{{ selectedService.mode }}
								</p>
							</div>
						</div>

						<div class="field-grid">
							<label class="field">
								<span>Service Key</span>
								<input
									:value="selectedService.name"
									type="text"
									@input="updateSelectedServiceName"
								/>
							</label>
							<label class="field">
								<span>Alias Name</span>
								<input v-model="selectedService.alias_name" type="text" />
							</label>
							<label class="field">
								<span>Event Type</span>
								<select
									:value="selectedService.event_type"
									@change="setSelectedServiceEventType"
								>
									<option value="temperature">temperature</option>
									<option value="valve">valve</option>
								</select>
							</label>
							<label class="field">
								<span>Mode</span>
								<select
									:value="selectedService.mode"
									@change="setSelectedServiceMode"
								>
									<option value="simulator">simulator</option>
									<option value="modbus">modbus</option>
								</select>
							</label>
							<label class="field field-toggle">
								<span>Enabled</span>
								<input v-model="selectedService.enabled" type="checkbox" />
							</label>
							<label class="field field-toggle">
								<span>ML Enabled</span>
								<input v-model="selectedService.ml_enabled" type="checkbox" />
							</label>
							<label class="field">
								<span>Validation File</span>
								<select v-model="selectedService.validation_file">
									<option v-for="path in rulePaths" :key="path" :value="path">
										{{ path }}
									</option>
								</select>
							</label>
							<label class="field field-readonly">
								<span>SQL Target</span>
								<input :value="selectedService.sql_target" type="text" readonly />
							</label>
						</div>

						<details class="accordion-panel" open>
							<summary>Flow Control</summary>
							<div class="accordion-body">
								<div class="field-grid">
									<label class="field">
										<span>Reserved Units</span>
										<input
											v-model.number="selectedService.flow_control.reserved_units"
											type="number"
											min="1"
										/>
									</label>
									<label class="field">
										<span>Overload Policy</span>
										<input
											v-model="selectedService.flow_control.overload_policy"
											type="text"
										/>
									</label>
									<label class="field">
										<span>Sampling Every N</span>
										<input
											v-model.number="selectedService.flow_control.sampling_every_n"
											type="number"
											min="1"
										/>
									</label>
									<label class="field field-toggle">
										<span>Adaptive Enabled</span>
										<input
											v-model="selectedService.flow_control.adaptive_enabled"
											type="checkbox"
										/>
									</label>
									<label class="field field-toggle">
										<span>Supports Backpressure</span>
										<input
											v-model="selectedService.flow_control.supports_backpressure"
											type="checkbox"
										/>
									</label>
									<label class="field">
										<span>High Multiplier</span>
										<input
											v-model.number="
												selectedService.flow_control.backpressure
													.high_interval_multiplier
											"
											type="number"
											min="1"
										/>
									</label>
									<label class="field">
										<span>Critical Multiplier</span>
										<input
											v-model.number="
												selectedService.flow_control.backpressure
													.critical_interval_multiplier
											"
											type="number"
											min="1"
										/>
									</label>
								</div>
							</div>
						</details>

						<details
							v-if="
								selectedService.mode === 'simulator' && selectedService.simulator
							"
							class="accordion-panel"
						>
							<summary>Simulator Settings</summary>
							<div class="accordion-body">
								<div class="field-grid">
									<label class="field">
										<span>Interval</span>
										<input
											v-model="selectedService.simulator.interval"
											type="text"
										/>
									</label>
									<label class="field">
										<span>Seed</span>
										<input
											v-model.number="selectedService.simulator.seed"
											type="number"
										/>
									</label>
									<label class="field">
										<span>Profile</span>
										<input
											v-model="selectedService.simulator.profile"
											type="text"
										/>
									</label>
								</div>
							</div>
						</details>

						<details
							v-if="selectedService.mode === 'modbus' && selectedService.modbus"
							class="accordion-panel"
						>
							<summary>Modbus Settings</summary>
							<div class="accordion-body">
								<div class="field-grid">
									<label class="field">
										<span>Address</span>
										<input v-model="selectedService.modbus.address" type="text" />
									</label>
									<label class="field">
										<span>MW Base</span>
										<input
											v-model.number="selectedService.modbus.mw_base"
											type="number"
											min="0"
										/>
									</label>
									<label class="field">
										<span>Register Count</span>
										<input
											v-model.number="selectedService.modbus.register_count"
											type="number"
											min="1"
										/>
									</label>
									<label class="field">
										<span>Slave ID</span>
										<input
											v-model.number="selectedService.modbus.slave_id"
											type="number"
											min="1"
										/>
									</label>
									<label class="field">
										<span>Interval Override</span>
										<input
											v-model="selectedService.modbus.interval"
											type="text"
										/>
									</label>
								</div>
							</div>
						</details>
					</div>

					<div v-else class="empty-state">
						Select a service from the list to edit it.
					</div>
				</article>
			</section>

			<section v-else-if="activeTab === 'rules'">
				<article class="editor-panel">
					<div v-if="selectedRuleSpec" class="detail-stack">
						<div class="panel-header editor-panel-header">
							<div>
								<p class="panel-eyebrow">Rules</p>
								<h2>Validation File Settings</h2>
							</div>
							<div class="editor-panel-actions">
								<select v-model="selectedRulePath" class="selector-control">
									<option v-for="path in ruleOptions" :key="path" :value="path">
										{{ path }}
									</option>
								</select>
								<Button
									label="Add Rule File"
									icon="pi pi-plus"
									severity="secondary"
									outlined
									@click="addRuleFile"
								/>
								<Button
									label="Remove Rule File"
									icon="pi pi-trash"
									severity="danger"
									outlined
									@click="removeSelectedRule"
									:disabled="selectedRuleUsageCount > 0"
								/>
							</div>
						</div>

						<div class="panel-subheader">
							<div>
								<h2>{{ selectedRulePath }}</h2>
								<p class="panel-subcopy">
									{{ inferRuleEventType(selectedRuleSpec) }} ·
									{{ selectedRuleUsageCount }} linked services
								</p>
							</div>
						</div>

						<div class="field-grid">
							<label class="field field-wide">
								<span>Rule File Path</span>
								<input
									:value="selectedRulePath"
									type="text"
									@input="renameSelectedRulePath"
								/>
							</label>
							<div class="field field-readonly">
								<span>Linked Services</span>
								<div class="readonly-card">{{ selectedRuleUsageCount }}</div>
							</div>
						</div>

						<details class="accordion-panel" open>
							<summary>Required Fields</summary>
							<div class="accordion-body">
								<div class="stack-editor">
									<div
										v-for="(field, index) in selectedRuleSpec.required_fields"
										:key="`${field}-${index}`"
										class="editor-row"
									>
										<input
											:value="field"
											type="text"
											placeholder="field name"
											@input="updateRequiredField(index, $event)"
										/>
										<Button
											icon="pi pi-times"
											severity="secondary"
											text
											@click="removeRequiredField(index)"
										/>
									</div>
									<Button
										label="Add Field"
										icon="pi pi-plus"
										severity="secondary"
										outlined
										@click="addRequiredField"
									/>
								</div>
							</div>
						</details>

						<details class="accordion-panel">
							<summary>Bounds</summary>
							<div class="accordion-body">
								<div class="stack-editor">
									<div
										v-for="entry in selectedRuleBounds"
										:key="entry.field"
										class="editor-row editor-row-wide"
									>
										<input
											:value="entry.field"
											type="text"
											placeholder="field"
											@input="renameBoundField(entry.field, $event)"
										/>
										<input
											:value="entry.bounds.min ?? ''"
											type="number"
											step="0.01"
											placeholder="min"
											@input="updateBoundLimit(entry.field, 'min', $event)"
										/>
										<input
											:value="entry.bounds.max ?? ''"
											type="number"
											step="0.01"
											placeholder="max"
											@input="updateBoundLimit(entry.field, 'max', $event)"
										/>
										<Button
											icon="pi pi-times"
											severity="secondary"
											text
											@click="removeBoundField(entry.field)"
										/>
									</div>
									<Button
										label="Add Bound"
										icon="pi pi-plus"
										severity="secondary"
										outlined
										@click="addBoundRow"
									/>
								</div>
							</div>
						</details>

						<details class="accordion-panel">
							<summary>Anomaly Rules</summary>
							<div class="accordion-body">
								<div class="stack-editor">
									<div
										v-for="(rule, index) in selectedRuleSpec.anomaly_rules"
										:key="anomalyRuleKey(rule, index)"
										class="editor-row editor-row-quad"
									>
										<input
											v-model="rule.field"
											type="text"
											placeholder="field"
										/>
										<select v-model="rule.op">
											<option value=">">&gt;</option>
											<option value=">=">&gt;=</option>
											<option value="<">&lt;</option>
											<option value="<=">&lt;=</option>
											<option value="==">==</option>
											<option value="!=">!=</option>
										</select>
										<input
											v-model.number="rule.value"
											type="number"
											step="0.01"
										/>
										<input
											v-model="rule.label"
											type="text"
											placeholder="label"
										/>
										<Button
											icon="pi pi-times"
											severity="secondary"
											text
											@click="removeAnomalyRule(index)"
										/>
									</div>
									<Button
										label="Add Rule"
										icon="pi pi-plus"
										severity="secondary"
										outlined
										@click="addAnomalyRule"
									/>
								</div>
							</div>
						</details>
					</div>

					<div v-else class="empty-state">
						Select a validation file from the list to edit it.
					</div>
				</article>
			</section>

			<section v-else-if="activeTab === 'runtime'">
				<article class="editor-panel">
					<div class="panel-header">
						<div>
							<p class="panel-eyebrow">Runtime</p>
							<h2>System Behavior</h2>
						</div>
					</div>

					<div class="runtime-simple-grid">
						<div class="runtime-section">
							<h3>Buffering</h3>
						</div>
						<div class="runtime-buffer-grid">
							<label
								v-for="field in runtimeBufferFields"
								:key="field.key"
								class="field"
							>
								<span>{{ field.label }}</span>
								<input
									:value="field.value"
									type="number"
									step="0.01"
									:min="field.key === 'shared_units' ? 1 : 0.01"
									:max="
										field.key === 'sampling_shared_threshold' ? 0.99 : undefined
									"
									@input="updateRuntimeBufferField(field.key, $event)"
								/>
							</label>
						</div>

						<details class="accordion-panel" open>
							<summary>Pressure Thresholds</summary>
							<div class="accordion-body">
								<div class="runtime-pressure-stack">
									<div
										v-for="section in runtimePressureSections"
										:key="section.id"
										class="runtime-pressure-row"
									>
										<div class="runtime-pressure-row__label">
											{{ section.label }}
										</div>
										<div class="runtime-pressure-row__inputs">
											<label
												v-for="field in section.fields"
												:key="field.key"
												class="field"
											>
												<span>{{ field.label }}</span>
												<input
													:value="field.value"
													type="number"
													step="0.01"
													@input="updateRuntimePressureField(field.key, $event)"
												/>
											</label>
										</div>
									</div>
								</div>
							</div>
						</details>
					</div>
				</article>
			</section>

			<section v-else-if="activeTab === 'profiles'">
				<article class="editor-panel">
					<div v-if="selectedProfileEntry" class="detail-stack">
						<div class="panel-header editor-panel-header">
							<div>
								<p class="panel-eyebrow">Profiles</p>
								<h2>Assignments</h2>
							</div>
							<div class="editor-panel-actions">
								<select v-model="selectedProfileName" class="selector-control">
									<option
										v-for="profileName in profileOptions"
										:key="profileName"
										:value="profileName"
									>
										{{ profileName }}
									</option>
								</select>
								<Button
									label="Add Profile"
									icon="pi pi-plus"
									severity="secondary"
									outlined
									@click="addProfile"
								/>
								<Button
									label="Remove Profile"
									icon="pi pi-trash"
									severity="danger"
									outlined
									@click="removeProfile(selectedProfileEntry[0])"
								/>
							</div>
						</div>

						<div class="panel-subheader">
							<div>
								<h2>{{ selectedProfileEntry[0] }}</h2>
								<p class="panel-subcopy">
									{{ selectedProfileServiceCount }} assigned services
								</p>
							</div>
						</div>

						<div class="field-grid">
							<label class="field">
								<span>Profile Name</span>
								<input
									:value="selectedProfileEntry[0]"
									type="text"
									@input="renameProfile(selectedProfileEntry[0], $event)"
								/>
							</label>
						</div>

						<details class="accordion-panel" open>
							<summary>Service Assignments</summary>
							<div class="accordion-body">
								<div class="profile-service-list">
									<label
										v-for="service in services"
										:key="`${selectedProfileEntry[0]}-${service.name}`"
										class="profile-service-row"
									>
										<input
											type="checkbox"
											:checked="
												selectedProfileEntry[1].enabled_sources.includes(
													service.name,
												)
											"
											@change="
												toggleProfileService(
													selectedProfileEntry[0],
													service.name,
													($event.target as HTMLInputElement).checked,
												)
											"
										/>
										<div>
											<strong>{{ displayServiceName(service) }}</strong>
											<small>{{ service.name }}</small>
										</div>
									</label>
								</div>
							</div>
						</details>
					</div>

					<div v-else class="empty-state">
						No profiles yet. Add one to start assigning services.
					</div>
				</article>
			</section>
		</template>
	</main>
</template>

<style scoped>
.admin-shell {
	display: grid;
	gap: 1.5rem;
	padding: 1.8rem;
}

.admin-hero {
	padding: 1.5rem;
	border: 1px solid #d7e4f7;
	border-radius: 24px;
	background:
		radial-gradient(circle at top right, rgba(16, 185, 129, 0.18), transparent 34%),
		linear-gradient(135deg, #f8fbff 0%, #eef6ff 45%, #f8fff9 100%);
}

.eyebrow,
.panel-eyebrow {
	margin: 0 0 0.4rem;
	font-size: 0.76rem;
	font-weight: 800;
	letter-spacing: 0.12em;
	text-transform: uppercase;
	color: #2f5d8a;
}

h1,
h2,
h3 {
	margin: 0;
	color: #15324b;
}

h1 {
	font-size: clamp(1.85rem, 2.8vw, 2.4rem);
	line-height: 1.05;
}

h2 {
	font-size: 1.1rem;
}

h3 {
	font-size: 0.98rem;
}

.hero-copy {
	max-width: 42rem;
	margin: 0.75rem 0 0;
	line-height: 1.55;
	color: #4b647a;
}

.summary-card,
.editor-panel,
.feedback-panel {
	border: 1px solid #dbe7f3;
	border-radius: 20px;
	background: #ffffff;
	box-shadow: 0 16px 34px rgba(12, 34, 58, 0.06);
}

.summary-label {
	font-size: 0.78rem;
	font-weight: 700;
	text-transform: uppercase;
	letter-spacing: 0.08em;
	color: #668099;
}

.action-bar {
	display: flex;
	justify-content: space-between;
	align-items: center;
	gap: 1rem;
	flex-wrap: wrap;
	padding: 1rem 1.2rem;
	border-radius: 18px;
	background: #f7fafc;
	border: 1px solid #dbe7f3;
}

.workspace-tabs,
.overview-shell,
.overview-notes {
	display: grid;
	gap: 1rem;
}

.workspace-tabs {
	grid-template-columns: repeat(5, minmax(0, 1fr));
}

.workspace-tab {
	display: grid;
	gap: 0.18rem;
	padding: 0.85rem 1rem;
	border-radius: 16px;
	border: 1px solid #d7e3ef;
	background: #ffffff;
	text-align: left;
	cursor: pointer;
	color: #48627a;
}

.workspace-tab strong {
	color: #18344e;
	font-size: 0.95rem;
}

.workspace-tab small {
	color: #678198;
}

.workspace-tab.active {
	border-color: #4b93d0;
	background: #eef6ff;
	box-shadow: inset 0 0 0 1px rgba(75, 147, 208, 0.18);
}

.overview-shell {
	grid-template-columns: minmax(0, 1.2fr) minmax(18rem, 0.8fr);
}

.overview-notes {
	grid-template-columns: repeat(3, minmax(0, 1fr));
	margin-top: 1.25rem;
	padding-top: 1.25rem;
	border-top: 1px solid #e3edf6;
}

.overview-note {
	display: grid;
	gap: 0.35rem;
}

.overview-note strong {
	color: #18344e;
}

.status-row,
.action-row,
.panel-header,
.editor-row {
	display: flex;
	align-items: center;
	gap: 0.75rem;
}

.status-row {
	flex-wrap: wrap;
}

.status-pill {
	padding: 0.45rem 0.8rem;
	border-radius: 999px;
	font-size: 0.84rem;
	font-weight: 700;
	background: #e7f0fb;
	color: #24527a;
}

.status-pill.dirty {
	background: #fff3d6;
	color: #9a6200;
}

.status-pill-muted {
	background: #eef4f8;
	color: #58738b;
}

.status-pill-success {
	background: #dff6e7;
	color: #197548;
}

.feedback-panel {
	padding: 1rem 1.2rem;
}

.feedback-panel p {
	margin: 0;
	color: #405d75;
}

.feedback-panel-error {
	border-color: #f1b6b6;
	background: #fff6f6;
}

.feedback-panel-error p {
	color: #a03c3c;
}

.summary-grid,
.field-grid {
	display: grid;
	gap: 1.25rem;
}

.summary-grid {
	grid-template-columns: repeat(3, minmax(0, 1fr));
}

.summary-card {
	padding: 1.2rem;
	display: grid;
	gap: 0.55rem;
}

.summary-card strong {
	font-size: 1.85rem;
	color: #15324b;
}

.summary-card p {
	margin: 0;
	color: #577088;
	line-height: 1.55;
}

.editor-panel {
	padding: 1.2rem;
}

.panel-header {
	justify-content: space-between;
	margin-bottom: 1rem;
}

.editor-panel-header {
	align-items: start;
}

.editor-panel-actions,
.panel-subheader {
	display: flex;
	align-items: center;
	gap: 0.75rem;
	flex-wrap: wrap;
}

.editor-panel-actions {
	justify-content: flex-end;
}

.selector-control {
	min-width: 16rem;
	max-width: 24rem;
}

.panel-subheader {
	justify-content: space-between;
	margin-top: -0.15rem;
}

.panel-subcopy {
	margin: 0.2rem 0 0;
	color: #5f778d;
}

.field-grid {
	grid-template-columns: repeat(2, minmax(0, 1fr));
}

.runtime-simple-grid,
.runtime-buffer-grid,
.runtime-pressure-stack,
.overview-shell {
	display: grid;
	gap: 1rem;
}

.runtime-section {
	grid-column: 1 / -1;
	padding-top: 0.15rem;
}

.runtime-section h3 {
	font-size: 0.98rem;
	color: #15324b;
}

.runtime-simple-grid {
	gap: 1.5rem;
}

.runtime-buffer-grid {
	grid-template-columns: repeat(2, minmax(12rem, 18rem));
	gap: 1.25rem;
	align-items: start;
}

.runtime-pressure-stack {
	gap: 1.2rem;
}

.runtime-pressure-row {
	display: grid;
	grid-template-columns: minmax(8rem, 10rem) minmax(0, 24rem);
	gap: 1.25rem;
	align-items: start;
}

.runtime-pressure-row__label {
	padding-top: 2rem;
	font-size: 1rem;
	font-weight: 700;
	color: #45627a;
}

.runtime-pressure-row__inputs {
	display: grid;
	grid-template-columns: repeat(2, minmax(10rem, 12rem));
	gap: 1rem 1.25rem;
}

.field {
	display: grid;
	gap: 0.45rem;
	font-size: 0.92rem;
	color: #45627a;
}

.field span {
	font-weight: 700;
}

.field-inline {
	flex: 1;
}

.field-wide {
	grid-column: 1 / -1;
}

.field-toggle {
	align-content: end;
}

.field-toggle input {
	inline-size: 1.1rem;
	block-size: 1.1rem;
}

.field-readonly .readonly-card {
	min-height: 2.9rem;
	display: flex;
	align-items: center;
	padding: 0.8rem 0.95rem;
	border-radius: 14px;
	background: #f6f9fc;
	border: 1px solid #d7e4f0;
	color: #37536c;
	font-weight: 700;
}

input,
select {
	inline-size: 100%;
	padding: 0.78rem 0.92rem;
	border-radius: 14px;
	border: 1px solid #ccd9e6;
	background: #fbfdff;
	font: inherit;
	color: #18344e;
}

input:focus,
select:focus {
	outline: none;
	border-color: #4b93d0;
	box-shadow: 0 0 0 4px rgba(75, 147, 208, 0.14);
}

input[readonly] {
	background: #f4f8fb;
	color: #627d94;
}

.detail-stack {
	display: grid;
	gap: 1rem;
}

.accordion-panel {
	border: 1px solid #dbe7f3;
	border-radius: 18px;
	background: #f8fbfd;
}

.accordion-panel summary {
	display: flex;
	align-items: center;
	justify-content: space-between;
	padding: 1rem 1.05rem;
	cursor: pointer;
	font-weight: 700;
	color: #18344e;
	list-style: none;
}

.accordion-panel summary::after {
	content: "+";
	flex: 0 0 auto;
	font-size: 1.1rem;
	font-weight: 700;
	color: #678198;
}

.accordion-panel[open] summary::after {
	content: "-";
}

.accordion-panel summary::-webkit-details-marker {
	display: none;
}

.accordion-body {
	padding: 0 1.05rem 1.05rem;
}

.profile-service-list,
.stack-editor {
	display: grid;
	gap: 0.8rem;
}

.profile-service-list {
	grid-template-columns: repeat(auto-fit, minmax(14rem, 1fr));
	align-content: start;
}

.profile-service-row {
	display: flex;
	align-items: center;
	gap: 1rem;
	padding: 0.85rem 0.95rem;
	border-radius: 16px;
	border: 1px solid #d7e3ef;
	background: #ffffff;
}

.profile-service-row > div {
	flex: 1;
}

.profile-service-row input[type="checkbox"] {
	flex: 0 0 auto;
	inline-size: 1.1rem;
	block-size: 1.1rem;
	margin: 0;
}

.profile-service-row small {
	display: block;
	margin: 0.2rem 0 0;
	color: #5f778d;
}

.stack-editor {
	max-height: 24rem;
	overflow-y: auto;
	padding-right: 0.3rem;
}

.editor-row {
	padding: 0.35rem 0;
}

.editor-row-wide {
	display: grid;
	grid-template-columns: 1.4fr 1fr 1fr auto;
}

.editor-row-quad {
	display: grid;
	grid-template-columns: 1.1fr 0.75fr 0.9fr 1.3fr auto;
}

.empty-state {
	padding: 1rem;
	border-radius: 16px;
	border: 1px dashed #cad7e4;
	background: #f8fbfd;
	color: #5b758c;
	line-height: 1.6;
}

@media (max-width: 1180px) {
	.workspace-tabs,
	.overview-shell,
	.summary-grid,
	.runtime-simple-grid,
	.runtime-buffer-grid,
	.field-grid,
	.profile-service-list {
		grid-template-columns: 1fr;
	}

	.overview-notes {
		grid-template-columns: 1fr;
	}

	.editor-panel-header,
	.panel-subheader {
		flex-direction: column;
		align-items: stretch;
	}

	.editor-panel-actions {
		justify-content: stretch;
	}

	.selector-control {
		max-width: none;
		min-width: 0;
	}

	.runtime-pressure-row {
		grid-template-columns: 1fr;
		gap: 0.75rem;
	}

	.runtime-pressure-row__label {
		padding-top: 0;
	}

	.runtime-pressure-row__inputs {
		grid-template-columns: 1fr 1fr;
	}
}

@media (max-width: 720px) {
	.admin-shell {
		padding: 1rem;
	}

	.editor-row-wide,
	.editor-row-quad {
		grid-template-columns: 1fr;
	}
}
</style>
