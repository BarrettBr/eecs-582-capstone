<script setup lang="ts">
import { computed, reactive, watch } from "vue";
import { storeToRefs } from "pinia";
import Chart from "primevue/chart";
import Button from "primevue/button";
import { useSystemStore } from "@/stores/system";
import { getDefaultIngestAPIBaseURL } from "@/utils/wsHelper";
import type { ServiceCatalogEntry, ServiceMachineEntry } from "@/stores/systemTypes";

interface MachineHistoryResponse {
	labels: string[];
	values: number[];
	alert_counts: number[];
}

interface CachedMachineChart {
	labels: string[];
	values: number[];
}

const cache = reactive<Record<string, CachedMachineChart>>({});
const loadingByKey = reactive<Record<string, boolean>>({});
const expanded = reactive<Record<string, boolean>>({});
const expandedServices = reactive<Record<string, boolean>>({});
const debounceTimers = new Map<string, ReturnType<typeof setTimeout>>();

const store = useSystemStore();
const { availableServices } = storeToRefs(store);

const services = computed(() =>
	availableServices.value
		.filter((service) => (service.machines?.length ?? 0) > 0)
		.map((service) => ({
			...service,
			machines: (service.machines ?? []).slice().sort((left, right) => left.id.localeCompare(right.id)),
		})),
);

watch(
	services,
	(nextServices) => {
		if (nextServices.length === 0) {
			return;
		}
		for (const service of nextServices) {
			if (!(service.name in expandedServices)) {
				expandedServices[service.name] = false;
			}
		}
		const hasOpenService = nextServices.some(
			(service) => expandedServices[service.name],
		);
		if (!hasOpenService) {
			expandedServices[nextServices[0]!.name] = true;
		}
	},
	{ immediate: true },
);

const topStats = computed(() => {
	const allMachines = services.value.flatMap((service) => service.machines ?? []);
	const machinesWithAlerts = allMachines.filter(
		(machine) => (machine.recent_alerts ?? 0) > 0,
	).length;
	const disconnectedMachines = allMachines.filter(
		(machine) => !machine.connected,
	).length;
	const healthyMachines = allMachines.length - machinesWithAlerts - disconnectedMachines;
	return [
		{
			label: "Tracked Services",
			value: String(services.value.length),
		},
		{
			label: "Total Machines",
			value: String(allMachines.length),
		},
		{
			label: "Machines With Alerts",
			value: String(machinesWithAlerts),
		},
		{
			label: "Disconnected Machines",
			value: String(disconnectedMachines),
		},
		{
			label: "Healthy Machines",
			value: String(Math.max(0, healthyMachines)),
		},
	];
});

function machineKey(serviceName: string, machineID: string): string {
	return `${serviceName}|${machineID}`;
}

function statusClass(machine: ServiceMachineEntry): string {
	if (!machine.connected) {
		return "status-red";
	}
	if ((machine.recent_alerts ?? 0) > 0) {
		return "status-yellow";
	}
	return "status-green";
}

function formatRate(rate?: number): string {
	if (typeof rate !== "number" || !Number.isFinite(rate)) {
		return "0/s";
	}
	if (rate >= 1000) {
		return `${(rate / 1000).toFixed(rate >= 10000 ? 0 : 1)}k/s`;
	}
	return `${Math.round(rate)}/s`;
}

function chartDataFor(key: string, machine: ServiceMachineEntry) {
	const cached = cache[key];
	return {
		labels: cached?.labels ?? [],
		datasets: [
			{
				label: machine.name || machine.id,
				data: cached?.values ?? [],
				fill: false,
				borderColor: "#0ea5e9",
				tension: 0.35,
			},
		],
	};
}

const chartOptions = {
	animation: { duration: 0 },
	responsive: true,
	maintainAspectRatio: false,
	plugins: { legend: { display: false } },
	scales: { y: { beginAtZero: false } },
};

async function fetchMachineHistory(service: ServiceCatalogEntry, machine: ServiceMachineEntry): Promise<void> {
	const key = machineKey(service.name, machine.id);
	if (loadingByKey[key]) {
		return;
	}
	loadingByKey[key] = true;
	try {
		const response = await fetch(
			`${getDefaultIngestAPIBaseURL()}/machineHistory/${encodeURIComponent(machine.id)}?service_name=${encodeURIComponent(service.name)}&window=hour`,
		);
		if (!response.ok) {
			throw new Error(`status ${response.status}`);
		}
		const payload = (await response.json()) as MachineHistoryResponse;
		cache[key] = {
			labels: payload.labels ?? [],
			values: payload.values ?? [],
		};
	} catch (error) {
		console.error("Machine history fetch failed", error);
		cache[key] = { labels: [], values: [] };
	} finally {
		loadingByKey[key] = false;
	}
}

function inspectMachine(service: ServiceCatalogEntry, machine: ServiceMachineEntry): void {
	const key = machineKey(service.name, machine.id);
	const nextExpanded = !expanded[key];
	expanded[key] = nextExpanded;
	if (!nextExpanded) {
		return;
	}
	if (cache[key]) {
		return;
	}
	const previousTimer = debounceTimers.get(key);
	if (previousTimer) {
		clearTimeout(previousTimer);
	}
	const timer = setTimeout(() => {
		void fetchMachineHistory(service, machine);
		debounceTimers.delete(key);
	}, 125);
	debounceTimers.set(key, timer);
}

