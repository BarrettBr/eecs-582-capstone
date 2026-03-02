<!-- Name: App Menu
Description: This file holds the top level creator for the sidebar used for navigation
Programmers: Adam Berry 
Creation Date: 2/25
Revision Dates: 
- 2026-03-01, Barrett Brown: Updated /Dashboard redirect to /dashboard to match with rest of code
3/1 adam berry reformat style
Preconditions: Not Relevant
Postconditions: Not Relevant
Error Types: Not Relevant
Invariants: Dependencies described in /Docs/web.md
Known Faults: None
-->
<script setup lang="ts">
import AppMenuItem from "./AppMenuItem.vue";
import type { MenuItem } from "./AppMenuItem.vue";

const allItems: MenuItem[] = [
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
];
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
