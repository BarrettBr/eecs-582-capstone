<!-- Name: Index.vue
Description: The component for the home page view
Programmers: Adam Berry 
Creation Date: 2/14
Revision Dates: Adam Berry 2/14, Adam Berry 2/15
Preconditions: Not Relevant
Postconditions: Not Relevant
Error Types: Not Relevant
Invariants: Dependencies described in /Docs/web.md
Known Faults: None
-->


<script setup lang="ts">
import { ref, onMounted } from "vue";
import Button from "primevue/button";
import Card from "primevue/card";
</script>

<template>
	<div class="index">
		<p class="starter-text"> Home Page starter </p>
	</div>
</template>

<style scoped>

.starter-text{
	color: black;
}
.index {
	padding: 2rem;
}


.grid {
	display: grid;
	grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
	gap: 1.5rem;
}

.card {
	padding: 1rem;
}
</style>
