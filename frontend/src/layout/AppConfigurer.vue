<!-- Name: App Configurer
Description: This file contains a lot of the app config that is needed, for example colors 
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
import { updatePreset, updateSurfacePalette } from '@primeuix/themes';
import { onMounted } from 'vue';

const { layoutConfig } = useLayout();

const modernPrimary = {
    name: 'indigo',
    palette: { 50: '#eef2ff', 100: '#e0e7ff', 200: '#c7d2fe', 300: '#a5b4fc', 400: '#818cf8', 500: '#6366f1', 600: '#4f46e5', 700: '#4338ca', 800: '#3730a3', 900: '#312e81', 950: '#1e1b4b' }
};

const modernSurface = {
    name: 'slate',
    palette: { 0: '#ffffff', 50: '#f8fafc', 100: '#f1f5f9', 200: '#e2e8f0', 300: '#cbd5e1', 400: '#94a3b8', 500: '#64748b', 600: '#475569', 700: '#334155', 800: '#1e293b', 900: '#0f172a', 950: '#020617' }
};

onMounted(() => {
    layoutConfig.primary = modernPrimary.name;
    layoutConfig.surface = modernSurface.name;
    
    updatePreset({
        semantic: {
            primary: modernPrimary.palette,
            colorScheme: {
                light: {
                    primary: { color: '{primary.500}', contrastColor: '#ffffff', hoverColor: '{primary.600}', activeColor: '{primary.700}' }
                }
            }
        }
    });

    updateSurfacePalette(modernSurface.palette);
});
</script>

<template>
    <span></span>
</template>
