<!-- Name: Dashboard.vue
Description: The component for creating the dashboard
Programmers: Adam Berry
Creation Date: 2/14
Revision Dates: Adam Berry 2/14, Adam Berry 2/15, Barret Brown 3/1, Adam Berry 3/1, Barrett Brown 3/14
Preconditions: Not Relevant
Postconditions: Not Relevant
Error Types: Not Relevant
Invariants: Dependencies described in /Docs/web.md
Known Faults: None
-->
<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { storeToRefs } from "pinia";
import { useRoute, useRouter } from "vue-router";
import { useSystemStore } from "@/stores/system";
import Chart from "primevue/chart";
import Card from "primevue/card";
import Tag from "primevue/tag";
import Button from "primevue/button";
import Dialog from "primevue/dialog";
import MultiSelect from "primevue/multiselect";
import EventSwitcherTable from "@/components/EventSwitcherTable.vue";
import HistoricalSnapshotTable from "@/components/HistoricalSnapshotTable.vue";

const store = useSystemStore();
const route = useRoute();
const router = useRouter();
const subscribeDialogVisible = ref(false);
const pendingServiceSelection = ref<string[]>([]);

const {
	loading,
	availableServices,
	recentEvents,
	recentValveEvents,
	mlAlerts,
	currentSubscriptions,
	selectedDashboardServices,
	streamState,
	faultStatus,
	status,
	metrics,
	temperatureChartBounds,
} = storeToRefs(store);

const {
	injectSimulatorFault,
	setFocusedService,
	updateDashboardSubscriptions,
} = store;

onMounted(() => {
	store.startStream();
});

const currentServiceName = computed(() => {
	const raw = route.params.serviceName;
	return typeof raw === "string" && raw.trim() !== "" ? raw : null;
});

const isServiceDashboard = computed(() => currentServiceName.value !== null);

const currentService = computed(() => {
	return (
		availableServices.value.find(
			(service) => service.name === currentServiceName.value,
		) ?? null
	);
});

const serviceOptions = computed(() =>
	availableServices.value.map((service) => ({
		label: `${service.name} (${service.event_type})`,
		value: service.name,
	})),
);

const dashboardTitle = computed(() => {
	if (!currentService.value) {
		return "Dashboard";
	}
	return `${currentService.value.name} Dashboard`;
});

const dashboardSubtitle = computed(() => {
	if (isServiceDashboard.value) {
		return "Mini-dashboard for one service.";
	}
	return "Overview dashboard for the currently selected services.";
});

const activeServiceSummary = computed(() => {
	if (currentSubscriptions.value.length === 0) {
		return "No subscribed services";
	}
	return currentSubscriptions.value.join(", ");
});

const canInjectFault = computed(() => {
	if (currentServiceName.value) {
		return currentServiceName.value === "temp_dev";
	}
	return currentSubscriptions.value.includes("temp_dev");
});

const chartTitle = computed(() => {
	if (currentService.value?.event_type === "valve") {
		return "Live Flow (temperature chart unavailable for valve view)";
	}
	return "Live Temperature";
});

watch(
	currentServiceName,
	(serviceName) => {
		setFocusedService(serviceName);
	},
	{ immediate: true },
);

watch(
	[currentServiceName, availableServices],
	([serviceName, services]) => {
		if (
			serviceName &&
			services.length > 0 &&
			!services.some((service) => service.name === serviceName)
		) {
			router.replace("/dashboard");
		}
	},
	{ immediate: true },
);

watch(faultStatus, (newValue) => {
	if (newValue === "Injected") {
		setTimeout(() => {
			store.faultStatus = "Idle";
		}, 2000);
	}
});

