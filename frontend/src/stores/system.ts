// src/stores/system.ts
import { ref, computed } from "vue"; // Removed lifecycle hooks
import { defineStore } from "pinia";
import {
	close as closeSocket,
	connect as connectSocket,
	getDefaultStreamURL,
	onBatch,
	send as sendSocket,
	type ManagedWebSocket,
	type StreamBatch,
	type StreamMessage,
} from "@/utils/wsHelper";

export const useSystemStore = defineStore("system", () => {
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
		ml: "Waiting...",
	});

	const metrics = ref({
		totalRecords: 0,
		activeAlerts: 0,
		lastIngest: "Waiting for stream",
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
				source_name: "temp_dev",
			},
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

	// CHANGED: Wrapped in a function instead of onMounted
	async function startStream() {
		if (socket) return; // Prevent duplicate connections

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
	}

	// CHANGED: Wrapped in a function instead of onBeforeUnmount
	function stopStream() {
		closeSocket(socket);
		socket = null;
	}

	// CHANGED: Added computed property for the charts
	const temperatureChartData = computed(() => {
		return {
			labels: recentEvents.value
				.map((e) => new Date(e.timestamp).toLocaleTimeString())
				.reverse(),
			datasets: [
				{
					label: "Temperature",
					data: recentEvents.value.map((e) => e.temperature).reverse(),
					fill: false,
					borderColor: "#10b981",
					tension: 0.4,
				},
			],
		};
	});

	// Must return everything that is used in components
	return {
		loading,
		recentEvents,
		recentValveEvents,
		mlAlerts,
		streamState,
		faultStatus,
		status,
		metrics,
		injectSimulatorFault,
		startStream,
		stopStream,
		temperatureChartData,
	};
});
