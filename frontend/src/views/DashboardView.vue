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
import { ref, onBeforeUnmount, onMounted } from "vue";
import Card from "primevue/card";
import {
	close as closeSocket,
	connect as connectSocket,
	getDefaultStreamURL,
	onBatch,
	send as sendSocket,
	type ManagedWebSocket,
	type StreamBatch,
	type StreamMessage
} from "@/utils/wsHelper";

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

interface ValveEventData {
	id?: number;
	timestamp: string;
	sensor_type?: string;
	valve_number?: number;
	is_open?: boolean;
	flow_rate?: number;
	anomalies?: string[];
}

interface MLAnomalyPayload {
	schema: string;
	event_type: string;
	generated_at: string;
	has_anomaly: boolean;
	labels: string[];
	score?: number;
	raw_response: unknown;
}

const loading = ref(true);
const recentEvents = ref<TempEventData[]>([]);
const recentValveEvents = ref<ValveEventData[]>([]);
const mlAlerts = ref<MLAnomalyPayload[]>([]);
const streamState = ref("Connecting...");
const faultStatus = ref("Idle");
let socket: ManagedWebSocket | null = null;

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
	console.log("Stream event received", event);
	updateActiveAlerts();
}

function pushMLAlert(message: MLAnomalyPayload): void {
	mlAlerts.value.unshift(message);
	mlAlerts.value = mlAlerts.value.slice(0, 8);
	status.value.ml = "Receiving";
	console.log("ML result received", message);
	updateActiveAlerts();
}

function pushValveEvent(event: ValveEventData): void {
	recentValveEvents.value.unshift(event);
	recentValveEvents.value = recentValveEvents.value.slice(0, 8);
	metrics.value.totalRecords += 1;
	metrics.value.lastIngest = new Date(event.timestamp).toLocaleString();
	console.log("Valve stream event received", event);
	updateActiveAlerts();
}

function injectSimulatorFault(): void {
	const sent = sendSocket(socket, {
		kind: "inject_fault",
		source: "frontend",
		timestamp: new Date().toISOString(),
		data: {
			source_name: "temp_dev"
		}
	});
	faultStatus.value = sent ? "Injected" : "Failed to send";
}

function handleStreamMessage(batch: StreamBatch): void {
	if (batch.kind !== "batch" || !Array.isArray(batch.messages)) {
		return;
	}

	console.log("Websocket batch received", batch);

	for (const message of batch.messages) {
		if (message.kind === "event") {
			if (message.event_type === "temperature") {
				pushRecentEvent(message.data as TempEventData);
				continue;
			}
			if (message.event_type === "valve") {
				pushValveEvent(message.data as ValveEventData);
			}
			continue;
		}
		if (message.kind === "ml_result") {
			pushMLAlert(message.data as MLAnomalyPayload);
		}
	}
}

onMounted(async () => {
	const streamURL = getDefaultStreamURL();
	console.log("Attempting websocket connection", streamURL);
	socket = connectSocket(streamURL);

	socket.addEventListener("open", () => {
		console.log("Websocket connected", streamURL);
		streamState.value = "Connected";
		status.value.ingestion = "Streaming";
		status.value.ml = "Listening";
		loading.value = false;
	});

	onBatch(socket, (batch: StreamBatch) => {
		handleStreamMessage(batch);
	});

	socket.addEventListener("close", () => {
		console.log("Websocket closed", streamURL);
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
	closeSocket(socket);
	socket = null;
});
</script>

<template>
	<main>
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
							<li>
								<strong>Fault Injector:</strong>
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
								{{ JSON.stringify(alert) }}
							</li>
						</ul>
					</template>
				</Card>

				<Card class="card">
					<template #title>Valve Events</template>
					<template #content>
						<ul class="metrics-list">
							<li v-if="recentValveEvents.length === 0">No valve events yet</li>
							<li
								v-for="event in recentValveEvents"
								:key="event.timestamp"
							>
								{{ new Date(event.timestamp).toLocaleTimeString() }}
								flow {{ event.flow_rate ?? "n/a" }}
								<span v-if="event.anomalies && event.anomalies.length > 0">
									alerts {{ event.anomalies.join(", ") }}
								</span>
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
