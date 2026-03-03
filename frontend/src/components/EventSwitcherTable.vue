<script setup lang="ts">
import { ref, computed } from "vue";
import DataTable from "primevue/datatable";
import Column from "primevue/column";
import Tag from "primevue/tag";
import SelectButton from "primevue/selectbutton";
import Card from "primevue/card";

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
	recentEvents: TempEvent[];
	mlAlerts: MLAlert[];
}>();

const selectedView = ref("events");

const options = [
	{ label: "Recent Events", value: "events" },
	{ label: "ML Alerts", value: "ml" },
];

const currentData = computed(() => {
	return selectedView.value === "events" ? props.recentEvents : props.mlAlerts;
});

const recentEventRows = computed(() => {
	return props.recentEvents.slice(0, 5);
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
			<!-- Temperature Events -->
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

			<!-- ML Alerts -->
			<DataTable
				v-else
				:value="currentData"
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
</style>
