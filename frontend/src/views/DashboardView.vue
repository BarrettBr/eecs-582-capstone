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
import { buildLiveChartOption } from "@/stores/systemChart";
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
} = storeToRefs(store);

const {
	injectFault,
	setFocusedService,
	updateDashboardSubscriptions,
} = store;
const selectedChartKey = ref<string | null>(null);

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

function displayServiceName(serviceName: string | null | undefined): string {
	if (!serviceName) {
		return "";
	}
	const service = availableServices.value.find(
		(entry) => entry.name === serviceName,
	);
	return service?.alias_name?.trim() || service?.name || serviceName;
}

const serviceOptions = computed(() =>
	availableServices.value.map((service) => ({
		label: `${displayServiceName(service.name)} (${service.event_type})`,
		value: service.name,
	})),
);

const serviceDisplayNames = computed<Record<string, string>>(() =>
	Object.fromEntries(
		availableServices.value.map((service) => [
			service.name,
			displayServiceName(service.name),
		]),
	),
);

const dashboardTitle = computed(() => {
	if (!currentService.value) {
		return "Dashboard";
	}
	return `${displayServiceName(currentService.value.name)} Dashboard`;
});

const dashboardSubtitle = computed(() => {
	if (isServiceDashboard.value) {
		return "Mini-dashboard for one service.";
	}
	return "Overview dashboard for the currently selected services.";
});

const activeServiceSummary = computed(() => {
	const count = currentSubscriptions.value.length;
	if (count === 0) {
		return "No subscribed services";
	}
	if (count === 1) {
		return "1 service subscribed";
	}
	return `${count} services subscribed`;
});

const activeDashboardServiceEntries = computed(() => {
	const scopedNames = new Set(
		currentServiceName.value
			? [currentServiceName.value]
			: currentSubscriptions.value,
	);
	return availableServices.value.filter((service) => scopedNames.has(service.name));
});

const chartSwitcherOptions = computed(() =>
	activeDashboardServiceEntries.value.map((service) => {
		const option = buildLiveChartOption(service);
		return {
			label: option.label,
			value: option.key,
			title: option.title,
			serviceName: option.serviceName,
		};
	}),
);

const activeChartOption = computed(() => {
	return (
		chartSwitcherOptions.value.find(
			(option) => option.value === selectedChartKey.value,
		) ?? chartSwitcherOptions.value[0] ?? null
	);
});

const chartTitle = computed(() => {
	return activeChartOption.value?.title ?? "Live Metric";
});

const selectedFaultSourceName = computed(() => {
	if (currentServiceName.value) {
		return currentServiceName.value;
	}
	return activeChartOption.value?.serviceName ?? null;
});

const canInjectFault = computed(() => selectedFaultSourceName.value !== null);

const chartData = computed(() =>
	store.getChartData(activeChartOption.value?.value ?? null),
);

const activeChartBounds = computed(() =>
	store.getChartBounds(activeChartOption.value?.value ?? null),
);

watch(
	currentServiceName,
	(serviceName) => {
		setFocusedService(serviceName);
	},
	{ immediate: true },
);

watch(
	chartSwitcherOptions,
	(options) => {
		if (options.length === 0) {
			selectedChartKey.value = null;
			return;
		}
		if (
			selectedChartKey.value &&
			options.some((option) => option.value === selectedChartKey.value)
		) {
			return;
		}
		selectedChartKey.value = options[0]!.value;
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
		typeof activeChartBounds.value.min === "number" &&
		typeof activeChartBounds.value.max === "number"
	) {
		yScale.min = activeChartBounds.value.min;
		yScale.max = activeChartBounds.value.max;
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

function injectSelectedFault() {
	injectFault(selectedFaultSourceName.value);
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
					:activeServiceNames="
						currentSubscriptions.map((serviceName) =>
							displayServiceName(serviceName),
						)
					"
					@subscribe="openSubscribeDialog"
				/>

				<DashboardChartCard
					class="dashboard-card dashboard-card--wide"
					:title="chartTitle"
					:selection="selectedChartKey"
					:selections="chartSwitcherOptions"
					:data="chartData"
					:options="chartOptions"
					@update:selection="selectedChartKey = $event"
				/>

				<DashboardMetricsCard
					class="dashboard-card dashboard-card--narrow"
					:metrics="metrics"
					:canInjectFault="canInjectFault"
					:faultStatus="faultStatus"
					@inject-fault="injectSelectedFault"
				/>

				<EventSwitcherTable
					class="dashboard-card dashboard-card--wide"
					:recentEvents="recentEvents"
					:recentValveEvents="recentValveEvents"
					:mlAlerts="mlAlerts"
					:serviceDisplayNames="serviceDisplayNames"
				/>

				<HistoricalSnapshotTable
					class="dashboard-card dashboard-card--full"
					:getEventHistorySnapshot="store.getHistoricalEventSnapshot"
					:getMLAlertHistorySnapshot="store.getHistoricalMLAlertSnapshot"
					:serviceDisplayNames="serviceDisplayNames"
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
