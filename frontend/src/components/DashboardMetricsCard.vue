<!-- Name: DashboardMetricsCard.vue
Description: Displays aggregate ingest metrics and the simulator fault action for the dashboard.
Programmers: Adam Berry, Barrett Brown
Creation Date: 2/14
Revision Dates: Adam Berry 2/14, Adam Berry 2/15, Barrett Brown 3/1, Adam Berry 3/1, Barrett Brown 3/14
Revision Notes: Barrett Brown 3/14 split the larger dashboard view into focused components.
Preconditions: The parent provides the current metric values and whether fault injection is available.
Postconditions: Not Relevant
Error Types: Not Relevant
Invariants: Dependencies described in /Docs/web.md
Known Faults: None
-->
<script setup lang="ts">
import Card from "primevue/card";
import Tag from "primevue/tag";
import Button from "primevue/button";

defineProps<{
	metrics: {
		totalRecords: number;
		activeAlerts: number;
		lastIngest: string;
	};
	canInjectFault: boolean;
	faultStatus: string;
}>();

const emit = defineEmits<{
	(event: "inject-fault"): void;
}>();
</script>

<template>
	<Card class="metrics-card">
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
						@click="emit('inject-fault')"
					/>
					<Tag
						:value="faultStatus"
						:severity="faultStatus === 'Injected' ? 'danger' : 'info'"
					/>
				</div>
			</div>
		</template>
	</Card>
</template>

<style scoped>
.metrics-card {
	padding: 1rem;
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

</style>