const chartOptions = computed(() => {
	const yScale: {
		beginAtZero: boolean;
		min?: number;
		max?: number;
	} = {
		beginAtZero: false,
	};

	if (
		typeof temperatureChartBounds.value.min === "number" &&
		typeof temperatureChartBounds.value.max === "number"
	) {
		yScale.min = temperatureChartBounds.value.min;
		yScale.max = temperatureChartBounds.value.max;
	}

	return {
		animation: {
			duration: 0,
		},
		hover: {
			mode: "nearest",
			intersect: true,
		},
		scales: {
			y: yScale,
		},
		responsive: true,
		maintainAspectRatio: false,
	};
});

function getSocketSeverity(state: string) {
	switch (state) {
		case "Connected":
			return "success";
		case "Connecting...":
			return "warning";
		case "Error":
		case "Disconnected":
			return "danger";
		default:
			return "info";
	}
}

function openSubscribeDialog() {
	pendingServiceSelection.value = selectedDashboardServices.value.slice();
	subscribeDialogVisible.value = true;
}

function applySubscriptions() {
	updateDashboardSubscriptions(pendingServiceSelection.value);
	subscribeDialogVisible.value = false;
}
</script>

<template>
	<main>
		<div class="dashboard">
			<div class="dashboard-header">
				<div>
					<h1 class="dashboard-title">{{ dashboardTitle }}</h1>
					<p class="dashboard-subtitle">{{ dashboardSubtitle }}</p>
				</div>
			</div>

			<div v-if="loading" class="loading">Loading system data...</div>

			<div v-else class="grid">
				<Card class="card">
					<template #title>
						<div class="card-title-row">
							<span>System Status</span>
							<Button
								v-if="!isServiceDashboard"
								label="Subscribe"
								icon="pi pi-plus"
								size="small"
								severity="secondary"
								outlined
								@click="openSubscribeDialog"
							/>
						</div>
					</template>
					<template #content>
						<div class="status-grid">
							<div class="status-item">
								<span class="status-label">API</span>
								<Tag
									:value="status.api"
									:severity="status.api === 'Online' ? 'success' : 'danger'"
								/>
							</div>

							<div class="status-item">
								<span class="status-label">Ingestion</span>
								<Tag
									:value="status.ingestion"
									:severity="
										status.ingestion === 'Streaming' ? 'success' : 'danger'
									"
								/>
							</div>

							<div class="status-item">
								<span class="status-label">ML Service</span>
								<Tag
									:value="status.ml"
									:severity="status.ml === 'Receiving' ? 'success' : 'danger'"
								/>
							</div>

							<div class="status-item">
								<span class="status-label">Websocket</span>
								<Tag
									:value="streamState"
									:severity="getSocketSeverity(streamState)"
								/>
							</div>
						</div>
						<div class="service-summary">
							<div class="service-summary-label">Active Services</div>
							<div class="service-summary-value">{{ activeServiceSummary }}</div>
						</div>
					</template>
				</Card>

				<Card class="card col-span-2">
					<template #title>{{ chartTitle }}</template>
					<template #content>
						<Chart
							type="line"
							:data="store.temperatureChartData"
							:options="chartOptions"
							style="height: 300px"
						/>
					</template>
				</Card>

				<Card class="card">
					<template #title>Metrics</template>
					<template #content>
						<div class="metrics-grid">
							<div class="metric-item">
								<span class="metric-label">Total Records</span>
								<span class="metric-value">
									{{ metrics.totalRecords }}
								</span>
							</div>

							<div class="metric-item">
								<span class="metric-label">Active Alerts</span>
								<Tag
									:value="metrics.activeAlerts"
									:severity="metrics.activeAlerts > 0 ? 'danger' : 'success'"
								/>
							</div>

							<div class="metric-item">
								<span class="metric-label">Last Ingest</span>
								<span class="metric-subtle">
									{{ metrics.lastIngest }}
								</span>
							</div>

							<div v-if="canInjectFault" class="metric-actions">
								<Button
									label="Inject Fault"
									icon="pi pi-exclamation-triangle"
									severity="warning"
									@click="injectSimulatorFault"
								/>
								<Tag
									:value="faultStatus"
									:severity="faultStatus === 'Injected' ? 'danger' : 'info'"
								/>
							</div>
						</div>
					</template>
				</Card>

				<EventSwitcherTable
					:recentEvents="recentEvents"
					:recentValveEvents="recentValveEvents"
					:mlAlerts="mlAlerts"
				/>

				<HistoricalSnapshotTable
					:getEventHistorySnapshot="store.getHistoricalEventSnapshot"
					:getMLAlertHistorySnapshot="store.getHistoricalMLAlertSnapshot"
				/>
			</div>
		</div>

		<Dialog
			v-model:visible="subscribeDialogVisible"
			modal
			header="Select Services"
			:style="{ width: '30rem', maxWidth: '90vw' }"
		>
			<div class="subscribe-dialog">
				<p class="subscribe-copy">
					Choose the services you want shown on the overview dashboard.
				</p>
				<MultiSelect
					v-model="pendingServiceSelection"
					:options="serviceOptions"
					optionLabel="label"
					optionValue="value"
					placeholder="Select services"
					display="chip"
					class="subscribe-select"
				/>
				<div class="subscribe-actions">
					<Button
						label="Cancel"
						severity="secondary"
						text
						@click="subscribeDialogVisible = false"
					/>
					<Button label="Apply" @click="applySubscriptions" />
				</div>
			</div>
		</Dialog>
	</main>
