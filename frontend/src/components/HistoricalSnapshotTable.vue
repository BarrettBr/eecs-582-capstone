<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import DataTable from "primevue/datatable";
import Column from "primevue/column";
import Tag from "primevue/tag";
import SelectButton from "primevue/selectbutton";
import Card from "primevue/card";
import Button from "primevue/button";

interface TempEvent {
	timestamp: string;
	display_time?: string;
	sensor_number?: number;
	temperature?: number;
	fan_on?: boolean;
	anomalies?: string[];
}

interface MLAlert {
	generated_at: string;
	display_time?: string;
	has_anomaly: boolean;
	labels: string[];
	score?: number;
}

const props = defineProps<{
	getEventHistorySnapshot: () => TempEvent[];
	getMLAlertHistorySnapshot: () => MLAlert[];
}>();

const selectedView = ref("events");
const snapshotEvents = ref<TempEvent[]>([]);
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

// description: Captures a frozen copy of the current event and alert lists.
// input: Reads the latest props passed in from the dashboard.
// output: Updates the snapshot tables and the snapshot timestamp label.
function refreshSnapshot(): void {
	snapshotEvents.value = props.getEventHistorySnapshot();
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
					<div class="snapshot-meta">Snapshot taken at {{ snapshotTakenAt }}</div>
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

				<Column field="sensor_number" header="Sensor" sortable />

				<Column field="temperature" header="Temperature" sortable>
					<template #body="{ data }">
						{{ data.temperature ?? "n/a" }} °C
					</template>
				</Column>

				<Column header="Fan">
					<template #body="{ data }">
						<Tag
							:value="data.fan_on ? 'ON' : 'OFF'"
							:severity="data.fan_on ? 'success' : 'secondary'"
						/>
					</template>
				</Column>

				<Column header="Alerts">
					<template #body="{ data }">
						<Tag
							v-if="data.anomalies?.length"
							:value="data.anomalies.join(', ')"
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
						{{ data.labels?.join(', ') || '-' }}
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
