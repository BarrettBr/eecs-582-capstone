// Name: stores/system.ts
// Description: This file acts as a store for the state of the layout (sidebar closed etc)
// Programmers: Barrett Brown, Adam Berry
// Creation Date: 3/1
// Revision Dates: 
// Preconditions: None
// Postconditions: Not Relevant
// Error Types: Not Relevant
// Invariants: Dependencies described in /Docs/web.md
// Known Faults: None


import { ref } from "vue";
import { defineStore } from "pinia";
import {
	close as closeSocket,
	connect as connectSocket,
	getDefaultStreamURL,
	onBatch,
	send as sendSocket,
	type ManagedWebSocket,
	type StreamBatch,
} from "@/utils/wsHelper";

export const useSystemStore = defineStore("system", () => {
	const CHART_FLUSH_INTERVAL_MS = 250;

	interface SystemStatus {
		api: string;
		ingestion: string;
		ml: string;
	}

	interface TempEventData {
		id?: number;
		timestamp: string;
		display_time?: string;
		display_timestamp?: string;
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
		display_time?: string;
		display_timestamp?: string;
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
		display_time?: string;
		has_anomaly: boolean;
		labels: string[];
		score?: number;
		raw_response: unknown;
	}

	const loading = ref(true);
	const recentEvents = ref<TempEventData[]>([]);
	const chartEvents = ref<TempEventData[]>([]); 
	const recentValveEvents = ref<ValveEventData[]>([]);
	const mlAlerts = ref<MLAnomalyPayload[]>([]);
	const streamState = ref("Connecting...");
	const faultStatus = ref("Idle");
	let socket: ManagedWebSocket | null = null;
	let chartBuffer: TempEventData[] = [];
	let chartFlushTimer: ReturnType<typeof setTimeout> | null = null;

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

	const temperatureChartData = ref({
		labels: [] as string[],
		datasets: [
			{
				label: "Temperature",
				data: [] as Array<number | undefined>,
				fill: false,
				borderColor: "#10b981",
				tension: 0.4,
			},
		],
	});

	function updateActiveAlerts(): void {
		const ruleAlerts = recentEvents.value.reduce((count, event) => {
			return count + (event.anomalies?.length ?? 0);
		}, 0);
		metrics.value.activeAlerts = ruleAlerts + mlAlerts.value.length;
	}

	function formatDisplayTime(timestamp: string): string {
		return new Date(timestamp).toLocaleTimeString();
	}

	function formatDisplayTimestamp(timestamp: string): string {
		return new Date(timestamp).toLocaleString();
	}

	function normalizeTempEvent(event: TempEventData): TempEventData {
		return {
			...event,
			display_time: formatDisplayTime(event.timestamp),
			display_timestamp: formatDisplayTimestamp(event.timestamp),
		};
	}

	function normalizeValveEvent(event: ValveEventData): ValveEventData {
		return {
			...event,
			display_time: formatDisplayTime(event.timestamp),
			display_timestamp: formatDisplayTimestamp(event.timestamp),
		};
	}

	function normalizeMLAlert(message: MLAnomalyPayload): MLAnomalyPayload {
		return {
			...message,
			display_time: formatDisplayTime(message.generated_at),
		};
	}

	function flushChartUpdates(): void {
		const count = chartBuffer.length;
		const labels = new Array<string>(count);
		const temperatures = new Array<number | undefined>(count);

		for (let index = 0; index < count; index += 1) {
			const event = chartBuffer[count-1-index];
			labels[index] = event.display_time ?? formatDisplayTime(event.timestamp);
			temperatures[index] = event.temperature;
		}

		chartEvents.value = chartBuffer.slice();
		temperatureChartData.value = {
			labels,
			datasets: [
				{
					...temperatureChartData.value.datasets[0],
					data: temperatures,
				},
			],
		};
	}

	function scheduleChartFlush(): void {
		if (chartFlushTimer !== null) {
			return;
		}
		chartFlushTimer = setTimeout(() => {
			chartFlushTimer = null;
			flushChartUpdates();
		}, CHART_FLUSH_INTERVAL_MS);
	}

	function pushRecentEvent(event: TempEventData): void {
		const normalized = normalizeTempEvent(event);

        // Strictly keep 8 for the dashboard list
        recentEvents.value.unshift(normalized);
        // recentEvents.value = recentEvents.value.slice(0, 8);

        // Keep the latest chart samples buffered and flush chart updates on a timer.
        chartBuffer.unshift(normalized);
        chartBuffer = chartBuffer.slice(0, 50);
        scheduleChartFlush();

        metrics.value.totalRecords += 1;
        metrics.value.lastIngest = normalized.display_timestamp ?? formatDisplayTimestamp(normalized.timestamp);
        //console.log("Stream event received", event);
        updateActiveAlerts();
    }

	function pushMLAlert(message: MLAnomalyPayload): void {
		const normalized = normalizeMLAlert(message);
		mlAlerts.value.unshift(normalized);
		mlAlerts.value = mlAlerts.value.slice(0, 8);
		status.value.ml = "Receiving";
		//console.log("ML result received", message);
		updateActiveAlerts();
	}

	function pushValveEvent(event: ValveEventData): void {
		const normalized = normalizeValveEvent(event);
		recentValveEvents.value.unshift(normalized);
		recentValveEvents.value = recentValveEvents.value.slice(0, 8);
		metrics.value.totalRecords += 1;
		metrics.value.lastIngest = normalized.display_timestamp ?? formatDisplayTimestamp(normalized.timestamp);
		//console.log("Valve stream event received", event);
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

		//console.log("Websocket batch received", batch);

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
			status.value.ml = "Offline";
			status.value.api = "Offline";

			loading.value = false;
		});

		socket.addEventListener("error", (err) => {
				console.error("Websocket stream error", err);

				streamState.value = "Error";
				status.value.ingestion = "Stream Error";
				status.value.ml = "Offline";
				status.value.api = "Offline";

				loading.value = false;
		});
	}

	function stopStream() {
		closeSocket(socket);
		socket = null;
		if (chartFlushTimer !== null) {
			clearTimeout(chartFlushTimer);
			chartFlushTimer = null;
		}
	}



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
		chartEvents, 
	};
});
