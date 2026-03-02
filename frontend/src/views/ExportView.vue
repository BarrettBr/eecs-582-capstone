<script setup lang="ts">
import Card from "primevue/card";
import Button from "primevue/button";
</script>

<template>
	<main class="page-shell">
		<section class="page-hero">
			<div>
				<p class="eyebrow">Export</p>
				<h1>Export Dashboard</h1>
				<p class="page-copy">
					Manage saved reports and build exports from recent system activity.
				</p>
			</div>
		</section>

		<section class="page-grid">
			<Card class="feature-card feature-card-wide">
				<template #title>Export Window</template>
				<template #content>
					<p class="section-copy">
						Choose how much recent data to include.
					</p>
					<div class="button-row">
						<Button label="Last Hour" severity="secondary" outlined />
						<Button label="Last Day" severity="secondary" outlined />
						<Button label="Last Week" severity="secondary" outlined />
					</div>
				</template>
			</Card>

			<Card class="feature-card">
				<template #title>Output Format</template>
				<template #content>
					<p class="section-copy">
						Pick the file type you want to generate.
					</p>
					<div class="stack-list">
						<div class="list-row">
							<span>PDF summary report</span>
							<Tag value="Planned" severity="info" />
						</div>
						<div class="list-row">
							<span>Plain text export</span>
							<Tag value="Planned" severity="info" />
						</div>
						<div class="list-row">
							<span>CSV / raw data export</span>
							<Tag value="Optional" severity="secondary" />
						</div>
					</div>
				</template>
			</Card>

			<Card class="feature-card">
				<template #title>Data Scope</template>
				<template #content>
					<p class="section-copy">
						Choose which parts of the system to include.
					</p>
					<div class="stack-list">
						<div class="list-row">
							<span>Temperature events</span>
							<Tag value="Enabled" severity="success" />
						</div>
						<div class="list-row">
							<span>Valve events</span>
							<Tag value="Optional" severity="secondary" />
						</div>
						<div class="list-row">
							<span>ML alerts</span>
							<Tag value="Enabled" severity="success" />
						</div>
					</div>
				</template>
			</Card>

			<Card class="feature-card feature-card-wide">
				<template #title>Export Queue</template>
				<template #content>
					<p class="section-copy">
						Track current export requests and finished downloads here.
					</p>
					<div class="queue-shell">
						<div class="queue-row">
							<span>No export jobs yet.</span>
							<Button label="Request Export" icon="pi pi-download" disabled />
						</div>
					</div>
				</template>
			</Card>
		</section>
	</main>
</template>

<style scoped>
.page-shell {
	padding: 2rem;
}

.page-hero {
	display: flex;
	justify-content: space-between;
	align-items: flex-start;
	gap: 1rem;
	margin-bottom: 1.5rem;
	padding: 1.5rem;
	border-radius: 14px;
	background:
		linear-gradient(135deg, #f8fafc 0%, #e0f2fe 100%);
	border: 1px solid #cbd5e1;
}

.eyebrow {
	margin: 0 0 0.4rem;
	font-size: 0.8rem;
	font-weight: 700;
	text-transform: uppercase;
	letter-spacing: 0.08em;
	color: #0369a1;
}

h1 {
	margin: 0;
	font-size: 1.85rem;
	line-height: 1.15;
	color: #0f172a;
}

.page-copy,
.section-copy {
	margin: 0.75rem 0 0;
	color: #475569;
	line-height: 1.6;
}

.page-grid {
	display: grid;
	grid-template-columns: repeat(12, 1fr);
	gap: 1.25rem;
}

.feature-card {
	grid-column: span 12;
}

.button-row {
	display: flex;
	flex-wrap: wrap;
	gap: 0.75rem;
	margin-top: 1rem;
}

.stack-list {
	display: grid;
	gap: 0.9rem;
	margin-top: 1rem;
}

.list-row,
.queue-row {
	display: flex;
	justify-content: space-between;
	align-items: center;
	gap: 1rem;
	padding: 0.9rem 1rem;
	border-radius: 10px;
	background-color: #f8fafc;
	border: 1px solid #e2e8f0;
}

.queue-shell {
	margin-top: 1rem;
}

@media (min-width: 900px) {
	.feature-card {
		grid-column: span 4;
	}

	.feature-card-wide {
		grid-column: span 8;
	}
}
</style>
