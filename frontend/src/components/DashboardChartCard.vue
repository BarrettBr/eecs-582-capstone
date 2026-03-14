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
import Chart from "primevue/chart";
import Card from "primevue/card";

const props = defineProps<{
	title: string;
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
</script>

<template>
	<Card class="card col-span-2">
		<template #title>{{ props.title }}</template>
		<template #content>
			<Chart
				type="line"
				:data="props.data"
				:options="props.options"
				style="height: 300px"
			/>
		</template>
	</Card>
</template>

<style scoped>
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
