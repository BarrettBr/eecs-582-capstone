<!-- Name: DashboardChartCard.vue
Description: Renders the live chart panel used by the overview and service dashboards.
Programmers: Adam Berry, Barrett Brown
Creation Date: 2/14
Revision Dates: Adam Berry 2/14, Adam Berry 2/15, Barrett Brown 3/1, Adam Berry 3/1, Barrett Brown 3/14
Revision Notes: Barrett Brown 3/14 split the larger dashboard view into focused components.
Preconditions: The parent passes PrimeVue chart data and options objects.
Postconditions: Not Relevant
Error Types: Not Relevant
Invariants: Dependencies described in /Docs/web.md
Known Faults: None
-->
<script setup lang="ts">
import { computed, ref, watch } from "vue";
import Chart from "primevue/chart";
import Card from "primevue/card";
import Button from "primevue/button";
import Dialog from "primevue/dialog";

const props = defineProps<{
	title: string;
	selection: string | null;
	selections: Array<{
		label: string;
		value: string;
	}>;
	data: {
		labels: string[];
		datasets: Array<{
			label: string;
			data: Array<number | undefined>;
			fill: boolean;
			borderColor: string;
			tension: number;
		}>;
	};
	options: Record<string, unknown>;
}>();

const emit = defineEmits<{
	(event: "update:selection", value: string): void;
}>();

const switcherVisible = ref(false);
const searchQuery = ref("");
const filteredSelections = computed(() => {
	const query = searchQuery.value.trim().toLowerCase();
	if (query === "") {
		return props.selections;
	}
	return props.selections.filter((option) =>
		option.label.toLowerCase().includes(query),
	);
});

watch(switcherVisible, (visible) => {
	if (visible) {
		searchQuery.value = "";
	}
});

function selectChart(value: string): void {
	emit("update:selection", value);
	switcherVisible.value = false;
}
</script>

<template>
	<Card class="card col-span-2">
		<template #title>
			<div class="chart-header">
				<div class="chart-header-copy">
					<div class="chart-header-label">Live Dashboard</div>
					<div class="chart-header-title">{{ props.title }}</div>
				</div>
				<Button
					v-if="props.selections.length > 1"
					label="Switcher"
					icon="pi pi-sliders-h"
					size="small"
					severity="secondary"
					outlined
					@click="switcherVisible = true"
				/>
			</div>
		</template>
		<template #content>
			<Chart
				type="line"
				:data="props.data"
				:options="props.options"
				style="height: 300px"
			/>

			<Dialog
				v-if="props.selections.length > 1"
				v-model:visible="switcherVisible"
				modal
				dismissableMask
				header="Choose Chart"
				:style="{ width: '28rem', maxWidth: '92vw' }"
				>
					<div class="switcher-dialog">
						<p class="switcher-copy">
							Select which live service stream you want graphed.
						</p>
						<label class="switcher-search">
							<span class="switcher-search-label">Search Charts</span>
							<input
								v-model="searchQuery"
								type="search"
								class="switcher-search-input"
								placeholder="Search by chart name"
							/>
						</label>
						<div class="switcher-options">
							<button
								v-for="option in filteredSelections"
								:key="option.value"
								type="button"
								class="switcher-option"
							:class="{
								'switcher-option--active': option.value === props.selection,
							}"
							@click="selectChart(option.value)"
						>
							<span>{{ option.label }}</span>
							<span
								v-if="option.value === props.selection"
								class="switcher-option-state"
							>
								Selected
								</span>
							</button>
							<div
								v-if="filteredSelections.length === 0"
								class="switcher-empty"
							>
								No charts match that search.
							</div>
						</div>
					</div>
				</Dialog>
		</template>
	</Card>
</template>

<style scoped>
.chart-header {
	display: flex;
	justify-content: space-between;
	align-items: flex-start;
	gap: 1rem;
}

.chart-header-copy {
	display: grid;
	gap: 0.35rem;
	min-height: 3.75rem;
	min-width: 0;
}

.chart-header-label {
	font-size: 0.82rem;
	font-weight: 700;
	letter-spacing: 0.06em;
	text-transform: uppercase;
	color: var(--p-surface-500);
}

.chart-header-title {
	line-height: 1.3;
	overflow-wrap: anywhere;
}

.switcher-dialog {
	display: grid;
	gap: 1rem;
}

.switcher-copy {
	margin: 0;
	color: var(--p-surface-600);
}

.switcher-search {
	display: grid;
	gap: 0.45rem;
}

.switcher-search-label {
	font-size: 0.85rem;
	font-weight: 600;
	color: var(--p-surface-600);
}

.switcher-search-input {
	width: 100%;
	padding: 0.75rem 0.9rem;
	border: 1px solid var(--p-surface-300);
	border-radius: 10px;
	background: var(--p-surface-0);
	color: inherit;
	font: inherit;
}

.switcher-search-input:focus {
	outline: none;
	border-color: var(--p-primary-500);
	box-shadow: 0 0 0 1px var(--p-primary-300);
}

.switcher-options {
	display: grid;
	gap: 0.65rem;
	max-height: 18rem;
	padding-right: 0.35rem;
	overflow-y: auto;
}

.switcher-option {
	display: flex;
	align-items: center;
	justify-content: space-between;
	gap: 1rem;
	width: 100%;
	padding: 0.9rem 1rem;
	border: 1px solid var(--p-surface-300);
	border-radius: 12px;
	background: var(--p-surface-0);
	color: inherit;
	text-align: left;
	cursor: pointer;
	transition:
		border-color 120ms ease,
		background-color 120ms ease,
		transform 120ms ease;
}

.switcher-option:hover {
	border-color: var(--p-primary-300);
	background: var(--p-surface-50);
	transform: translateY(-1px);
}

.switcher-option--active {
	border-color: var(--p-primary-500);
	background: color-mix(in srgb, var(--p-primary-50) 80%, white);
}

.switcher-option-state {
	font-size: 0.82rem;
	font-weight: 600;
	color: var(--p-primary-600);
}

.switcher-empty {
	padding: 1rem;
	border: 1px dashed var(--p-surface-300);
	border-radius: 12px;
	color: var(--p-surface-600);
	text-align: center;
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
}
</style>
