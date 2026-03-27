// Name: stores/systemDashboardState.ts
// Description: Encapsulates dashboard event buffers, history snapshots, and reactive summary state.
// Programmers: Adam Berry, Barrett Brown
// Creation Date: 3/1
// Revision Dates: 3/14 Barrett Brown
// Revision Notes: 3/14 Barrett Brown split the larger system store into focused modules.
// Preconditions: Temperature and valve events are normalized before being queued.
// Postconditions: Not Relevant
// Error Types: Not Relevant
// Invariants: UI flush batching merges websocket bursts into one render pass.
// Known Faults: None

import { ref } from "vue";
import type {
	MLAnomalyPayload,
	SystemStatus,
	TempEventData,
	ValveEventData,
} from "@/stores/systemTypes";
import type { createLiveChartState } from "@/stores/systemChart";

const LIVE_EVENT_LIMIT = 8;
const LIVE_ALERT_LIMIT = 8;

type LiveChartState = ReturnType<typeof createLiveChartState>;

export function createDashboardState(
	uiFlushIntervalMs: number,
	liveChart: LiveChartState,
) {
	const recentEvents = ref<TempEventData[]>([]);
	const recentValveEvents = ref<ValveEventData[]>([]);
	const mlAlerts = ref<MLAnomalyPayload[]>([]);
	const faultStatus = ref("Idle");
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

	let pendingTempEvents: TempEventData[] = [];
	let pendingValveEvents: ValveEventData[] = [];
	let pendingMLAlerts: MLAnomalyPayload[] = [];
	let pendingRecordDelta = 0;
	let pendingLastIngestMs = 0;
	let pendingLastIngestLabel = "";
	let pendingChartDirty = false;
	let uiFlushTimer: ReturnType<typeof setTimeout> | null = null;
	let historicalEventBuffer: TempEventData[] = [];
	let historicalValveEventBuffer: ValveEventData[] = [];
	let historicalMLAlertBuffer: MLAnomalyPayload[] = [];

	function updateActiveAlerts(): void {
		const ruleAlerts = recentEvents.value.reduce((count, event) => {
			return count + (event.anomalies?.length ?? 0);
		}, 0);
		metrics.value.activeAlerts = ruleAlerts + mlAlerts.value.length;
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

	function queueTempEvent(event: TempEventData): void {
		pendingTempEvents.push(event);
		pendingRecordDelta += 1;
		pendingChartDirty = true;
		if (
			event.display_timestamp &&
			event.timestamp_ms &&
			event.timestamp_ms >= pendingLastIngestMs
		) {
			pendingLastIngestMs = event.timestamp_ms;
			pendingLastIngestLabel = event.display_timestamp;
		}
	}

	function queueValveEvent(event: ValveEventData): void {
		pendingValveEvents.push(event);
		pendingRecordDelta += 1;
		pendingChartDirty = true;
		if (
			event.display_timestamp &&
			event.timestamp_ms &&
			event.timestamp_ms >= pendingLastIngestMs
		) {
			pendingLastIngestMs = event.timestamp_ms;
			pendingLastIngestLabel = event.display_timestamp;
		}
	}

	function queueMLAlert(alert: MLAnomalyPayload): void {
		pendingMLAlerts.push(alert);
	}

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
			for (const event of pendingTempEvents) {
				liveChart.appendTemperatureSample(event);
			}
		}

		if (hasValveEvents) {
			for (let index = pendingValveEvents.length - 1; index >= 0; index -= 1) {
				historicalValveEventBuffer.unshift(pendingValveEvents[index]!);
			}
			recentValveEvents.value = prependNewestFirst(
				recentValveEvents.value,
				pendingValveEvents,
				LIVE_EVENT_LIMIT,
			);
			for (const event of pendingValveEvents) {
				liveChart.appendValveSample(event);
			}
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
			liveChart.flush();
		}
		if (hasTempEvents || hasValveEvents || hasMLAlerts) {
			updateActiveAlerts();
		}

		pendingTempEvents = [];
		pendingValveEvents = [];
		pendingMLAlerts = [];
		pendingRecordDelta = 0;
		pendingLastIngestMs = 0;
		pendingLastIngestLabel = "";
		pendingChartDirty = false;
	}

	function scheduleUIFlush(): void {
		if (uiFlushTimer !== null) {
			return;
		}
		uiFlushTimer = setTimeout(() => {
			uiFlushTimer = null;
			flushPendingUIUpdates();
		}, uiFlushIntervalMs);
	}

	function getHistoricalEventSnapshot(): Array<TempEventData | ValveEventData> {
		return [...historicalEventBuffer, ...historicalValveEventBuffer]
			.slice()
			.sort(
				(left, right) => (right.timestamp_ms ?? 0) - (left.timestamp_ms ?? 0),
			);
	}

	function getHistoricalMLAlertSnapshot(): MLAnomalyPayload[] {
		return historicalMLAlertBuffer.slice();
	}

	function resetDashboardData(): void {
		recentEvents.value = [];
		recentValveEvents.value = [];
		mlAlerts.value = [];
		metrics.value.activeAlerts = 0;
		metrics.value.lastIngest = "Waiting for stream";
		faultStatus.value = "Idle";
		pendingTempEvents = [];
		pendingValveEvents = [];
		pendingMLAlerts = [];
		pendingRecordDelta = 0;
		pendingLastIngestMs = 0;
		pendingLastIngestLabel = "";
		pendingChartDirty = false;
		historicalEventBuffer = [];
		historicalValveEventBuffer = [];
		historicalMLAlertBuffer = [];
		liveChart.reset();
	}

	function clearTimers(): void {
		if (uiFlushTimer !== null) {
			clearTimeout(uiFlushTimer);
			uiFlushTimer = null;
		}
	}

	return {
		recentEvents,
		recentValveEvents,
		mlAlerts,
		faultStatus,
		status,
		metrics,
		queueTempEvent,
		queueValveEvent,
		queueMLAlert,
		flushPendingUIUpdates,
		scheduleUIFlush,
		getHistoricalEventSnapshot,
		getHistoricalMLAlertSnapshot,
		resetDashboardData,
		clearTimers,
	};
}
