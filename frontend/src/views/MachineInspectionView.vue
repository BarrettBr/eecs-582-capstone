<script setup lang="ts">
import { onMounted } from "vue";
import { useSystemStore } from "@/stores/system";
import DashboardHeader from "@/components/DashboardHeader.vue";
import MachineInspectionMenu from "@/components/MachineInspectionMenu.vue";

const store = useSystemStore();

onMounted(() => {
	store.startStream();
});
</script>

<template>
	<main class="machine-inspection-view">
		<DashboardHeader
			title="Machine Insepction"
			subtitle="Inspect individual machine health and last-hour trends."
		/>
		<div class="inspection-panel">
			<MachineInspectionMenu />
		</div>
	</main>
</template>

<style scoped>
.machine-inspection-view {
	padding: 2rem;
}

.inspection-panel {
	margin-top: 1rem;
	border: 1px solid var(--p-surface-200);
	border-radius: 14px;
	background: var(--p-surface-0);
}
</style>
