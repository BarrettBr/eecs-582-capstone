<script setup lang="ts">
import { ref, onMounted } from "vue";
import Card from "primevue/card";

</script>

<template>
	<div class="about">
		<h1 class="title">About Page</h1>

		<div class="grid">
			<!-- System Status -->
			<Card class="card">
				<template #content>
					<p class="text"> The about section of the page</p>
				</template>
			</Card>
		</div>
	</div>
</template>

<style scoped>
.about {
	padding: 2rem;
}

.title {
	margin-bottom: 1.5rem;
	color:black;
}

.text {
	color: white;
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

