// Name: stores/systemChart.ts
// Description: Encapsulates the rolling live chart state used by the dashboard store.
// Programmers: Adam Berry, Barrett Brown
// Creation Date: 3/1
// Revision Dates: 3/14 Barrett Brown
// Revision Notes: 3/14 Barrett Brown split the larger system store into focused modules.
// Preconditions: Temperature samples use millisecond timestamps.
// Postconditions: Not Relevant
// Error Types: Not Relevant
// Invariants: Chart buckets stay aligned to the configured bucket width and point limit.
// Known Faults: None

import { ref } from "vue";
import type {
	ServiceCatalogEntry,
	ServiceEventType,
	TempEventData,
	ValveEventData,
} from "@/stores/systemTypes";
import {
	formatDisplayTime,
	resolveTimestampMs,
} from "@/stores/systemFormatters";

interface ChartBucket {
	bucketStartMs: number;
	sum: number;
	count: number;
}

const CHART_POINT_LIMIT = 50;
const CHART_BUCKET_WIDTH_MS = 150;
const CHART_Y_MIN_PADDING = 0.5;
const CHART_Y_PADDING_RATIO = 0.15;
const CHART_Y_STEP = 1;
const CHART_Y_MIN_RANGE = 4;
const CHART_Y_SHRINK_DELAY_MS = 10_000;

interface LiveChartPresentation {
	datasetLabel: string;
	cardTitle: string;
	borderColor: string;
	minFloor?: number;
}

interface LiveChartSeriesState {
	presentation: LiveChartPresentation;
	chartBuckets: ChartBucket[];
	chartShrinkLockedUntilMs: number;
}

export interface LiveChartOption {
	key: string;
	serviceName: string;
	eventType: ServiceEventType;
	label: string;
	title: string;
}

type ChartDataset = {
	label: string;
	data: Array<number | undefined>;
	fill: boolean;
	borderColor: string;
	tension: number;
};

type LiveChartData = {
	labels: string[];
	datasets: [ChartDataset];
};

type LiveChartBounds = {
	min: number | null;
	max: number | null;
};

const EMPTY_CHART_BOUNDS: LiveChartBounds = {
	min: null,
	max: null,
};

const CHART_PRESENTATIONS: Record<ServiceEventType, LiveChartPresentation> = {
	temperature: {
		datasetLabel: "Temperature",
		cardTitle: "Live Temperature",
		borderColor: "#10b981",
	},
	valve: {
		datasetLabel: "Valve Flow Rate",
		cardTitle: "Live Valve Flow",
		borderColor: "#0ea5e9",
		minFloor: 0,
	},
};

export function buildLiveChartKey(
	serviceName: string,
	eventType: ServiceEventType,
): string {
	return `${serviceName}:${eventType}`;
}

export function buildLiveChartOption(
	service: Pick<ServiceCatalogEntry, "name" | "alias_name" | "event_type">,
): LiveChartOption {
	const presentation = CHART_PRESENTATIONS[service.event_type];
	const displayName = service.alias_name?.trim() || service.name;
	return {
		key: buildLiveChartKey(service.name, service.event_type),
		serviceName: service.name,
		eventType: service.event_type,
		label: `${displayName} ${presentation.datasetLabel}`,
		title: `${presentation.cardTitle} - ${displayName}`,
	};
}

function createChartData(presentation: LiveChartPresentation): LiveChartData {
	return {
		labels: [],
		datasets: [
			{
				label: presentation.datasetLabel,
				data: [],
				fill: false,
				borderColor: presentation.borderColor,
				tension: 0.4,
			},
		],
	};
}

