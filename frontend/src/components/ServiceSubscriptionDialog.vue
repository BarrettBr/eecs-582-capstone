<!-- Name: ServiceSubscriptionDialog.vue
Description: Hosts the service multi-select dialog used to update overview dashboard subscriptions.
Programmers: Adam Berry, Barrett Brown
Creation Date: 2/14
Revision Dates: Adam Berry 2/14, Adam Berry 2/15, Barrett Brown 3/1, Adam Berry 3/1, Barrett Brown 3/14
Revision Notes: Barrett Brown 3/14 split the larger dashboard view into focused components.
Preconditions: The parent passes the current service selection and available service options.
Postconditions: Not Relevant
Error Types: Not Relevant
Invariants: Dependencies described in /Docs/web.md
Known Faults: None
-->
<script setup lang="ts">
import { computed, ref, watch } from "vue";
import Dialog from "primevue/dialog";
import Button from "primevue/button";

const props = defineProps<{
	visible: boolean;
	selection: string[];
	serviceOptions: Array<{
		label: string;
		value: string;
	}>;
}>();

const emit = defineEmits<{
	(event: "update:visible", value: boolean): void;
	(event: "apply", value: string[]): void;
}>();

const localSelection = ref<string[]>([]);
const searchQuery = ref("");
const selectionSet = computed(() => new Set(localSelection.value));
const selectedCountLabel = computed(() => {
	if (localSelection.value.length === 1) {
		return "1 service selected";
	}
	return `${localSelection.value.length} services selected`;
});
const filteredServiceOptions = computed(() => {
	const query = searchQuery.value.trim().toLowerCase();
	if (query === "") {
		return props.serviceOptions;
	}
	return props.serviceOptions.filter((option) =>
		option.label.toLowerCase().includes(query),
	);
});

watch(
	() => [props.visible, props.selection] as const,
	([visible, selection]) => {
		if (visible) {
			localSelection.value = selection.slice();
			searchQuery.value = "";
		}
	},
	{ immediate: true },
);

// description: Hides the dialog without changing the current dashboard subscriptions.
// input: No arguments; uses the current open dialog state.
// output: Emits the close request back to the parent view.
function closeDialog(): void {
	emit("update:visible", false);
}

// description: Applies the selected services to the dashboard subscription set.
// input: Reads the local selection cloned when the dialog opened.
// output: Emits the new subscription list and closes the dialog.
function applySelection(): void {
	emit("apply", localSelection.value.slice());
	emit("update:visible", false);
}

function toggleService(serviceValue: string): void {
	if (selectionSet.value.has(serviceValue)) {
		localSelection.value = localSelection.value.filter(
			(value) => value !== serviceValue,
		);
		return;
	}
	localSelection.value = [...localSelection.value, serviceValue];
}
</script>

<template>
	<Dialog
		:visible="props.visible"
		modal
		dismissableMask
		header="Select Services"
		:style="{ width: '30rem', maxWidth: '90vw' }"
		@update:visible="emit('update:visible', $event)"
		>
			<div class="subscribe-dialog">
				<p class="subscribe-copy">
					Choose the services you want shown on the overview dashboard.
				</p>
				<label class="subscribe-search">
					<span class="subscribe-search-label">Search Services</span>
					<input
						v-model="searchQuery"
						type="search"
						class="subscribe-search-input"
						placeholder="Search by service name"
					/>
				</label>
				<div class="subscribe-summary">{{ selectedCountLabel }}</div>
				<div class="subscribe-list" role="listbox" aria-label="Service options">
					<button
						v-for="option in filteredServiceOptions"
						:key="option.value"
						type="button"
						class="subscribe-option"
						:class="{
						'subscribe-option--selected': selectionSet.has(option.value),
					}"
					@click="toggleService(option.value)"
				>
					<span class="subscribe-option-check" aria-hidden="true">
						{{ selectionSet.has(option.value) ? "✓" : "" }}
					</span>
						<span class="subscribe-option-label">{{ option.label }}</span>
					</button>
					<div
						v-if="filteredServiceOptions.length === 0"
						class="subscribe-empty"
					>
						No services match that search.
					</div>
				</div>
				<div class="subscribe-actions">
					<Button label="Cancel" severity="secondary" text @click="closeDialog" />
					<Button label="Subscribe" @click="applySelection" />
			</div>
		</div>
	</Dialog>
</template>

<style scoped>
.subscribe-dialog {
	display: grid;
	gap: 1rem;
}

.subscribe-copy {
	margin: 0;
	color: var(--p-surface-600);
}

.subscribe-summary {
	font-size: 0.9rem;
	font-weight: 600;
	color: var(--p-surface-700);
}

.subscribe-search {
	display: grid;
	gap: 0.45rem;
}

.subscribe-search-label {
	font-size: 0.85rem;
	font-weight: 600;
	color: var(--p-surface-600);
}

.subscribe-search-input {
	width: 100%;
	padding: 0.75rem 0.9rem;
	border: 1px solid var(--p-surface-300);
	border-radius: 10px;
	background: var(--p-surface-0);
	color: inherit;
	font: inherit;
}

.subscribe-search-input:focus {
	outline: none;
	border-color: var(--p-primary-500);
	box-shadow: 0 0 0 1px var(--p-primary-300);
}

.subscribe-list {
	display: grid;
	gap: 0.65rem;
	max-height: 18rem;
	padding-right: 0.35rem;
	overflow-y: auto;
}

.subscribe-option {
	display: flex;
	align-items: center;
	gap: 0.9rem;
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

.subscribe-option:hover {
	border-color: var(--p-primary-300);
	background: var(--p-surface-50);
	transform: translateY(-1px);
}

.subscribe-option--selected {
	border-color: var(--p-primary-500);
	background: color-mix(in srgb, var(--p-primary-50) 80%, white);
}

.subscribe-option-check {
	display: inline-flex;
	align-items: center;
	justify-content: center;
	flex: 0 0 1.35rem;
	width: 1.35rem;
	height: 1.35rem;
	border: 1px solid var(--p-surface-400);
	border-radius: 0.35rem;
	font-size: 0.85rem;
	font-weight: 700;
	color: var(--p-primary-700);
	background: var(--p-surface-0);
}

.subscribe-option--selected .subscribe-option-check {
	border-color: var(--p-primary-500);
	background: var(--p-primary-100);
}

.subscribe-option-label {
	line-height: 1.35;
}

.subscribe-empty {
	padding: 1rem;
	border: 1px dashed var(--p-surface-300);
	border-radius: 12px;
	color: var(--p-surface-600);
	text-align: center;
}

.subscribe-actions {
	display: flex;
	justify-content: flex-end;
	gap: 0.75rem;
	padding-top: 0.25rem;
}
</style>
