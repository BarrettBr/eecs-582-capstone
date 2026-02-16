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
