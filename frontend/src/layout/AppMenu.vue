<!-- Name: App Menu
Description: This file holds the top level creator for the sidebar used for navigation
Programmers: Adam Berry
Creation Date: 2/25
Revision Dates:
- 2026-03-01, Barrett Brown: Updated /Dashboard redirect to /dashboard to match with rest of code
- 2026-03-14, Barrett Brown: Added dynamic service menu entries backed by the live service catalog.
- 2026-03-15, Barrett Brown: Added per-service rate labels in the sidebar service menu.
3/1 adam berry reformat style
Preconditions: Not Relevant
Postconditions: Not Relevant
Error Types: Not Relevant
Invariants: Dependencies described in /Docs/web.md
Known Faults: None
-->
<script setup lang="ts">
import { computed, onMounted } from "vue";
import { storeToRefs } from "pinia";
import { useSystemStore } from "@/stores/system";
import AppMenuItem from "./AppMenuItem.vue";
import type { MenuItem } from "./AppMenuItem.vue";

const store = useSystemStore();
const { availableServices } = storeToRefs(store);

onMounted(() => {
	store.loadAvailableServices();
});

function formatServiceRate(rate?: number): string | undefined {
	if (typeof rate !== "number" || !Number.isFinite(rate) || rate <= 0) {
		return undefined;
	}
	if (rate >= 1000) {
		return `${(rate / 1000).toFixed(rate >= 10000 ? 0 : 1)}k/s`;
	}
	return `${Math.round(rate)}/s`;
}

const serviceItems = computed<MenuItem[]>(() => {
	if (availableServices.value.length === 0) {
		return [
			{
				label: "No services yet",
				icon: "pi pi-circle-off",
				to: "/dashboard",
			},
		];
	}

	return availableServices.value.map((service) => ({
		label: service.name,
		meta: formatServiceRate(service.admitted_eps_5s),
		icon: "pi pi-chart-line",
		to: `/services/${service.name}`,
	}));
});

const allItems = computed<MenuItem[]>(() => [
	{
		label: "Dashboard",
		icon: "pi pi-home",
		to: "/dashboard",
	},
	{
		label: "Export",
		icon: "pi pi-download",
		to: "/export",
	},
	{
		label: "Admin",
		icon: "pi pi-shield",
		to: "/admin",
	},
	{ separator: true },
	{
		label: "Services",
		icon: "pi pi-bars",
		items: serviceItems.value,
	},
	{
		label: "Other",
		icon: "pi pi-bars",
		items: [
			{
				label: "About",
				icon: "pi pi-info-circle",
				to: "/about",
			},
		],
	},
]);
</script>

<template>
	<nav class="layout-menu" aria-label="Main navigation">
		<ul class="menu-root">
			<AppMenuItem
				v-for="item in allItems"
				:key="item.label ?? 'sep'"
				:item="item"
			/>
		</ul>
	</nav>
</template>

<style scoped>
.layout-menu {
	padding: 0.5rem 0;
}

.menu-root {
	list-style: none;
	padding: 0;
	margin: 0;
}
</style>
