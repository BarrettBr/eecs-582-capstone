<!-- Name: App Menu
Description: This file holds the top level creator for the sidebar used for navigation
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
import { ref } from 'vue';
import AppMenuItem from './AppMenuItem.vue';

const model = ref([
    {
        items: [
            { label: 'Home', icon: 'pi pi-fw pi-home', to: '/' },
            { label: 'Dashboard', icon: 'pi pi-fw pi-th-large', to: '/Dashboard' },
            { label: 'Charts', icon: 'pi pi-fw pi-chart-line', to: '/chartdoc' }
        ]
    },
]);
</script>

<template>
    <ul class="layout-menu">
        <template v-for="(item, i) in model[0].items" :key="item.label">
            <app-menu-item :item="item" :index="i" :root="true"></app-menu-item>
        </template>
    </ul>
</template>

<style lang="scss" scoped></style>
