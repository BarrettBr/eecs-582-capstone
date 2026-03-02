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
	const DEFAULT_UI_FLUSH_INTERVAL_MS = 25;
	const parsedUIFlushInterval = Number(
		import.meta.env.VITE_STREAM_UI_FLUSH_INTERVAL_MS ??
			DEFAULT_UI_FLUSH_INTERVAL_MS,
	);
	const UI_FLUSH_INTERVAL_MS =
		Number.isFinite(parsedUIFlushInterval) && parsedUIFlushInterval > 0
			? parsedUIFlushInterval
			: DEFAULT_UI_FLUSH_INTERVAL_MS;
	const LIVE_EVENT_LIMIT = 8;
	const LIVE_ALERT_LIMIT = 8;
	const CHART_POINT_LIMIT = 50;

	interface SystemStatus {
		api: string;
		ingestion: string;
		ml: string;
	}

	interface TempEventData {
		id?: number;
		timestamp: string;
		timestamp_ms?: number;
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
		timestamp_ms?: number;
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
		generated_at_ms?: number;
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
	let pendingTempEvents: TempEventData[] = [];
	let pendingValveEvents: ValveEventData[] = [];
	let pendingMLAlerts: MLAnomalyPayload[] = [];
	let pendingRecordDelta = 0;
	let pendingLastIngestLabel = "";
	let pendingChartDirty = false;
	let uiFlushTimer: ReturnType<typeof setTimeout> | null = null;
	let historicalEventBuffer: TempEventData[] = [];
	let historicalMLAlertBuffer: MLAnomalyPayload[] = [];

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

	// description: Recounts alert totals from rule-based and ML results.
	// input: Reads the current event and alert arrays.
	// output: Updates the active alert metric shown in the UI.
	function updateActiveAlerts(): void {
		const ruleAlerts = recentEvents.value.reduce((count, event) => {
			return count + (event.anomalies?.length ?? 0);
		}, 0);
		metrics.value.activeAlerts = ruleAlerts + mlAlerts.value.length;
	}

	// description: Converts one ISO timestamp into a short display time.
	// input: A timestamp string from the backend stream.
	// output: Returns a local time string for tables and charts.
	function formatDisplayTime(timestampMs: number): string {
		return new Date(timestampMs).toLocaleTimeString();
	}

	// description: Converts one ISO timestamp into a fuller display string.
	// input: A timestamp string from the backend stream.
	// output: Returns a local date-time string for summary fields.
	function formatDisplayTimestamp(timestampMs: number): string {
		return new Date(timestampMs).toLocaleString();
	}

	function resolveTimestampMs(timestamp: string, timestampMs?: number): number {
		if (
			typeof timestampMs === "number" &&
			Number.isFinite(timestampMs) &&
			timestampMs > 0
		) {
			return timestampMs;
		}
		const parsed = Date.parse(timestamp);
		return Number.isFinite(parsed) ? parsed : Date.now();
	}

	// description: Adds cached display timestamps to a temperature event.
	// input: One raw temperature event from the stream.
	// output: Returns the normalized event used by the dashboard.
	function normalizeTempEvent(event: TempEventData): TempEventData {
		const timestampMs = resolveTimestampMs(event.timestamp, event.timestamp_ms);
		return {
			...event,
			timestamp_ms: timestampMs,
			display_time: formatDisplayTime(timestampMs),
			display_timestamp: formatDisplayTimestamp(timestampMs),
		};
	}

	// description: Adds cached display timestamps to a valve event.
	// input: One raw valve event from the stream.
	// output: Returns the normalized valve event for display.
	function normalizeValveEvent(event: ValveEventData): ValveEventData {
		const timestampMs = resolveTimestampMs(event.timestamp, event.timestamp_ms);
		return {
			...event,
			timestamp_ms: timestampMs,
			display_time: formatDisplayTime(timestampMs),
			display_timestamp: formatDisplayTimestamp(timestampMs),
		};
	}

	// description: Adds a cached display time to one ML alert message.
	// input: One ML alert payload from the stream.
	// output: Returns the normalized ML alert for UI rendering.
	function normalizeMLAlert(message: MLAnomalyPayload): MLAnomalyPayload {
		const generatedAtMs = resolveTimestampMs(
			message.generated_at,
			message.generated_at_ms,
		);
		return {
			...message,
			generated_at_ms: generatedAtMs,
			display_time: formatDisplayTime(generatedAtMs),
		};
	}

	// description: Rebuilds the reactive chart data from the buffered samples.
	// input: Reads the current non-reactive chart buffer.
	// output: Updates chart labels and temperature series in one batch.
	function flushChartUpdates(): void {
		const count = chartBuffer.length;
		const labels = new Array<string>(count);
		const temperatures = new Array<number | undefined>(count);

		for (let index = 0; index < count; index += 1) {
			const event = chartBuffer[count - 1 - index]!;
			labels[index] =
				event.display_time ??
				formatDisplayTime(
					resolveTimestampMs(event.timestamp, event.timestamp_ms),
				);
			temperatures[index] = event.temperature;
		}

		chartEvents.value = chartBuffer.slice();
		const currentDataset = temperatureChartData.value.datasets[0]!;
		temperatureChartData.value = {
			labels,
			datasets: [
				{
					label: currentDataset.label,
					fill: currentDataset.fill,
					borderColor: currentDataset.borderColor,
					tension: currentDataset.tension,
					data: temperatures,
				},
			],
		};
	}

	// description: Schedules one UI flush so multiple websocket batches merge into one render pass.
	// input: Uses the pending timer state.
	// output: Starts one delayed flush if none is already pending.
	function scheduleUIFlush(): void {
		if (uiFlushTimer !== null) {
			return;
		}
		uiFlushTimer = setTimeout(() => {
			uiFlushTimer = null;
			flushPendingUIUpdates();
		}, UI_FLUSH_INTERVAL_MS);
	}

	// description: Returns a frozen copy of the full temperature event history.
	// input: No arguments; reads the non-reactive history buffer.
	// output: Returns an array snapshot for the historical table.
	function getHistoricalEventSnapshot(): TempEventData[] {
		return historicalEventBuffer.slice();
	}

	// description: Returns a frozen copy of the full ML alert history.
	// input: No arguments; reads the non-reactive ML history buffer.
	// output: Returns an array snapshot for the historical table.
	function getHistoricalMLAlertSnapshot(): MLAnomalyPayload[] {
		return historicalMLAlertBuffer.slice();
	}

	function prependNewestFirst<T>(
		current: T[],
		incoming: T[],
		limit: number,
	): T[] {
		if (incoming.length === 0) {
			return current;
		}
		const newestFirst = incoming.slice().reverse();
		return newestFirst.concat(current).slice(0, limit);
	}

	// description: Applies the pending non-reactive batch delta to reactive UI state in one pass.
	// input: Reads the pending batch buffers and counters.
	// output: Updates the dashboard refs once per flush interval.
	function flushPendingUIUpdates(): void {
		const hasTempEvents = pendingTempEvents.length > 0;
		const hasValveEvents = pendingValveEvents.length > 0;
		const hasMLAlerts = pendingMLAlerts.length > 0;
		if (
			!hasTempEvents &&
			!hasValveEvents &&
			!hasMLAlerts &&
			pendingRecordDelta === 0
		) {
			return;
		}

		if (hasTempEvents) {
			for (let index = pendingTempEvents.length - 1; index >= 0; index -= 1) {
				historicalEventBuffer.unshift(pendingTempEvents[index]!);
			}
			recentEvents.value = prependNewestFirst(
				recentEvents.value,
				pendingTempEvents,
				LIVE_EVENT_LIMIT,
			);
			chartBuffer = prependNewestFirst(
				chartBuffer,
				pendingTempEvents,
				CHART_POINT_LIMIT,
			);
		}

		if (hasValveEvents) {
			recentValveEvents.value = prependNewestFirst(
				recentValveEvents.value,
				pendingValveEvents,
				LIVE_EVENT_LIMIT,
			);
		}

		if (hasMLAlerts) {
			for (let index = pendingMLAlerts.length - 1; index >= 0; index -= 1) {
				historicalMLAlertBuffer.unshift(pendingMLAlerts[index]!);
			}
			mlAlerts.value = prependNewestFirst(
				mlAlerts.value,
				pendingMLAlerts,
				LIVE_ALERT_LIMIT,
			);
			status.value.ml = "Receiving";
		}

		if (pendingRecordDelta > 0) {
			metrics.value.totalRecords += pendingRecordDelta;
		}
		if (pendingLastIngestLabel !== "") {
			metrics.value.lastIngest = pendingLastIngestLabel;
		}
		if (pendingChartDirty) {
			flushChartUpdates();
		}
		if (hasTempEvents || hasValveEvents || hasMLAlerts) {
			updateActiveAlerts();
		}

		pendingTempEvents = [];
		pendingValveEvents = [];
		pendingMLAlerts = [];
		pendingRecordDelta = 0;
		pendingLastIngestLabel = "";
		pendingChartDirty = false;
	}

	// description: Sends a request that forces the simulator into a fault burst.
	// input: Uses the current websocket connection and the temp_dev source name.
	// output: Updates the local fault status based on send success.
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

	// description: Routes one websocket batch into the right frontend collections.
	// input: A parsed websocket batch from the ingest stream.
	// output: Pushes temperature, valve, or ML messages into store state.
	function handleStreamMessage(batch: StreamBatch): void {
		if (batch.kind !== "batch" || !Array.isArray(batch.messages)) {
			return;
		}

		//console.log("Websocket batch received", batch);
		let batchRecordDelta = 0;
		let lastIngestMs = 0;
		let lastIngestLabel = "";

		for (const message of batch.messages) {
			if (message.kind === "event") {
				if (message.event_type === "temperature") {
					const normalized = normalizeTempEvent(message.data as TempEventData);
					pendingTempEvents.push(normalized);
					batchRecordDelta += 1;
					pendingChartDirty = true;
					if ((normalized.timestamp_ms ?? 0) >= lastIngestMs) {
						lastIngestMs = normalized.timestamp_ms ?? lastIngestMs;
						lastIngestLabel =
							normalized.display_timestamp ??
							formatDisplayTimestamp(lastIngestMs);
					}
					continue;
				}
				if (message.event_type === "valve") {
					const normalized = normalizeValveEvent(
						message.data as ValveEventData,
					);
					pendingValveEvents.push(normalized);
					batchRecordDelta += 1;
					if ((normalized.timestamp_ms ?? 0) >= lastIngestMs) {
						lastIngestMs = normalized.timestamp_ms ?? lastIngestMs;
						lastIngestLabel =
							normalized.display_timestamp ??
							formatDisplayTimestamp(lastIngestMs);
					}
				}
				continue;
			}
			if (message.kind === "ml_result") {
				pendingMLAlerts.push(
					normalizeMLAlert(message.data as MLAnomalyPayload),
				);
			}
		}

		pendingRecordDelta += batchRecordDelta;
		if (lastIngestLabel !== "") {
			pendingLastIngestLabel = lastIngestLabel;
		}
		scheduleUIFlush();
	}

	// description: Opens the live websocket stream and attaches listeners.
	// input: No arguments; uses the configured stream URL.
	// output: Connects the store to live ingest updates.
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

	// description: Stops the live stream and clears any pending chart timer.
	// input: No arguments; uses the current socket and timer state.
	// output: Closes the stream connection and resets local handles.
	function stopStream() {
		flushPendingUIUpdates();
		closeSocket(socket);
		socket = null;
		if (uiFlushTimer !== null) {
			clearTimeout(uiFlushTimer);
			uiFlushTimer = null;
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
		getHistoricalEventSnapshot,
		getHistoricalMLAlertSnapshot,
		temperatureChartData,
		chartEvents,
	};
});
