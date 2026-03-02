<!-- Name: Dashboard.vue
Description: The component for creating the dashboard
Programmers: Adam Berry 
Creation Date: 2/14
Revision Dates: Adam Berry 2/14, Adam Berry 2/15, Barret Brown 3/1, Adam Berry 3/1
Preconditions: Not Relevant
Postconditions: Not Relevant
Error Types: Not Relevant
Invariants: Dependencies described in /Docs/web.md
Known Faults: None
-->
<script setup lang="ts">
import { computed, onMounted, watch } from "vue";
import { storeToRefs } from "pinia";
import { useSystemStore } from "@/stores/system";
import Chart from "primevue/chart";
import Card from "primevue/card";
import Tag from "primevue/tag";
import Button from "primevue/button";
import EventSwitcherTable from "@/components/EventSwitcherTable.vue";
import HistoricalSnapshotTable from "@/components/HistoricalSnapshotTable.vue";

const store = useSystemStore();

// load the variable names in the expected way
const {
	loading,
	recentEvents,
	recentValveEvents,
	mlAlerts,
	streamState,
	faultStatus,
	status,
	metrics,
	temperatureChartBounds,
} = storeToRefs(store);

const { injectSimulatorFault } = store;

onMounted(() => {
	store.startStream();
});

watch(faultStatus, (newValue) => {
  if (newValue === "Injected") {
    setTimeout(() => {
      store.faultStatus = "Idle";
    }, 2000);
  }
});

const chartOptions = computed(() => {
	const yScale: {
		beginAtZero: boolean;
		min?: number;
		max?: number;
	} = {
		beginAtZero: false,
	};

	if (
		typeof temperatureChartBounds.value.min === "number" &&
		typeof temperatureChartBounds.value.max === "number"
	) {
		yScale.min = temperatureChartBounds.value.min;
		yScale.max = temperatureChartBounds.value.max;
	}

	return {
		animation: {
			duration: 0,
		},
		hover: {
			mode: "nearest",
			intersect: true,
		},
		scales: {
			y: yScale,
		},
		responsive: true,
		maintainAspectRatio: false,
	};
});

// description: Maps the websocket state into a tag severity.
// input: The current websocket status label.
// output: Returns the severity string used by the status badge.
function getSocketSeverity(state: string) {
  switch (state) {
    case "Connected":
      return "success";
    case "Connecting...":
      return "warning";
    case "Error":
    case "Disconnected":
      return "danger";
    default:
      return "info";
  }
}

</script>

<template>
	<main>
		<div class="dashboard">
			<div v-if="loading" class="loading">Loading system data...</div>

			<div v-else class="grid">
				<Card class="card">
					<template #title>System Status</template>
					<template #content>
						<div class="status-grid">

							<div class="status-item">
								<span class="status-label">API</span>
								<Tag
										:value="status.api"
										:severity="status.api === 'Online' ? 'success' : 'danger'"
										/>
							</div>

							<div class="status-item">
								<span class="status-label">Ingestion</span>
								<Tag
										:value="status.ingestion"
										:severity="status.ingestion === 'Streaming' ? 'success' : 'danger'"
										/>
							</div>

							<div class="status-item">
								<span class="status-label">ML Service</span>
								<Tag
										:value="status.ml"
										:severity="status.ml === 'Receiving' ? 'success' : 'danger'"
										/>
							</div>

							<div class="status-item">
								<span class="status-label">Websocket</span>
								<Tag
										:value="streamState"
										:severity="getSocketSeverity(streamState)"
										/>
							</div>

						</div>
					</template>
				</Card>

				<Card class="card col-span-2">
					<template #title>Live Temperature</template>
					<template #content>
						<Chart
							type="line"
							:data="store.temperatureChartData"
							:options="chartOptions"
							style="height: 300px"
						/>
					</template>
				</Card>
				<Card class="card">
				<template #title>Metrics</template>

				<template #content>
					<div class="metrics-grid">

						<div class="metric-item">
							<span class="metric-label">Total Records</span>
							<span class="metric-value">
								{{ metrics.totalRecords }}
							</span>
						</div>

						<div class="metric-item">
							<span class="metric-label">Active Alerts</span>
							<Tag
									:value="metrics.activeAlerts"
									:severity="metrics.activeAlerts > 0 ? 'danger' : 'success'"
									/>
						</div>

						<div class="metric-item">
							<span class="metric-label">Last Ingest</span>
							<span class="metric-subtle">
								{{ metrics.lastIngest }}
							</span>
						</div>

						<div class="metric-actions">
							<Button
									label="Inject Fault"
									icon="pi pi-exclamation-triangle"
									severity="warning"
									@click="injectSimulatorFault"
									/>
								<Tag
										:value="faultStatus"
										:severity="faultStatus === 'Injected' ? 'danger' : 'info'"
										/>
						</div>

					</div>
				</template>
				</Card>
				<EventSwitcherTable
						:recentEvents="recentEvents"
						:mlAlerts="mlAlerts"
						/>
				<HistoricalSnapshotTable
						:getEventHistorySnapshot="store.getHistoricalEventSnapshot"
						:getMLAlertHistorySnapshot="store.getHistoricalMLAlertSnapshot"
						/>
			</div>
		</div>
	</main>
</template>

<style scoped>
.dashboard {
	padding: 2rem;
}

.dashboard-title {
	margin-bottom: 1.5rem;
	color: black;
}
/* Update your existing grid class */
.grid {
    display: grid;
    grid-template-columns: repeat(12, 1fr); /* Switch to a 12-column base */
    gap: 1.5rem;
}

/* Define how the cards behave by default */
.card {
    padding: 1rem;
    grid-column: span 12; /* On mobile/small screens, they take full width */
}

/* Define the spans for larger screens */
@media (min-width: 768px) {
    .card {
        grid-column: span 4; /* Standard cards take 4/12 (3 columns total) */
    }
    
    .col-span-2 {
        grid-column: span 8 !important; /* Spanning cards take 8/12 (2/3 of the row) */
    }

    .col-span-full {
        grid-column: span 12 !important;
    }
}

.status-list,
.metrics-list {
	list-style: none;
	padding: 0;
	margin: 0;
}

.status-list li,
.metrics-list li {
	margin-bottom: 0.5rem;
}

.loading {
	font-style: italic;
}

.metrics-grid {
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
}

.metric-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.metric-label {
  font-weight: 500;
  color: var(--text-color-secondary);
}

.metric-value {
  font-size: 1.5rem;
  font-weight: 600;
}

.metric-subtle {
  font-size: 0.9rem;
  color: var(--text-color-secondary);
}

.metric-actions {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.status-grid {
  display: flex;
  flex-direction: column;
  gap: 1.25rem;
}

.status-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.status-label {
  font-weight: 500;
  color: var(--text-color-secondary);
}
</style>
