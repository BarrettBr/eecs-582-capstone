<!-- Name: Sidebar
Description: This is the sidebar for the application that lets you navigate
Programmers: Adam Berry 
Creation Date: 2/25
Revision Dates: 
Preconditions: Not Relevant
Postconditions: Not Relevant
Error Types: Not Relevant
Invariants: Dependencies described in /Docs/web.md
Known Faults: None
-->


<script setup>
import { useLayout } from './layout';
import { onBeforeUnmount, ref, watch } from 'vue';
import { useRoute } from 'vue-router';
import AppMenu from './AppMenu.vue';

const { layoutState, isDesktop, hasOpenOverlay } = useLayout();
const route = useRoute();
const sidebarRef = ref(null);
let outsideClickListener = null;

</script>

<template>
    <div ref="sidebarRef" class="layout-sidebar">
		<AppMenu />
    </div>
</template>
