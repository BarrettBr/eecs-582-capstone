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
import { onMounted, ref } from "vue";
import { storeToRefs } from "pinia";
import { useSystemStore } from "@/stores/system";
import Chart from "primevue/chart";
import Card from "primevue/card";

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
} = storeToRefs(store);

const { injectSimulatorFault } = store;

onMounted(() => {
	store.startStream();
});

const chartOptions = ref({
    animation: {
        duration: 0 // Set to 0 to remove the "flash/jump" entirely
    },
    hover: {
        mode: 'nearest',
        intersect: true
    },
    scales: {
        y: {
            beginAtZero: false
        }
    },
    responsive: true,
    maintainAspectRatio: false
});
</script>

<template>
	<main>
		<div class="dashboard">
			<div v-if="loading" class="loading">Loading system data...</div>

			<div v-else class="grid">
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