function toggleService(serviceName: string): void {
	expandedServices[serviceName] = !expandedServices[serviceName];
}
</script>

<template>
	<section class="machine-inspection" aria-label="Machine inspection">
		<div class="section-header">Machine Inspection</div>
		<div class="stats-row">
			<div v-for="stat in topStats" :key="stat.label" class="stat-card">
				<div class="stat-value">{{ stat.value }}</div>
				<div class="stat-label">{{ stat.label }}</div>
			</div>
		</div>
		<div v-if="services.length === 0" class="empty-state">No machine data yet</div>
		<div v-for="service in services" :key="service.name" class="service-accordion">
			<button type="button" class="service-summary" @click="toggleService(service.name)">
				<span>{{ service.alias_name || service.name }} with {{ service.machines?.length ?? 0 }} machines</span>
				<span class="count-pill">{{ service.machines?.length ?? 0 }}</span>
			</button>
			<div v-if="expandedServices[service.name]" class="machine-list">
				<div v-for="machine in service.machines" :key="machine.id" class="machine-row">
					<div
						class="machine-main"
						role="button"
						tabindex="0"
						@click="inspectMachine(service, machine)"
						@keydown.enter.prevent="inspectMachine(service, machine)"
						@keydown.space.prevent="inspectMachine(service, machine)"
					>
						<span class="status-dot" :class="statusClass(machine)" />
						<span class="machine-id">{{ machine.id }}</span>
						<span class="machine-name">{{ machine.name || machine.instance_name }}</span>
						<span class="machine-metric">alerts: {{ machine.recent_alerts ?? 0 }}</span>
						<span class="machine-metric">{{ formatRate(machine.admitted_eps_5s) }}</span>
						<Button
							icon="pi pi-search"
							size="small"
							severity="secondary"
							text
							@click.stop="inspectMachine(service, machine)"
						/>
					</div>
					<div v-if="expanded[machineKey(service.name, machine.id)]" class="inspect-chart">
						<div v-if="loadingByKey[machineKey(service.name, machine.id)]" class="chart-loading">
							Loading last hour...
						</div>
						<Chart
							v-else
							type="line"
							:data="chartDataFor(machineKey(service.name, machine.id), machine)"
							:options="chartOptions"
							style="height: 180px"
						/>
					</div>
				</div>
			</div>
		</div>
	</section>
</template>

<style scoped>
.machine-inspection {
	padding: 0.8rem 0.95rem 1rem;
}

.section-header {
	font-size: 0.78rem;
	font-weight: 700;
	letter-spacing: 0.06em;
	text-transform: uppercase;
	color: var(--p-surface-500);
	padding: 0.45rem 0.35rem 0.65rem;
}

.empty-state {
	font-size: 0.82rem;
	color: var(--p-surface-500);
	padding: 0.25rem 0.4rem;
}

.service-accordion {
	border: 1px solid var(--p-surface-200);
	border-radius: 10px;
	margin-bottom: 0.55rem;
	background: var(--p-surface-0);
}

.service-summary {
	display: flex;
	justify-content: space-between;
	align-items: center;
	width: 100%;
	padding: 0.55rem 0.65rem;
	cursor: pointer;
	font-size: 0.84rem;
	font-weight: 600;
	border: none;
	background: transparent;
	color: inherit;
	text-align: left;
}

.count-pill {
	font-size: 0.74rem;
	color: var(--p-surface-600);
	background: var(--p-surface-100);
	border-radius: 999px;
	padding: 0.1rem 0.45rem;
}

.machine-list {
	padding: 0.15rem 0.45rem 0.45rem;
}

.machine-row {
	border: 1px solid var(--p-surface-200);
	border-radius: 8px;
	margin-top: 0.35rem;
}

.machine-main {
	display: flex;
	align-items: center;
	gap: 0.45rem;
	padding: 0.35rem 0.4rem;
	cursor: pointer;
}

.status-dot {
	width: 9px;
	height: 9px;
	border-radius: 50%;
	flex-shrink: 0;
}

.status-green {
	background: #16a34a;
}

.status-yellow {
	background: #f59e0b;
}

.status-red {
	background: #dc2626;
}

.machine-id {
	font-weight: 700;
	font-size: 0.76rem;
}

.machine-name {
	font-size: 0.75rem;
	flex: 1;
	min-width: 0;
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
}

.machine-metric {
	font-size: 0.7rem;
	color: var(--p-surface-600);
	white-space: nowrap;
}

.inspect-chart {
	padding: 0.2rem 0.4rem 0.4rem;
}

.chart-loading {
	font-size: 0.76rem;
	color: var(--p-surface-500);
	padding: 0.65rem 0;
}

.stats-row {
	display: grid;
	grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
	gap: 0.55rem;
	padding: 0.15rem 0.1rem 0.8rem;
}

.stat-card {
	border: 1px solid var(--p-surface-200);
	border-radius: 10px;
	background: var(--p-surface-50);
	padding: 0.5rem 0.6rem;
}

.stat-value {
	font-size: 1.05rem;
	font-weight: 700;
	color: var(--p-surface-900);
}

.stat-label {
	font-size: 0.72rem;
	color: var(--p-surface-600);
}
</style>
