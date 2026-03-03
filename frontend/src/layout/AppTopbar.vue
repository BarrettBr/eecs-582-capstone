<!-- Name: Topbar
Description: This is the bar that stretches across the screen and has the app name (sentinel) and the user icon
Programmers: Adam Berry 
Creation Date: 2/25
Revision Dates: 
Preconditions: Not Relevant
Postconditions: Not Relevant
Error Types: Not Relevant
Invariants: Dependencies described in /Docs/web.md
Known Faults: None
-->
<script setup lang="ts">
import { ref, onMounted, onUnmounted } from "vue";
import { useRouter } from "vue-router";
import Avatar from "primevue/avatar";
import Button from "primevue/button";
import Menu from "primevue/menu";
import { useLayoutStore } from "@/stores/layout";

const layout = useLayoutStore();
const router = useRouter();

const userMenu = ref<InstanceType<typeof Menu> | null>(null);

const isMobile = ref(window.innerWidth < 1024);

function handleResize() {
	isMobile.value = window.innerWidth < 1024;
}

onMounted(() => {
	window.addEventListener("resize", handleResize);
});

onUnmounted(() => {
	window.removeEventListener("resize", handleResize);
});

const userMenuItems = [
	{
		label: "Account",
		items: [
			{
				label: "Profile",
				icon: "pi pi-user",
				command: () => {
					/* TODO: navigate to profile */
				},
			},
		],
	},
];

function toggleUserMenu(event: Event) {
	userMenu.value?.toggle(event);
}
</script>

<template>
	<div class="layout-topbar">
		<!-- Left: hamburger -->
		<Button
			v-if="isMobile"
			icon="pi pi-bars"
			text
			rounded
			class="topbar-menu-btn"
			aria-label="Toggle menu"
			@click="layout.toggleSidebar()"
		/>

		<!-- Spacer -->
		<div class="topbar-spacer" />

		<!-- Right: user avatar -->
		<div class="topbar-right">
			<Button
				text
				rounded
				class="topbar-user-btn"
				aria-label="User menu"
				aria-haspopup="true"
				@click="toggleUserMenu"
			>
				<Avatar
					:label="'U'"
					shape="circle"
					size="normal"
					class="topbar-avatar"
				/>
				<span class="topbar-username hidden-mobile">{{ "User" }}</span>
				<i class="pi pi-chevron-down hidden-mobile" style="font-size: 0.7rem" />
			</Button>

			<Menu ref="userMenu" :model="userMenuItems" popup />
		</div>
	</div>
</template>

<style scoped>
.layout-topbar {
	display: flex;
	align-items: center;
	padding: 0 1.25rem;
	height: 4rem;
	background-color: var(--p-surface-0);
	border-bottom: 1px solid var(--p-surface-200);
	position: sticky;
	top: 0;
	z-index: 100;
}

.topbar-menu-btn {
	color: var(--p-surface-600) !important;
}

.topbar-spacer {
	flex: 1;
}

.topbar-right {
	display: flex;
	align-items: center;
	gap: 0.5rem;
}

.topbar-user-btn {
	display: flex;
	align-items: center;
	gap: 0.5rem;
	padding: 0.25rem 0.5rem !important;
	color: var(--p-surface-700) !important;
	border-radius: var(--p-border-radius-md) !important;
}

.topbar-avatar {
	background-color: var(--p-primary-color);
	color: var(--p-primary-contrast-color);
	font-size: 0.85rem;
	font-weight: 600;
}

.topbar-username {
	font-size: 0.875rem;
	font-weight: 500;
	color: var(--p-surface-700);
}

@media (max-width: 640px) {
	.hidden-mobile {
		display: none;
	}
}
</style>