</template>

<style scoped>
.dashboard {
	padding: 2rem;
}

.dashboard-header {
	margin-bottom: 1.25rem;
}

.dashboard-title {
	margin: 0;
	color: black;
}

.dashboard-subtitle {
	margin: 0.35rem 0 0;
	color: var(--p-surface-500);
}

.grid {
	display: grid;
	grid-template-columns: repeat(12, 1fr);
	gap: 1.5rem;
}

.card {
	padding: 1rem;
	grid-column: span 12;
}

@media (min-width: 768px) {
	.card {
		grid-column: span 4;
	}

	.col-span-2 {
		grid-column: span 8 !important;
	}

	.col-span-full {
		grid-column: span 12 !important;
	}
}

.card-title-row {
	display: flex;
	align-items: center;
	justify-content: space-between;
	gap: 1rem;
}

.loading {
	font-style: italic;
}

.metrics-grid {
	display: flex;
	flex-direction: column;
	gap: 1.25rem;
}

.metric-item {
	display: flex;
	justify-content: space-between;
	align-items: center;
}

.metric-label {
	font-weight: 500;
	color: var(--text-color-secondary);
}

.metric-value {
	font-size: 1.5rem;
	font-weight: 600;
	font-variant: tabular-nums;
}

.metric-subtle {
	font-size: 0.9rem;
	color: var(--text-color-secondary);
	text-align: right;
}

.metric-actions {
	display: flex;
	justify-content: space-between;
	align-items: center;
}

.status-grid {
	display: flex;
	flex-direction: column;
	gap: 1.25rem;
}

.status-item {
	display: flex;
	justify-content: space-between;
	align-items: center;
}

.status-label {
	font-weight: 500;
	color: var(--text-color-secondary);
}

.service-summary {
	margin-top: 1.25rem;
	padding-top: 1rem;
	border-top: 1px solid var(--p-surface-200);
}

.service-summary-label {
	font-size: 0.85rem;
	font-weight: 600;
	color: var(--p-surface-500);
}

.service-summary-value {
	margin-top: 0.35rem;
	line-height: 1.5;
}

.subscribe-dialog {
	display: grid;
	gap: 1rem;
}

.subscribe-copy {
	margin: 0;
	color: var(--p-surface-600);
}

.subscribe-select {
	width: 100%;
}

.subscribe-actions {
	display: flex;
	justify-content: flex-end;
	gap: 0.75rem;
}
</style>
