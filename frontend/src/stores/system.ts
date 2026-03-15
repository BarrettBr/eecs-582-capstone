// Name: stores/system.ts
// Description: This file acts as the orchestration store for dashboard stream state and service subscriptions.
// Programmers: Adam Berry, Barrett Brown
// Creation Date: 3/1
// Revision Dates: 3/15 Barrett Brown
// Revision Notes: 3/15 Barrett Brown added aggregated websocket service rate handling for sidebar labels.
// Revision Notes: 3/15 Barrett Brown decoupled API health from websocket disconnect state in the status store.
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
	getDefaultIngestAPIBaseURL,
	getDefaultStreamURL,
	onBatch,
	send as sendSocket,
	type ManagedWebSocket,
	type StreamBatch,
} from "@/utils/wsHelper";
import {
	normalizeMLAlert,
	normalizeTempEvent,
	normalizeValveEvent,
} from "@/stores/systemFormatters";
import { createTemperatureChartState } from "@/stores/systemChart";
import { createDashboardState } from "@/stores/systemDashboardState";
import type {
	IngestionMetricsResponse,
	IngestionStatusResponse,
	MLAnomalyPayload,
	ServiceCatalogEntry,
	ServiceCatalogPayload,
	ServiceRatesPayload,
	SubscriptionAckPayload,
	SubscriptionsPrunedPayload,
	TempEventData,
	ValveEventData,
} from "@/stores/systemTypes";

