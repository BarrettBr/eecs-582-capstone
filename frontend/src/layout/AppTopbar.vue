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

<script setup>
import { useLayout } from './layout';
import AppConfigurator from './AppConfigurer.vue';

</script>

<template>
    <div class="layout-topbar">
        <router-link to="/" class="layout-topbar-logo">
            <span>Sentinel</span>
        </router-link>

        <div class="layout-topbar-actions" style="margin-left: auto;">
            <button type="button" class="layout-topbar-action">
                <i class="pi pi-user"></i>
                <span>Profile</span>
            </button>
        </div>
    </div>
</template>
