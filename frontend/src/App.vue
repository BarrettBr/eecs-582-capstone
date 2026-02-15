<script setup lang="ts">
import { RouterLink } from "vue-router"
</script>

<template>
  <div class="layout">
    <header class="topbar">
      <h2 class="title">Sentinel Monitoring System</h2>

      <nav class="nav">
        <RouterLink to="/">Dashboard</RouterLink>
        <RouterLink to="/about">About</RouterLink>
      </nav>
    </header>

    <main>
	</main>
  </div>
</template>

<style scoped>
.layout {
  display: flex;
  flex-direction: column;
  min-height: 100vh;
}

.topbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1rem 2rem;
  background: #1f2937;
  color: white;
}

.title {
  margin: 0;
}

.nav a {
  margin-left: 1rem;
  text-decoration: none;
  color: white;
}

.nav a.router-link-active {
  font-weight: bold;
  border-bottom: 2px solid #42b983;
}

.content {
  flex: 1;
  padding: 2rem;
  background: #f3f4f6;
}
</style>

