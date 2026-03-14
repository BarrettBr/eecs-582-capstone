// Name: stores/systemChart.ts
// Description: Encapsulates the rolling temperature chart state used by the dashboard store.
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
import type { TempEventData } from "@/stores/systemTypes";
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

export function createTemperatureChartState() {
	let chartBuckets: ChartBucket[] = [];
	let chartShrinkLockedUntilMs = 0;

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
	const temperatureChartBounds = ref<{
		min: number | null;
		max: number | null;
	}>({
		min: null,
		max: null,
	});

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

	function resetChartBuckets(latestBucketStartMs: number): void {
		chartBuckets = [];
		for (let index = CHART_POINT_LIMIT - 1; index >= 0; index -= 1) {
			chartBuckets.push(
				createEmptyBucket(latestBucketStartMs - index * CHART_BUCKET_WIDTH_MS),
			);
		}
	}

	function advanceChartBuckets(latestBucketStartMs: number): void {
		if (chartBuckets.length === 0) {
			resetChartBuckets(latestBucketStartMs);
			return;
		}

		const currentLatestBucket = chartBuckets[chartBuckets.length - 1]!;
		if (latestBucketStartMs <= currentLatestBucket.bucketStartMs) {
			return;
		}

		const bucketShift = Math.floor(
			(latestBucketStartMs - currentLatestBucket.bucketStartMs) /
				CHART_BUCKET_WIDTH_MS,
		);
		if (bucketShift >= CHART_POINT_LIMIT) {
			resetChartBuckets(latestBucketStartMs);
			return;
		}

		for (let index = 0; index < bucketShift; index += 1) {
			chartBuckets.shift();
			const nextBucketStartMs =
				currentLatestBucket.bucketStartMs + (index + 1) * CHART_BUCKET_WIDTH_MS;
			chartBuckets.push(createEmptyBucket(nextBucketStartMs));
		}
	}

	function appendSample(event: TempEventData): void {
		const timestampMs = resolveTimestampMs(event.timestamp, event.timestamp_ms);
		const bucketStartMs = alignBucketStart(timestampMs);
		advanceChartBuckets(bucketStartMs);
		if (chartBuckets.length === 0) {
			return;
		}

		const oldestBucketStartMs = chartBuckets[0]!.bucketStartMs;
		if (bucketStartMs < oldestBucketStartMs) {
			return;
		}

		const bucketIndex = Math.floor(
			(bucketStartMs - oldestBucketStartMs) / CHART_BUCKET_WIDTH_MS,
		);
		const bucket = chartBuckets[bucketIndex];
		if (!bucket || typeof event.temperature !== "number") {
			return;
		}

		bucket.sum += event.temperature;
		bucket.count += 1;
	}

	function flush(): void {
		if (chartBuckets.length === 0) {
			reset();
			return;
		}

		const labels = new Array<string>(CHART_POINT_LIMIT);
		const temperatures = new Array<number | undefined>(CHART_POINT_LIMIT);
		for (let index = 0; index < CHART_POINT_LIMIT; index += 1) {
			const bucket = chartBuckets[index]!;
			labels[index] = formatDisplayTime(bucket.bucketStartMs);
			if (bucket.count > 0) {
				temperatures[index] = bucket.sum / bucket.count;
			}
		}

		const definedTemperatures = temperatures.filter(
			(value): value is number => typeof value === "number",
		);
		if (definedTemperatures.length > 0) {
			const observedMin = Math.min(...definedTemperatures);
			const observedMax = Math.max(...definedTemperatures);
			const observedRange = Math.max(0.1, observedMax - observedMin);
			const padding = Math.max(
				CHART_Y_MIN_PADDING,
				observedRange * CHART_Y_PADDING_RATIO,
			);
			let targetMin =
				Math.floor((observedMin - padding) / CHART_Y_STEP) * CHART_Y_STEP;
			let targetMax =
				Math.ceil((observedMax + padding) / CHART_Y_STEP) * CHART_Y_STEP;
			if (targetMax - targetMin < CHART_Y_MIN_RANGE) {
				const midpoint = (targetMin + targetMax) / 2;
				const halfRange = CHART_Y_MIN_RANGE / 2;
				targetMin =
					Math.floor((midpoint - halfRange) / CHART_Y_STEP) * CHART_Y_STEP;
				targetMax = targetMin + CHART_Y_MIN_RANGE;
			}
			const currentMin = temperatureChartBounds.value.min;
			const currentMax = temperatureChartBounds.value.max;
			const nowMs = Date.now();

			if (
				currentMin === null ||
				currentMax === null ||
				observedMin < currentMin ||
				observedMax > currentMax
			) {
				temperatureChartBounds.value = {
					min: targetMin,
					max: targetMax,
				};
				chartShrinkLockedUntilMs = nowMs + CHART_Y_SHRINK_DELAY_MS;
			} else if (
				nowMs >= chartShrinkLockedUntilMs &&
				targetMin >= currentMin + CHART_Y_STEP &&
				targetMax <= currentMax - CHART_Y_STEP
			) {
				temperatureChartBounds.value = {
					min: targetMin,
					max: targetMax,
				};
				chartShrinkLockedUntilMs = nowMs + CHART_Y_SHRINK_DELAY_MS;
			}
		}

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

	function reset(): void {
		chartBuckets = [];
		chartShrinkLockedUntilMs = 0;
		temperatureChartBounds.value = {
			min: null,
			max: null,
		};
		temperatureChartData.value = {
			labels: [],
			datasets: [
				{
					...temperatureChartData.value.datasets[0]!,
					data: [],
				},
			],
		};
	}

	return {
		temperatureChartData,
		temperatureChartBounds,
		appendSample,
		flush,
		reset,
	};
}
