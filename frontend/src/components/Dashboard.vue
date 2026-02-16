<script setup lang="ts">
import { ref, onMounted } from "vue";
import Button from "primevue/button";
import Card from "primevue/card";
import DonutGraph from "./DonutGraph.vue";

interface SystemStatus {
	api: string;
	ingestion: string;
	ml: string;
}

const loading = ref(true);

const status = ref<SystemStatus>({
	api: "Checking...",
	ingestion: "Checking...",
	ml: "Checking..."
});

const metrics = ref({
	totalRecords: 0,
	activeAlerts: 0,
	lastIngest: "—"
});

onMounted(async () => {
	try {
		// TODO: Replace with real API calls
		// const res = await fetch("/api/status")
		// const data = await res.json()

		// Mock for now
		setTimeout(() => {
			status.value = {
				api: "Online",
				ingestion: "Running",
				ml: "Available"
			};

			metrics.value = {
				totalRecords: 1243,
				activeAlerts: 2,
				lastIngest: new Date().toLocaleString()
			};

			loading.value = false;
		}, 800);
	} catch (err) {
		console.error("Failed to load dashboard data", err);
		loading.value = false;
	}
});
</script>

<template>
	<div class="dashboard">
		<h1 class="dashboard-title">System Dashboard</h1>

		<div v-if="loading" class="loading">Loading system data...</div>

		<div v-else class="grid">
			<!-- System Status -->
			<Card class="card">
				<template #title>System Status</template>
				<template #content>
					<ul class="status-list">
						<li><strong>API:</strong> {{ status.api }}</li>
						<li>
							<strong>Ingestion:</strong> {{ status.ingestion }}
						</li>
						<li><strong>ML Service:</strong> {{ status.ml }}</li>
					</ul>
				</template>
			</Card>

			<Card class="card">
				<template #content>
					<Donut-Graph > 
					</Donut-Graph > 
				</template>
			</Card>

			<!-- Metrics -->
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
					</ul>
				</template>
			</Card>
		</div>
	</div>
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
