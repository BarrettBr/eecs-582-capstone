<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import DataTable from "primevue/datatable";
import Column from "primevue/column";
import Tag from "primevue/tag";
import SelectButton from "primevue/selectbutton";
import Card from "primevue/card";
import Button from "primevue/button";

interface EventSnapshotRow {
	service_name?: string;
	timestamp: string;
	timestamp_ms?: number;
	display_time?: string;
	sensor_number?: number;
	temperature?: number;
	fan_on?: boolean;
	valve_number?: number;
	flow_rate?: number;
	is_open?: boolean;
	anomalies?: string[];
	event_type?: "temperature" | "valve";
}

interface MLAlert {
	service_name?: string;
	generated_at: string;
	display_time?: string;
	has_anomaly: boolean;
	labels: string[];
	score?: number;
}

interface HistoricalEventRow {
	timestamp: string;
	timestamp_ms?: number;
	display_time?: string;
	service_name?: string;
	source_label: string;
	reading_label: string;
	state_label: string;
	state_severity: "success" | "secondary";
	alerts: string[];
}

const props = defineProps<{
	getEventHistorySnapshot: () => EventSnapshotRow[];
	getMLAlertHistorySnapshot: () => MLAlert[];
	serviceDisplayNames: Record<string, string>;
}>();

const selectedView = ref("events");
const snapshotEvents = ref<HistoricalEventRow[]>([]);
const snapshotAlerts = ref<MLAlert[]>([]);
const snapshotTakenAt = ref("No snapshot yet");

const options = [
	{ label: "Recent Events", value: "events" },
	{ label: "ML Alerts", value: "ml" },
];

const currentRows = computed(() => {
	return selectedView.value === "events"
		? snapshotEvents.value
		: snapshotAlerts.value;
});

function displayServiceName(serviceName?: string): string {
	if (!serviceName) {
		return "-";
	}
	return props.serviceDisplayNames[serviceName] ?? serviceName;
}

function normalizeHistoricalEventRows(
	events: EventSnapshotRow[],
): HistoricalEventRow[] {
	return events.map((event) => {
		if (event.event_type === "valve") {
			return {
				timestamp: event.timestamp,
				timestamp_ms: event.timestamp_ms,
				display_time: event.display_time,
				service_name: event.service_name,
				source_label: `Valve ${event.valve_number ?? "-"}`,
				reading_label: `${event.flow_rate ?? "n/a"} L/m`,
				state_label: event.is_open ? "Open" : "Closed",
				state_severity: event.is_open ? "success" : "secondary",
				alerts: event.anomalies ?? [],
			};
		}

		return {
			timestamp: event.timestamp,
			timestamp_ms: event.timestamp_ms,
			display_time: event.display_time,
			service_name: event.service_name,
			source_label: `Sensor ${event.sensor_number ?? "-"}`,
			reading_label: `${event.temperature ?? "n/a"} °C`,
			state_label: event.fan_on ? "Fan On" : "Fan Off",
			state_severity: event.fan_on ? "success" : "secondary",
			alerts: event.anomalies ?? [],
		};
	});
}

function refreshSnapshot(): void {
	snapshotEvents.value = normalizeHistoricalEventRows(
		props.getEventHistorySnapshot(),
	);
	snapshotAlerts.value = props.getMLAlertHistorySnapshot();
	snapshotTakenAt.value = new Date().toLocaleTimeString();
}

onMounted(() => {
	refreshSnapshot();
});
</script>

<template>
	<Card class="snapshot-card">
		<template #title>
			<div class="snapshot-header">
				<div>
					<div class="snapshot-title">Historical Event Log</div>
					<div class="snapshot-meta">
						Snapshot taken at {{ snapshotTakenAt }}
					</div>
				</div>
				<div class="snapshot-controls">
					<SelectButton
						v-model="selectedView"
						:options="options"
						optionLabel="label"
						optionValue="value"
					/>
					<Button
						label="Refresh"
						icon="pi pi-refresh"
						size="small"
						severity="secondary"
						outlined
						@click="refreshSnapshot"
					/>
				</div>
			</div>
		</template>

		<template #content>
			<DataTable
				v-if="selectedView === 'events'"
				:value="currentRows"
				paginator
				:rows="10"
				stripedRows
				size="small"
				sortField="timestamp"
				:sortOrder="-1"
			>
				<Column field="timestamp" header="Time" sortable>
					<template #body="{ data }">
						{{ data.display_time ?? data.timestamp }}
					</template>
				</Column>

				<Column field="service_name" header="Service" sortable>
					<template #body="{ data }">
						{{ displayServiceName(data.service_name) }}
					</template>
				</Column>

				<Column field="source_label" header="Source" />

				<Column field="reading_label" header="Reading" />

				<Column header="State">
					<template #body="{ data }">
						<Tag
							:value="data.state_label"
							:severity="data.state_severity"
						/>
					</template>
				</Column>

				<Column header="Alerts">
					<template #body="{ data }">
						<Tag
							v-if="data.alerts.length"
							:value="data.alerts.join(', ')"
							severity="danger"
						/>
						<span v-else>-</span>
					</template>
				</Column>
			</DataTable>

			<DataTable
				v-else
				:value="currentRows"
				paginator
				:rows="10"
				stripedRows
				size="small"
				sortField="generated_at"
				:sortOrder="-1"
			>
				<Column field="generated_at" header="Generated" sortable>
					<template #body="{ data }">
						{{ data.display_time ?? data.generated_at }}
					</template>
				</Column>

				<Column field="service_name" header="Service" sortable>
					<template #body="{ data }">
						{{ displayServiceName(data.service_name) }}
					</template>
				</Column>

				<Column header="Anomaly">
					<template #body="{ data }">
						<Tag
							:value="data.has_anomaly ? 'YES' : 'NO'"
							:severity="data.has_anomaly ? 'danger' : 'success'"
						/>
					</template>
				</Column>

				<Column header="Labels">
					<template #body="{ data }">
						{{ data.labels?.join(", ") || "-" }}
					</template>
				</Column>

				<Column field="score" header="Score" sortable />
			</DataTable>
		</template>
	</Card>
</template>

<style scoped>
.snapshot-card {
	padding: 1rem;
	grid-column: span 12;
}

.snapshot-header {
	display: flex;
	justify-content: space-between;
	align-items: flex-start;
	gap: 1rem;
}

.snapshot-title {
	font-weight: 600;
}

.snapshot-meta {
	margin-top: 0.35rem;
	font-size: 0.85rem;
	color: var(--text-color-secondary);
}

.snapshot-controls {
	display: flex;
	flex-wrap: wrap;
	justify-content: flex-end;
	gap: 0.75rem;
}

@media (min-width: 768px) {
	.snapshot-card {
		grid-column: span 12;
	}
}

@media (max-width: 767px) {
	.snapshot-header {
		flex-direction: column;
	}

	.snapshot-controls {
		width: 100%;
		justify-content: space-between;
	}
}
</style>
