<script setup lang="ts">
import { computed, ref } from "vue";
import DataTable from "primevue/datatable";
import Column from "primevue/column";
import Tag from "primevue/tag";
import SelectButton from "primevue/selectbutton";
import Card from "primevue/card";

interface TempEvent {
	service_name?: string;
	timestamp: string;
	timestamp_ms?: number;
	display_time?: string;
	sensor_number?: number;
	temperature?: number;
	fan_on?: boolean;
	anomalies?: string[];
}

interface ValveEvent {
	service_name?: string;
	timestamp: string;
	timestamp_ms?: number;
	display_time?: string;
	valve_number?: number;
	flow_rate?: number;
	is_open?: boolean;
	anomalies?: string[];
}

interface MLAlert {
	service_name?: string;
	generated_at: string;
	display_time?: string;
	has_anomaly: boolean;
	labels: string[];
	score?: number;
}

interface EventRow {
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
	recentEvents: TempEvent[];
	recentValveEvents: ValveEvent[];
	mlAlerts: MLAlert[];
	serviceDisplayNames: Record<string, string>;
}>();

const selectedView = ref("events");

const options = [
	{ label: "Recent Events", value: "events" },
	{ label: "ML Alerts", value: "ml" },
];

function displayServiceName(serviceName?: string): string {
	if (!serviceName) {
		return "-";
	}
	return props.serviceDisplayNames[serviceName] ?? serviceName;
}

const recentEventRows = computed<EventRow[]>(() => {
	const temperatureRows: EventRow[] = props.recentEvents.map((event) => ({
		timestamp: event.timestamp,
		timestamp_ms: event.timestamp_ms,
		display_time: event.display_time,
		service_name: event.service_name,
		source_label: `Sensor ${event.sensor_number ?? "-"}`,
		reading_label: `${event.temperature ?? "n/a"} °C`,
		state_label: event.fan_on ? "Fan On" : "Fan Off",
		state_severity: event.fan_on ? "success" : "secondary",
		alerts: event.anomalies ?? [],
	}));
	const valveRows: EventRow[] = props.recentValveEvents.map((event) => ({
		timestamp: event.timestamp,
		timestamp_ms: event.timestamp_ms,
		display_time: event.display_time,
		service_name: event.service_name,
		source_label: `Valve ${event.valve_number ?? "-"}`,
		reading_label: `${event.flow_rate ?? "n/a"} L/m`,
		state_label: event.is_open ? "Open" : "Closed",
		state_severity: event.is_open ? "success" : "secondary",
		alerts: event.anomalies ?? [],
	}));

	return temperatureRows
		.concat(valveRows)
		.sort((left, right) => (right.timestamp_ms ?? 0) - (left.timestamp_ms ?? 0))
		.slice(0, 5);
});
</script>

<template>
	<Card class="card col-span-2">
		<template #title>
			<div class="switch-header">
				<span>Event Stream</span>
				<SelectButton
					v-model="selectedView"
					:options="options"
					optionLabel="label"
					optionValue="value"
				/>
			</div>
		</template>

		<template #content>
			<DataTable
				v-if="selectedView === 'events'"
				:value="recentEventRows"
				stripedRows
				size="small"
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
						<div class="alerts-cell">
							<Tag
								v-if="data.alerts.length"
								:value="data.alerts.join(', ')"
								severity="danger"
							/>
							<span v-else class="alerts-empty">-</span>
						</div>
					</template>
				</Column>
			</DataTable>

			<DataTable
				v-else
				:value="props.mlAlerts"
				paginator
				:rows="5"
				stripedRows
				size="small"
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
.switch-header {
	display: flex;
	justify-content: space-between;
	align-items: center;
}

.alerts-cell {
	min-width: 10rem;
}

.alerts-empty {
	display: inline-block;
	min-width: 1ch;
}
</style>
