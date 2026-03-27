<!-- Name: DashboardStatusCard.vue
Description: Displays backend stream health and the currently active service subscriptions.
Programmers: Adam Berry, Barrett Brown
Creation Date: 2/14
Revision Dates: Adam Berry 2/14, Adam Berry 2/15, Barrett Brown 3/1, Adam Berry 3/1, Barrett Brown 3/14
Revision Notes: Barrett Brown 3/14 split the larger dashboard view into focused components.
Preconditions: The parent provides the current system status fields and service summary text.
Postconditions: Not Relevant
Error Types: Not Relevant
Invariants: Dependencies described in /Docs/web.md
Known Faults: None
-->
<script setup lang="ts">
import { computed } from "vue";
import Card from "primevue/card";
import Tag from "primevue/tag";
import Button from "primevue/button";
import type { SystemStatus } from "@/stores/systemTypes";

const props = defineProps<{
	isServiceDashboard: boolean;
	status: SystemStatus;
	streamState: string;
	activeServiceSummary: string;
	activeServiceNames: string[];
}>();

const emit = defineEmits<{
	(event: "subscribe"): void;
}>();

// description: Maps the websocket state text into the matching PrimeVue tag severity.
// input: The current websocket connection label from the dashboard store.
// output: Returns the visual severity string used by the status tag.
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

const previewServiceNames = computed(() => props.activeServiceNames.slice(0, 3));

const hiddenServiceCount = computed(() =>
	Math.max(0, props.activeServiceNames.length - previewServiceNames.value.length),
);
</script>

<template>
	<Card class="status-card">
		<template #title>
			<div class="card-title-row">
				<span>System Status</span>
				<Button
					v-if="!props.isServiceDashboard"
					label="Subscribe"
					icon="pi pi-plus"
					size="small"
					severity="secondary"
					outlined
					@click="emit('subscribe')"
				/>
			</div>
		</template>
		<template #content>
			<div class="status-grid">
				<div class="status-item">
					<span class="status-label">API</span>
					<Tag
						:value="props.status.api"
						:severity="props.status.api === 'Online' ? 'success' : 'danger'"
					/>
				</div>

				<div class="status-item">
					<span class="status-label">Ingestion</span>
					<Tag
						:value="props.status.ingestion"
						:severity="
							props.status.ingestion === 'Streaming' ? 'success' : 'danger'
						"
					/>
				</div>

				<div class="status-item">
					<span class="status-label">ML Service</span>
					<Tag
						:value="props.status.ml"
						:severity="props.status.ml === 'Receiving' ? 'success' : 'danger'"
					/>
				</div>

				<div class="status-item">
					<span class="status-label">Websocket</span>
					<Tag
						:value="props.streamState"
						:severity="getSocketSeverity(props.streamState)"
					/>
				</div>
			</div>
			<div class="service-summary">
				<div class="service-summary-label">Active Services</div>
				<div class="service-summary-value">{{ props.activeServiceSummary }}</div>
				<div
					v-if="previewServiceNames.length > 0"
					class="service-summary-list"
				>
					<span
						v-for="serviceName in previewServiceNames"
						:key="serviceName"
						class="service-chip"
					>
						{{ serviceName }}
					</span>
					<span
						v-if="hiddenServiceCount > 0"
						class="service-chip service-chip--more"
					>
						+{{ hiddenServiceCount }} more
					</span>
				</div>
			</div>
		</template>
	</Card>
</template>

<style scoped>
.status-card {
	padding: 1rem;
}

.card-title-row {
	display: flex;
	align-items: center;
	justify-content: space-between;
	gap: 1rem;
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
	font-weight: 600;
}

.service-summary-list {
	display: flex;
	flex-wrap: wrap;
	gap: 0.5rem;
	margin-top: 0.75rem;
}

.service-chip {
	display: inline-flex;
	align-items: center;
	min-height: 2rem;
	padding: 0.3rem 0.7rem;
	border-radius: 999px;
	background: var(--p-surface-100);
	color: var(--p-surface-700);
	font-size: 0.85rem;
	line-height: 1.2;
}

.service-chip--more {
	background: var(--p-surface-200);
	color: var(--p-surface-800);
}

</style>
