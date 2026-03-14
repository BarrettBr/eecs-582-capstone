<!-- Name: Dashboard.vue
Description: The component for creating the dashboard
Programmers: Adam Berry, Barrett Brown
Creation Date: 2/14
Revision Dates: Adam Berry 2/14, Adam Berry 2/15, Barrett Brown 3/1, Adam Berry 3/1, Barrett Brown 3/14
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
import EventSwitcherTable from "@/components/EventSwitcherTable.vue";
import HistoricalSnapshotTable from "@/components/HistoricalSnapshotTable.vue";
import DashboardHeader from "@/components/DashboardHeader.vue";
import DashboardStatusCard from "@/components/DashboardStatusCard.vue";
import DashboardChartCard from "@/components/DashboardChartCard.vue";
import DashboardMetricsCard from "@/components/DashboardMetricsCard.vue";
import ServiceSubscriptionDialog from "@/components/ServiceSubscriptionDialog.vue";

const store = useSystemStore();
const route = useRoute();
const router = useRouter();
const subscribeDialogVisible = ref(false);

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

// description: Opens the subscription dialog seeded with the current overview selection.
// input: No arguments; uses the current dashboard mode to decide when the button is shown.
// output: Makes the service selection dialog visible.
function openSubscribeDialog() {
	subscribeDialogVisible.value = true;
}

// description: Applies the selected services returned from the dialog to the overview dashboard.
// input: The list of service names chosen in the subscription dialog.
// output: Updates the store subscription state and resets the visible dialog.
function applySubscriptions(serviceNames: string[]) {
	updateDashboardSubscriptions(serviceNames);
}
</script>

<template>
	<main>
		<div class="dashboard">
			<DashboardHeader
				:title="dashboardTitle"
				:subtitle="dashboardSubtitle"
			/>

			<div v-if="loading" class="loading">Loading system data...</div>

			<div v-else class="grid">
				<DashboardStatusCard
					class="dashboard-card dashboard-card--narrow"
					:isServiceDashboard="isServiceDashboard"
					:status="status"
					:streamState="streamState"
					:activeServiceSummary="activeServiceSummary"
					@subscribe="openSubscribeDialog"
				/>

				<DashboardChartCard
					class="dashboard-card dashboard-card--wide"
					:title="chartTitle"
					:data="store.temperatureChartData"
					:options="chartOptions"
				/>

				<DashboardMetricsCard
					class="dashboard-card dashboard-card--narrow"
					:metrics="metrics"
					:canInjectFault="canInjectFault"
					:faultStatus="faultStatus"
					@inject-fault="injectSimulatorFault"
				/>

				<EventSwitcherTable
					class="dashboard-card dashboard-card--wide"
					:recentEvents="recentEvents"
					:recentValveEvents="recentValveEvents"
					:mlAlerts="mlAlerts"
				/>

				<HistoricalSnapshotTable
					class="dashboard-card dashboard-card--full"
					:getEventHistorySnapshot="store.getHistoricalEventSnapshot"
					:getMLAlertHistorySnapshot="store.getHistoricalMLAlertSnapshot"
				/>
			</div>
		</div>

		<ServiceSubscriptionDialog
			:visible="subscribeDialogVisible"
			:selection="selectedDashboardServices"
			:serviceOptions="serviceOptions"
			@update:visible="subscribeDialogVisible = $event"
			@apply="applySubscriptions"
		/>
	</main>
</template>

<style scoped>
.dashboard {
	padding: 2rem;
}

.grid {
	display: grid;
	grid-template-columns: repeat(12, 1fr);
	gap: 1.5rem;
}

.dashboard-card {
	grid-column: span 12;
	min-width: 0;
}

.loading {
	font-style: italic;
}

@media (min-width: 768px) {
	.dashboard-card--narrow {
		grid-column: span 4;
	}

	.dashboard-card--wide {
		grid-column: span 8;
	}

	.dashboard-card--full {
		grid-column: span 12;
	}
}
</style>