export const useSystemStore = defineStore("system", () => {
	const DEFAULT_UI_FLUSH_INTERVAL_MS = 25;
	const parsedUIFlushInterval = Number(
		import.meta.env.VITE_STREAM_UI_FLUSH_INTERVAL_MS ??
			DEFAULT_UI_FLUSH_INTERVAL_MS,
	);
	const uiFlushIntervalMs =
		Number.isFinite(parsedUIFlushInterval) && parsedUIFlushInterval > 0
			? parsedUIFlushInterval
			: DEFAULT_UI_FLUSH_INTERVAL_MS;

	const loading = ref(true);
	const availableServices = ref<ServiceCatalogEntry[]>([]);
	const selectedDashboardServices = ref<string[]>([]);
	const currentSubscriptions = ref<string[]>([]);
	const focusedServiceName = ref<string | null>(null);
	const streamState = ref("Connecting...");
	let socket: ManagedWebSocket | null = null;

	const temperatureChart = createTemperatureChartState();
	const dashboardState = createDashboardState(uiFlushIntervalMs, temperatureChart);
	const {
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
	} = dashboardState;

	function sortServices(
		services: ServiceCatalogEntry[],
	): ServiceCatalogEntry[] {
		return services
			.slice()
			.sort((left, right) => left.name.localeCompare(right.name));
	}

	function activeDashboardServices(): string[] {
		if (focusedServiceName.value) {
			return [focusedServiceName.value];
		}
		return selectedDashboardServices.value.slice();
	}

	function syncSubscriptions(): void {
		if (!socket) {
			return;
		}
		sendSocket(socket, {
			kind: "set_service_subscriptions",
			source: "frontend",
			timestamp: new Date().toISOString(),
			data: {
				service_names: activeDashboardServices(),
			},
		});
	}

	function applyServiceCatalog(services: ServiceCatalogEntry[]): void {
		const previousRates = new Map(
			availableServices.value.map((service) => [
				service.name,
				service.admitted_eps_5s ?? 0,
			]),
		);
		const sortedServices = sortServices(services);
		const availableServiceNames = new Set(
			sortedServices.map((service) => service.name),
		);

		availableServices.value = sortedServices.map((service) => ({
			...service,
			admitted_eps_5s: previousRates.get(service.name) ?? 0,
		}));
		selectedDashboardServices.value = selectedDashboardServices.value.filter(
			(serviceName) => availableServiceNames.has(serviceName),
		);
		currentSubscriptions.value = currentSubscriptions.value.filter((serviceName) =>
			availableServiceNames.has(serviceName),
		);

		if (
			focusedServiceName.value &&
			!availableServiceNames.has(focusedServiceName.value)
		) {
			focusedServiceName.value = null;
		}

		if (
			!focusedServiceName.value &&
			selectedDashboardServices.value.length === 0 &&
			sortedServices.length > 0
		) {
			selectedDashboardServices.value = sortedServices.map(
				(service) => service.name,
			);
		}

		syncSubscriptions();
	}

	function applyServiceRates(payload: ServiceRatesPayload): void {
		const rateByName = new Map(
			(payload.services ?? []).map((service) => [
				service.name,
				service.admitted_eps_5s ?? 0,
			]),
		);
		availableServices.value = availableServices.value.map((service) => ({
			...service,
			admitted_eps_5s: rateByName.get(service.name) ?? 0,
		}));
	}

	async function loadInitialMetrics(): Promise<void> {
		try {
			const response = await fetch(
				`${getDefaultIngestAPIBaseURL()}/ingestion/metrics`,
			);
			if (!response.ok) {
				throw new Error(`status ${response.status}`);
			}

			const payload = (await response.json()) as IngestionMetricsResponse;
			if (
				typeof payload.total_records === "number" &&
				Number.isFinite(payload.total_records) &&
				payload.total_records >= 0
			) {
				metrics.value.totalRecords = payload.total_records;
			}
			status.value.api = "Online";
		} catch (error) {
			console.error("Failed to load ingest metrics", error);
			status.value.api = "Offline";
		}
	}

	async function loadAvailableServices(): Promise<void> {
		try {
			const response = await fetch(
				`${getDefaultIngestAPIBaseURL()}/ingestion/status`,
			);
			if (!response.ok) {
				throw new Error(`status ${response.status}`);
			}

			const payload = (await response.json()) as IngestionStatusResponse;
			applyServiceCatalog(payload.services ?? []);
			status.value.api = "Online";
		} catch (error) {
			console.error("Failed to load service catalog", error);
			status.value.api = "Offline";
		}
	}

	function setFocusedService(serviceName: string | null): void {
		const nextServiceName =
			serviceName && serviceName.trim() !== "" ? serviceName : null;
		if (focusedServiceName.value === nextServiceName) {
			return
		}
		focusedServiceName.value = nextServiceName;
		resetDashboardData();
		syncSubscriptions();
	}

	function updateDashboardSubscriptions(serviceNames: string[]): void {
		const unique = Array.from(
			new Set(
				serviceNames.filter((serviceName) =>
					availableServices.value.some(
						(service) => service.name === serviceName,
					),
				),
			),
		).sort((left, right) => left.localeCompare(right));
		focusedServiceName.value = null;
		selectedDashboardServices.value = unique;
		resetDashboardData();
		syncSubscriptions();
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

		for (const message of batch.messages) {
			if (message.kind === "service_catalog") {
				const payload = message.data as ServiceCatalogPayload;
				applyServiceCatalog(payload.services ?? []);
				continue;
			}
			if (message.kind === "subscription_ack") {
				const payload = message.data as SubscriptionAckPayload;
				currentSubscriptions.value = payload.current_services?.slice() ?? [];
				if (!focusedServiceName.value) {
					selectedDashboardServices.value =
						payload.current_services?.slice() ?? [];
				}
				continue;
			}
			if (message.kind === "service_rates") {
				applyServiceRates(message.data as ServiceRatesPayload);
				continue;
			}
			if (message.kind === "subscriptions_pruned") {
				const payload = message.data as SubscriptionsPrunedPayload;
				currentSubscriptions.value = payload.current_services?.slice() ?? [];
				if (!focusedServiceName.value) {
					selectedDashboardServices.value =
						payload.current_services?.slice() ?? [];
				}
				continue;
			}
			if (message.kind === "event") {
				if (message.event_type === "temperature") {
					queueTempEvent(
						normalizeTempEvent({
							...(message.data as TempEventData),
							service_name: message.service_name,
						}),
					);
					continue;
				}
				if (message.event_type === "valve") {
					queueValveEvent(
						normalizeValveEvent({
							...(message.data as ValveEventData),
							service_name: message.service_name,
						}),
					);
				}
				continue;
			}
			if (message.kind === "ml_result") {
				queueMLAlert(
					normalizeMLAlert({
						...(message.data as MLAnomalyPayload),
						service_name: message.service_name,
					}),
				);
			}
		}

		scheduleUIFlush();
	}

	async function startStream() {
		if (socket) return;

		await Promise.all([loadInitialMetrics(), loadAvailableServices()]);

		const streamURL = getDefaultStreamURL();
		console.log("Attempting websocket connection", streamURL);
		socket = connectSocket(streamURL);

		socket.addEventListener("open", () => {
			console.log("Websocket connected", streamURL);
			streamState.value = "Connected";
			status.value.ingestion = "Streaming";
			status.value.ml = "Listening";
			syncSubscriptions();
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
			loading.value = false;
		});

		socket.addEventListener("error", (err) => {
			console.error("Websocket stream error", err);
			streamState.value = "Error";
			status.value.ingestion = "Stream Error";
			status.value.ml = "Offline";
			loading.value = false;
		});
	}

	function stopStream() {
		flushPendingUIUpdates();
		closeSocket(socket);
		socket = null;
		clearTimers();
	}

	return {
		loading,
		recentEvents,
		recentValveEvents,
		mlAlerts,
		availableServices,
		selectedDashboardServices,
		currentSubscriptions,
		focusedServiceName,
		streamState,
		faultStatus,
		status,
		metrics,
		injectSimulatorFault,
		loadAvailableServices,
		startStream,
		stopStream,
		setFocusedService,
		updateDashboardSubscriptions,
		getHistoricalEventSnapshot,
		getHistoricalMLAlertSnapshot,
		temperatureChartData: temperatureChart.temperatureChartData,
		temperatureChartBounds: temperatureChart.temperatureChartBounds,
	};
});
