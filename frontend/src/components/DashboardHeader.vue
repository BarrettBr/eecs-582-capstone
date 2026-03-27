<!-- Name: DashboardHeader.vue
Description: Renders the title and subtitle block at the top of the dashboard page.
Programmers: Adam Berry, Barrett Brown
Creation Date: 2/14
Revision Dates: Adam Berry 2/14, Adam Berry 2/15, Barrett Brown 3/1, Adam Berry 3/1, Barrett Brown 3/14
Revision Notes: Barrett Brown 3/14 split the larger dashboard view into focused components.
Preconditions: The parent passes the current dashboard title and subtitle strings.
Postconditions: Not Relevant
Error Types: Not Relevant
Invariants: Dependencies described in /Docs/web.md
Known Faults: None
-->
<script setup lang="ts">
defineProps<{
	title: string;
	subtitle: string;
}>();
</script>

<template>
	<div class="dashboard-header">
		<div>
			<h1 class="dashboard-title">{{ title }}</h1>
			<p class="dashboard-subtitle">{{ subtitle }}</p>
		</div>
	</div>
</template>

<style scoped>
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
</style>
