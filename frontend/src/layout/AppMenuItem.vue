<!-- Name: App Menu Item
Description: This component generates an individual secion of the nav side section
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
import { ref, computed } from "vue";
import { useRoute, RouterLink } from "vue-router";
import { useLayoutStore } from "@/stores/layout";

export interface MenuItem {
	label?: string;
	icon?: string;
	to?: string;
	items?: MenuItem[];
	separator?: boolean;
	roles?: string[];
}

const props = defineProps<{ item: MenuItem; depth?: number }>();

const route = useRoute();
const layout = useLayoutStore();

const open = ref(false);

const hasChildren = computed(() => !!props.item.items?.length);
const isActive = computed(() =>
	props.item.to ? route.path.startsWith(props.item.to) : false,
);

// description: Opens or closes a nested menu section.
// input: No arguments; uses the local open state.
// output: Flips the submenu visibility for this item.
function toggle() {
	open.value = !open.value;
}

// description: Cleans up navigation state after a sidebar click.
// input: No arguments; reads the shared layout store.
// output: Closes the mobile sidebar menu.
function handleNavClick() {
	// Close mobile menu on navigation
	layout.closeMobileMenu();
}
</script>

<template>
	<li v-if="item.separator" class="menu-separator" role="separator" />

	<li v-else class="menu-item" :class="{ 'menu-item-active': isActive }">
		<!-- Leaf item with a route -->
		<RouterLink
			v-if="item.to && !hasChildren"
			:to="item.to"
			class="menu-link"
			:class="{ 'menu-link-active': isActive }"
			@click="handleNavClick"
		>
			<i v-if="item.icon" :class="['menu-icon', item.icon]" />
			<span class="menu-label">{{ item.label }}</span>
		</RouterLink>

		<!-- Parent item (toggles sub-menu) -->
		<button
			v-else
			class="menu-link menu-link-toggle"
			:class="{ 'menu-link-open': open }"
			type="button"
			@click="toggle"
		>
			<i v-if="item.icon" :class="['menu-icon', item.icon]" />
			<span class="menu-label">{{ item.label }}</span>
			<i
				class="pi pi-chevron-right menu-toggle-icon"
				:class="{ rotated: open }"
			/>
		</button>

		<!-- Sub-items -->
		<Transition name="slide">
			<ul v-if="hasChildren && open" class="menu-sub">
				<AppMenuItem
					v-for="child in item.items"
					:key="child.label"
					:item="child"
					:depth="(depth ?? 0) + 1"
				/>
			</ul>
		</Transition>
	</li>
</template>

<style scoped>
.menu-item {
	list-style: none;
}

.menu-separator {
	height: 1px;
	background-color: var(--p-surface-200);
	margin: 0.5rem 1rem;
}

.menu-link {
	display: flex;
	align-items: center;
	gap: 0.75rem;
	padding: 0.65rem 1rem;
	border-radius: var(--p-border-radius-md);
	margin: 0.125rem 0.5rem;
	text-decoration: none;
	color: var(--p-surface-600);
	font-size: 0.875rem;
	font-weight: 500;
	cursor: pointer;
	background: transparent;
	border: none;
	width: calc(100% - 1rem);
	text-align: left;
	transition:
		background-color 0.15s,
		color 0.15s;
}

.menu-link:hover {
	background-color: var(--p-surface-100);
	color: var(--p-surface-900);
}

.menu-link-active {
	background-color: var(--p-primary-50);
	color: var(--p-primary-600);
	font-weight: 600;
}

.menu-link-active:hover {
	background-color: var(--p-primary-100);
}

.menu-icon {
	font-size: 1rem;
	width: 1.25rem;
	text-align: center;
	flex-shrink: 0;
}

.menu-label {
	flex: 1;
}

.menu-toggle-icon {
	font-size: 0.7rem;
	transition: transform 0.2s ease;
}

.menu-toggle-icon.rotated {
	transform: rotate(90deg);
}

.menu-sub {
	padding: 0;
	margin: 0;
	padding-left: 1rem;
}

/* Slide transition */
.slide-enter-active,
.slide-leave-active {
	transition: all 0.2s ease;
	overflow: hidden;
}

.slide-enter-from,
.slide-leave-to {
	max-height: 0;
	opacity: 0;
}

.slide-enter-to,
.slide-leave-from {
	max-height: 400px;
	opacity: 1;
}
</style>
