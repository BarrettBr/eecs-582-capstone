<!-- Name: App Menu Item
Description: This component generates an individual secion of the nav side section
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
const props = defineProps({
    item: {
        type: Object,
        default: () => ({})
    },
    index: {
        type: Number,
        default: 0
    }
});
</script>

<template>
    <li>
        <router-link v-if="item.to && !item.items" :to="item.to" class="layout-menuitem-link" tabindex="0" >
            <i :class="item.icon" class="layout-menuitem-icon" />
            <span class="layout-menuitem-text">{{ item.label }}</span>
        </router-link>

        <a 
            v-else-if="item.url" 
            :href="item.url" 
            :target="item.target" 
            class="layout-menuitem-link" 
            tabindex="0"
        >
            <i :class="item.icon" class="layout-menuitem-icon" />
            <span class="layout-menuitem-text">{{ item.label }}</span>
        </a>
    </li>
</template>

<style lang="scss" scoped>
</style>