export function createLiveChartState() {
	const chartStateByKey = new Map<string, LiveChartSeriesState>();
	const liveChartData = ref<Record<string, LiveChartData>>({});
	const liveChartBounds = ref<Record<string, LiveChartBounds>>({});

	function alignBucketStart(timestampMs: number): number {
		return (
			Math.floor(timestampMs / CHART_BUCKET_WIDTH_MS) * CHART_BUCKET_WIDTH_MS
		);
	}

	function createEmptyBucket(bucketStartMs: number): ChartBucket {
		return {
			bucketStartMs,
			sum: 0,
			count: 0,
		};
	}

	function ensureChartState(
		chartKey: string,
		eventType: ServiceEventType,
	): LiveChartSeriesState {
		const existing = chartStateByKey.get(chartKey);
		if (existing) {
			return existing;
		}

		const presentation = CHART_PRESENTATIONS[eventType];
		const created: LiveChartSeriesState = {
			presentation,
			chartBuckets: [],
			chartShrinkLockedUntilMs: 0,
		};
		chartStateByKey.set(chartKey, created);
		liveChartData.value = {
			...liveChartData.value,
			[chartKey]: createChartData(presentation),
		};
		liveChartBounds.value = {
			...liveChartBounds.value,
			[chartKey]: { ...EMPTY_CHART_BOUNDS },
		};
		return created;
	}

	function resetChartBuckets(
		state: LiveChartSeriesState,
		latestBucketStartMs: number,
	): void {
		state.chartBuckets = [];
		for (let index = CHART_POINT_LIMIT - 1; index >= 0; index -= 1) {
			state.chartBuckets.push(
				createEmptyBucket(latestBucketStartMs - index * CHART_BUCKET_WIDTH_MS),
			);
		}
	}

	function advanceChartBuckets(
		state: LiveChartSeriesState,
		latestBucketStartMs: number,
	): void {
		if (state.chartBuckets.length === 0) {
			resetChartBuckets(state, latestBucketStartMs);
			return;
		}

		const currentLatestBucket = state.chartBuckets[state.chartBuckets.length - 1]!;
		if (latestBucketStartMs <= currentLatestBucket.bucketStartMs) {
			return;
		}

		const bucketShift = Math.floor(
			(latestBucketStartMs - currentLatestBucket.bucketStartMs) /
				CHART_BUCKET_WIDTH_MS,
		);
		if (bucketShift >= CHART_POINT_LIMIT) {
			resetChartBuckets(state, latestBucketStartMs);
			return;
		}

		for (let index = 0; index < bucketShift; index += 1) {
			state.chartBuckets.shift();
			const nextBucketStartMs =
				currentLatestBucket.bucketStartMs + (index + 1) * CHART_BUCKET_WIDTH_MS;
			state.chartBuckets.push(createEmptyBucket(nextBucketStartMs));
		}
	}

	function appendSample(
		chartKey: string,
		eventType: ServiceEventType,
		timestamp: string,
		timestampMsInput: number | undefined,
		value: number | undefined,
	): void {
		if (typeof value !== "number") {
			return;
		}

		const state = ensureChartState(chartKey, eventType);
		const timestampMs = resolveTimestampMs(timestamp, timestampMsInput);
		const bucketStartMs = alignBucketStart(timestampMs);
		advanceChartBuckets(state, bucketStartMs);
		if (state.chartBuckets.length === 0) {
			return;
		}

		const oldestBucketStartMs = state.chartBuckets[0]!.bucketStartMs;
		if (bucketStartMs < oldestBucketStartMs) {
			return;
		}

		const bucketIndex = Math.floor(
			(bucketStartMs - oldestBucketStartMs) / CHART_BUCKET_WIDTH_MS,
		);
		const bucket = state.chartBuckets[bucketIndex];
		if (!bucket) {
			return;
		}

		bucket.sum += value;
		bucket.count += 1;
	}

	function appendTemperatureSample(event: TempEventData): void {
		if (!event.service_name) {
			return;
		}
		appendSample(
			buildLiveChartKey(event.service_name, "temperature"),
			"temperature",
			event.timestamp,
			event.timestamp_ms,
			event.temperature,
		);
	}

	function appendValveSample(event: ValveEventData): void {
		if (!event.service_name) {
			return;
		}
		appendSample(
			buildLiveChartKey(event.service_name, "valve"),
			"valve",
			event.timestamp,
			event.timestamp_ms,
			event.flow_rate,
		);
	}

	function flush(): void {
		const nextChartData: Record<string, LiveChartData> = {};
		const nextChartBounds: Record<string, LiveChartBounds> = {};

		for (const [chartKey, state] of chartStateByKey.entries()) {
			if (state.chartBuckets.length === 0) {
				nextChartData[chartKey] = createChartData(state.presentation);
				nextChartBounds[chartKey] = { ...EMPTY_CHART_BOUNDS };
				continue;
			}

			const labels = new Array<string>(CHART_POINT_LIMIT);
			const values = new Array<number | undefined>(CHART_POINT_LIMIT);
			for (let index = 0; index < CHART_POINT_LIMIT; index += 1) {
				const bucket = state.chartBuckets[index]!;
				labels[index] = formatDisplayTime(bucket.bucketStartMs);
				if (bucket.count > 0) {
					values[index] = bucket.sum / bucket.count;
				}
			}

			const definedValues = values.filter(
				(value): value is number => typeof value === "number",
			);
			const currentBounds =
				liveChartBounds.value[chartKey] ?? { ...EMPTY_CHART_BOUNDS };
			let nextBounds = currentBounds;

			if (definedValues.length > 0) {
				const observedMin = Math.min(...definedValues);
				const observedMax = Math.max(...definedValues);
				const observedRange = Math.max(0.1, observedMax - observedMin);
				const padding = Math.max(
					CHART_Y_MIN_PADDING,
					observedRange * CHART_Y_PADDING_RATIO,
				);
				let targetMin =
					Math.floor((observedMin - padding) / CHART_Y_STEP) * CHART_Y_STEP;
				let targetMax =
					Math.ceil((observedMax + padding) / CHART_Y_STEP) * CHART_Y_STEP;
				if (typeof state.presentation.minFloor === "number") {
					targetMin = Math.max(state.presentation.minFloor, targetMin);
				}
				if (targetMax - targetMin < CHART_Y_MIN_RANGE) {
					const midpoint = (targetMin + targetMax) / 2;
					const halfRange = CHART_Y_MIN_RANGE / 2;
					targetMin =
						Math.floor((midpoint - halfRange) / CHART_Y_STEP) * CHART_Y_STEP;
					targetMax = targetMin + CHART_Y_MIN_RANGE;
					if (typeof state.presentation.minFloor === "number") {
						targetMin = Math.max(state.presentation.minFloor, targetMin);
						targetMax = Math.max(targetMin + CHART_Y_MIN_RANGE, targetMax);
					}
				}
				const nowMs = Date.now();

				if (
					currentBounds.min === null ||
					currentBounds.max === null ||
					observedMin < currentBounds.min ||
					observedMax > currentBounds.max
				) {
					nextBounds = {
						min: targetMin,
						max: targetMax,
					};
					state.chartShrinkLockedUntilMs = nowMs + CHART_Y_SHRINK_DELAY_MS;
				} else if (
					nowMs >= state.chartShrinkLockedUntilMs &&
					targetMin >= currentBounds.min + CHART_Y_STEP &&
					targetMax <= currentBounds.max - CHART_Y_STEP
				) {
					nextBounds = {
						min: targetMin,
						max: targetMax,
					};
					state.chartShrinkLockedUntilMs = nowMs + CHART_Y_SHRINK_DELAY_MS;
				}
			}

			nextChartBounds[chartKey] = nextBounds;
			nextChartData[chartKey] = {
				labels,
				datasets: [
					{
						label: state.presentation.datasetLabel,
						fill: false,
						borderColor: state.presentation.borderColor,
						tension: 0.4,
						data: values,
					},
				],
			};
		}

		liveChartData.value = nextChartData;
		liveChartBounds.value = nextChartBounds;
	}

	function getChartData(chartKey: string | null): LiveChartData {
		if (!chartKey) {
			return createChartData(CHART_PRESENTATIONS.temperature);
		}
		return liveChartData.value[chartKey] ?? createChartData(CHART_PRESENTATIONS.temperature);
	}

	function getChartBounds(chartKey: string | null): LiveChartBounds {
		if (!chartKey) {
			return { ...EMPTY_CHART_BOUNDS };
		}
		return liveChartBounds.value[chartKey] ?? { ...EMPTY_CHART_BOUNDS };
	}

	function reset(): void {
		chartStateByKey.clear();
		liveChartData.value = {};
		liveChartBounds.value = {};
	}

	return {
		liveChartData,
		liveChartBounds,
		appendTemperatureSample,
		appendValveSample,
		flush,
		reset,
		getChartData,
		getChartBounds,
	};
}
