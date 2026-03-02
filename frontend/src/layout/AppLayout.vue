<!-- Name: AppLayout
Description: This file is the overarching app layout that is carried across the site, it imports the side nav bits etc
Programmers: Adam Berry 
Creation Date: 2/25
Revision Dates: 3/1
Preconditions: Not Relevant
Postconditions: Not Relevant
Error Types: Not Relevant
Invariants: Dependencies described in /Docs/web.md
Known Faults: None
-->
<script setup lang="ts">
import { computed } from "vue";
import { useLayoutStore } from "@/stores/layout";
import AppTopbar from "./AppTopbar.vue";
import AppSidebar from "./AppSidebar.vue";

const layout = useLayoutStore();

const containerClass = computed(() => ({
	"layout-sidebar-active": layout.sidebarOpen,
}));
</script>

<template>
	<div class="layout-wrapper" :class="containerClass">
		<!-- Sidebar -->
		<AppSidebar />

		<!-- Overlay for mobile when sidebar is open -->
		<div
			class="layout-mask"
			:class="{ 'layout-mask-active': layout.mobileMenuOpen }"
			@click="layout.closeMobileMenu()"
		/>

		<!-- Main content -->
		<div class="layout-main-container">
			<AppTopbar />

			<div class="layout-main">
				<div class="layout-content-shell">
					<RouterView />
				</div>
			</div>
		</div>
	</div>
</template>

<style scoped>
.layout-wrapper {
	display: flex;
	min-height: 100vh;
	background-color: var(--p-surface-50);
}

.layout-main-container {
	display: flex;
	flex-direction: column;
	flex: 1;
	min-width: 0;
	/* Offset for the sidebar on desktop */
	margin-left: 260px;
	transition: margin-left 0.3s ease;
}

/* Collapsed sidebar: sidebar narrows, main expands */
.layout-sidebar-active .layout-main-container {
	margin-left: 260px;
}

.layout-main {
	flex: 1;
	padding: 1.5rem 2rem;
}

.layout-content-shell {
	width: 100%;
	max-width: 1440px;
	margin: 0 auto;
}

/* Mobile overlay */
.layout-mask {
	display: none;
	position: fixed;
	inset: 0;
	background: rgba(0, 0, 0, 0.4);
	z-index: 998;
}

@media (max-width: 991px) {
	.layout-main-container {
		margin-left: 0;
	}

	.layout-mask.layout-mask-active {
		display: block;
	}
}
</style>
