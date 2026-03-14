// Name: stores/systemFormatters.ts
// Description: Timestamp formatting and stream payload normalization helpers for the dashboard store.
// Programmers: Adam Berry, Barrett Brown
// Creation Date: 3/1
// Revision Dates: 3/14 Barrett Brown
// Revision Notes: 3/14 Barrett Brown split the larger system store into focused modules.
// Preconditions: Stream payloads use the expected timestamp fields.
// Postconditions: Not Relevant
// Error Types: Not Relevant
// Invariants: Normalized dashboard records always include cached display timestamp fields.
// Known Faults: None

import type {
	MLAnomalyPayload,
	TempEventData,
	ValveEventData,
} from "@/stores/systemTypes";

export function formatDisplayTime(timestampMs: number): string {
	return new Date(timestampMs).toLocaleTimeString();
}

export function formatDisplayTimestamp(timestampMs: number): string {
	return new Date(timestampMs).toLocaleString();
}

export function resolveTimestampMs(
	timestamp: string,
	timestampMs?: number,
): number {
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

export function normalizeTempEvent(event: TempEventData): TempEventData {
	const timestampMs = resolveTimestampMs(event.timestamp, event.timestamp_ms);
	return {
		...event,
		event_type: "temperature",
		timestamp_ms: timestampMs,
		display_time: formatDisplayTime(timestampMs),
		display_timestamp: formatDisplayTimestamp(timestampMs),
	};
}

export function normalizeValveEvent(event: ValveEventData): ValveEventData {
	const timestampMs = resolveTimestampMs(event.timestamp, event.timestamp_ms);
	return {
		...event,
		event_type: "valve",
		timestamp_ms: timestampMs,
		display_time: formatDisplayTime(timestampMs),
		display_timestamp: formatDisplayTimestamp(timestampMs),
	};
}

export function normalizeMLAlert(message: MLAnomalyPayload): MLAnomalyPayload {
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
