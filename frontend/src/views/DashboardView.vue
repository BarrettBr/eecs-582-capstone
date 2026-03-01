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
import { onMounted } from "vue";
import { storeToRefs } from "pinia";
import { useSystemStore } from "@/stores/system";
import Chart from "primevue/chart";
import Card from "primevue/card";

const store = useSystemStore();

// This keeps the variable names identical to your current dashboard template
const {
	loading,
	recentEvents,
	recentValveEvents,
	mlAlerts,
	streamState,
	faultStatus,
	status,
	metrics,
} = storeToRefs(store);

const { injectSimulatorFault } = store;

onMounted(() => {
	store.startStream();
});
</script>

<template>
	<main>
		<div class="dashboard">
			<h1 class="dashboard-title">System Dashboard</h1>

			<div v-if="loading" class="loading">Loading system data...</div>

			<div v-else class="grid">
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
					<template #title>System Status</template>
					<template #content>
						<ul class="status-list">
							<li><strong>API:</strong> {{ status.api }}</li>
							<li>
								<strong>Ingestion:</strong>
								{{ status.ingestion }}
							</li>
							<li><strong>ML Service:</strong> {{ status.ml }}</li>
							<li><strong>Websocket:</strong> {{ streamState }}</li>
						</ul>
					</template>
				</Card>
				<Card class="card">
					<template #title>Valve Events</template>
					<template #content>
						<ul class="metrics-list">
							<li v-if="recentValveEvents.length === 0">No valve events yet</li>
							<li v-for="event in recentValveEvents" :key="event.timestamp">
								{{ new Date(event.timestamp).toLocaleTimeString() }}
								flow {{ event.flow_rate ?? "n/a" }}
								<span v-if="event.anomalies && event.anomalies.length > 0">
									alerts {{ event.anomalies.join(", ") }}
								</span>
							</li>
						</ul>
					</template>
				</Card>

				<Card class="card">
					<template #title>Metrics</template>
					<template #content>
						<ul class="metrics-list">
							<li>
								<strong>Total Records:</strong>
								{{ metrics.totalRecords }}
							</li>
							<li>
								<strong>Active Alerts:</strong>
								{{ metrics.activeAlerts }}
							</li>
							<li>
								<strong>Last Ingest:</strong>
								{{ metrics.lastIngest }}
							</li>
							<li>
								<button type="button" @click="injectSimulatorFault">
									Inject Fault
								</button>
								{{ faultStatus }}
							</li>
						</ul>
					</template>
				</Card>
				<Card class="card">
					<template #title>Recent Events</template>
					<template #content>
						<ul class="metrics-list">
							<li v-if="recentEvents.length === 0">No events yet</li>
							<li v-for="event in recentEvents" :key="event.timestamp">
								{{ new Date(event.timestamp).toLocaleTimeString() }}
								temperature {{ event.temperature ?? "n/a" }}
								<span v-if="event.anomalies && event.anomalies.length > 0">
									alerts {{ event.anomalies.join(", ") }}
								</span>
							</li>
						</ul>
					</template>
				</Card>

				<Card class="card">
					<template #title>ML Alerts</template>
					<template #content>
						<ul class="metrics-list">
							<li v-if="mlAlerts.length === 0">No ML alerts yet</li>
							<li v-for="(alert, index) in mlAlerts" :key="index">
								{{ JSON.stringify(alert) }}
							</li>
						</ul>
					</template>
				</Card>
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

.grid {
	display: grid;
	grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
	gap: 1.5rem;
}

.card {
	padding: 1rem;
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
</style>
