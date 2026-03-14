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
import { ref, watch } from "vue";
import Dialog from "primevue/dialog";
import Button from "primevue/button";
import MultiSelect from "primevue/multiselect";

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

watch(
	() => [props.visible, props.selection] as const,
	([visible, selection]) => {
		if (visible) {
			localSelection.value = selection.slice();
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
</script>

<template>
	<Dialog
		:visible="props.visible"
		modal
		header="Select Services"
		:style="{ width: '30rem', maxWidth: '90vw' }"
		@update:visible="emit('update:visible', $event)"
	>
		<div class="subscribe-dialog">
			<p class="subscribe-copy">
				Choose the services you want shown on the overview dashboard.
			</p>
			<MultiSelect
				v-model="localSelection"
				:options="props.serviceOptions"
				optionLabel="label"
				optionValue="value"
				placeholder="Select services"
				display="chip"
				class="subscribe-select"
			/>
			<div class="subscribe-actions">
				<Button label="Cancel" severity="secondary" text @click="closeDialog" />
				<Button label="Apply" @click="applySelection" />
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

.subscribe-select {
	width: 100%;
}

.subscribe-actions {
	display: flex;
	justify-content: flex-end;
	gap: 0.75rem;
}
</style>
