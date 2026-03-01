<!-- Name: Sidebar
Description: This is the sidebar for the application that lets you navigate
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
import AppMenu from "./AppMenu.vue";
import Divider from "primevue/divider";
import { useLayoutStore } from "@/stores/layout";

const layout = useLayoutStore();
</script>

<template>
	<aside
		class="layout-sidebar"
		:class="{
			'sidebar-collapsed': !layout.sidebarOpen,
			'sidebar-mobile-open': layout.mobileMenuOpen,
		}"
	>
		<div class="sidebar-header">
			<div class="sidebar-logo">
				<i class="pi pi-chart-bar logo-icon" />
				<span class="logo-text">Sentinel</span>
			</div>
		</div>

		<Divider class="sidebar-divider" />

		<div class="sidebar-menu-wrap">
			<AppMenu />
		</div>

		<!-- Bottom: version info -->
		<div class="sidebar-footer">
			<span class="sidebar-version">v0.2.3</span>
			<span class="sidebar-version"><br />Favicon by Icons8</span>
		</div>
	</aside>
</template>

<style scoped>
.layout-sidebar {
	position: fixed;
	top: 0;
	left: 0;
	width: 260px;
	height: 100vh;
	background-color: var(--p-surface-0);
	border-right: 1px solid var(--p-surface-200);
	display: flex;
	flex-direction: column;
	z-index: 999;
	transition:
		transform 0.3s ease,
		width 0.3s ease;
	overflow: hidden;
}

.sidebar-header {
	padding: 1.25rem 1rem;
	flex-shrink: 0;
}

.sidebar-logo {
	display: flex;
	align-items: center;
	gap: 0.75rem;
}

.logo-icon {
	font-size: 1.5rem;
	color: var(--p-primary-color);
}

.logo-text {
	font-size: 1.1rem;
	font-weight: 700;
	color: var(--p-surface-900);
	letter-spacing: -0.02em;
}

.sidebar-divider {
	margin: 0 !important;
}

.sidebar-menu-wrap {
	flex: 1;
	overflow-y: auto;
	overflow-x: hidden;
	padding: 0.25rem 0;
}

.sidebar-footer {
	padding: 0.75rem 1.25rem;
	border-top: 1px solid var(--p-surface-200);
	flex-shrink: 0;
}

.sidebar-version {
	font-size: 0.75rem;
	color: var(--p-surface-400);
}

/* Mobile: sidebar off screen by default, slides in */
@media (max-width: 991px) {
	.layout-sidebar {
		transform: translateX(-100%);
	}

	.layout-sidebar.sidebar-mobile-open {
		transform: translateX(0);
	}
}
</style>
