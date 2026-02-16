<!-- Name: HomeView
Description: Used to describe the home page view
Programmers: Adam Berry 
Creation Date: 2/14
Revision Dates: Adam Berry 2/14, Adam Berry 2/15
Preconditions: Not Relevant
Postconditions: Not Relevant
Error Types: Not Relevant
Invariants: Dependencies described in /Docs/web.md
Known Faults: Depending on complexity we could just merge this with the Index component
-->

<script setup lang="ts">
import Index from "../components/Index.vue";
</script>

<template>
	<main>
		<Index />
	</main>
</template>
