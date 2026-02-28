<!-- Name: Dashboard.vue
Description: The component for creating the dashboard
Programmers: Adam Berry 
Creation Date: 2/14
Revision Dates: Adam Berry 2/14, Adam Berry 2/15
Preconditions: Not Relevant
Postconditions: Not Relevant
Error Types: Not Relevant
Invariants: Dependencies described in /Docs/web.md
Known Faults: None
-->



<script setup lang="ts">
import { ref, onBeforeUnmount, onMounted } from "vue";
import Card from "primevue/card";
import DonutGraph from "./DonutGraph.vue";

interface SystemStatus {
	api: string;
	ingestion: string;
	ml: string;
}

interface TempEventData {
	id?: number;
	timestamp: string;
	sensor_type?: string;
	sensor_number?: number;
	fan_on?: boolean;
	temperature?: number;
	heater_power?: number;
	anomalies?: string[];
}

interface StreamMessage {
	kind: string;
	event_type?: string;
	source: string;
	timestamp: string;
	data: unknown;
}

interface StreamBatch {
	kind: string;
	messages: StreamMessage[];
}

const loading = ref(true);
const recentEvents = ref<TempEventData[]>([]);
const mlAlerts = ref<string[]>([]);
const streamState = ref("Connecting...");
let socket: WebSocket | null = null;

const status = ref<SystemStatus>({
	api: "Online",
	ingestion: "Connecting...",
	ml: "Waiting..."
});

const metrics = ref({
	totalRecords: 0,
	activeAlerts: 0,
	lastIngest: "Waiting for stream"
});

function getStreamURL(): string {
	const envURL = import.meta.env.VITE_INGEST_WS_URL as string | undefined;
	if (envURL && envURL.trim() !== "") {
		return envURL;
	}

	const protocol = window.location.protocol === "https:" ? "wss" : "ws";
	const host = window.location.hostname || "127.0.0.1";
	return `${protocol}://${host}:8080/ws`;
}

function updateActiveAlerts(): void {
	const ruleAlerts = recentEvents.value.reduce((count, event) => {
		return count + (event.anomalies?.length ?? 0);
	}, 0);
	metrics.value.activeAlerts = ruleAlerts + mlAlerts.value.length;
}

function pushRecentEvent(event: TempEventData): void {
	recentEvents.value.unshift(event);
	recentEvents.value = recentEvents.value.slice(0, 8);
	metrics.value.totalRecords += 1;
	metrics.value.lastIngest = new Date(event.timestamp).toLocaleString();
	updateActiveAlerts();
}

function pushMLAlert(message: unknown): void {
	mlAlerts.value.unshift(JSON.stringify(message));
	mlAlerts.value = mlAlerts.value.slice(0, 8);
	status.value.ml = "Receiving";
	updateActiveAlerts();
}

function handleStreamMessage(raw: string): void {
	let batch: StreamBatch;
	try {
		batch = JSON.parse(raw) as StreamBatch;
	} catch (err) {
		console.error("Failed to parse websocket payload", err);
		return;
	}

	if (batch.kind !== "batch" || !Array.isArray(batch.messages)) {
		return;
	}

	for (const message of batch.messages) {
		if (message.kind === "event" && message.event_type === "temperature") {
			pushRecentEvent(message.data as TempEventData);
			continue;
		}
		if (message.kind === "ml_result") {
			pushMLAlert(message.data);
		}
	}
}

onMounted(async () => {
	socket = new WebSocket(getStreamURL());

	socket.addEventListener("open", () => {
		streamState.value = "Connected";
		status.value.ingestion = "Streaming";
		status.value.ml = "Listening";
		loading.value = false;
	});

	socket.addEventListener("message", (event) => {
		handleStreamMessage(String(event.data));
	});

	socket.addEventListener("close", () => {
		streamState.value = "Disconnected";
		status.value.ingestion = "Disconnected";
		loading.value = false;
	});

	socket.addEventListener("error", (err) => {
		console.error("Websocket stream error", err);
		streamState.value = "Error";
		status.value.ingestion = "Stream Error";
		loading.value = false;
	});
});

onBeforeUnmount(() => {
	socket?.close();
	socket = null;
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
						<li><strong>Websocket:</strong> {{ streamState }}</li>
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

			<Card class="card">
				<template #title>Recent Events</template>
				<template #content>
					<ul class="metrics-list">
						<li v-if="recentEvents.length === 0">No events yet</li>
						<li
							v-for="event in recentEvents"
							:key="event.timestamp"
						>
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
							{{ alert }}
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
