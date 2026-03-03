<script setup lang="ts">
import Card from "primevue/card";
import Button from "primevue/button";
</script>

<template>
	<main class="page-shell">
		<section class="page-hero">
			<div>
				<p class="eyebrow">Admin</p>
				<h1>Admin Dashboard</h1>
				<p class="page-copy">
					Manage users, rules, and system inputs from one place.
				</p>
			</div>
		</section>

		<section class="page-grid">
			<Card class="feature-card">
				<template #title>Users & Permissions</template>
				<template #content>
					<p class="section-copy">
						Manage who can access the system and what they can do.
					</p>
					<div class="stack-list">
						<div class="list-row">
							<span>Admin users</span>
							<Tag value="Full Access" severity="danger" />
						</div>
						<div class="list-row">
							<span>Operators</span>
							<Tag value="Operational" severity="warn" />
						</div>
						<div class="list-row">
							<span>View-only users</span>
							<Tag value="Read Only" severity="success" />
						</div>
					</div>
					<div class="button-row">
						<Button label="Invite User" icon="pi pi-user-plus" disabled />
						<Button label="Edit Roles" severity="secondary" outlined disabled />
					</div>
				</template>
			</Card>

			<Card class="feature-card feature-card-wide">
				<template #title>Rule Management</template>
				<template #content>
					<p class="section-copy">
						Manage alert rules, thresholds, and review settings.
					</p>
					<div class="stack-list">
						<div class="list-row">
							<span>Temperature high threshold</span>
							<Tag value="Active" severity="success" />
						</div>
						<div class="list-row">
							<span>Valve flow mismatch rule</span>
							<Tag value="Draft" severity="secondary" />
						</div>
						<div class="list-row">
							<span>Custom ML label overrides</span>
							<Tag value="Planned" severity="info" />
						</div>
					</div>
					<div class="button-row">
						<Button label="Add Rule" icon="pi pi-plus" disabled />
						<Button
							label="Review Changes"
							severity="secondary"
							outlined
							disabled
						/>
					</div>
				</template>
			</Card>

			<Card class="feature-card feature-card-wide">
				<template #title>Service Inputs</template>
				<template #content>
					<p class="section-copy">Manage which data sources are active.</p>
					<div class="stack-list">
						<div class="list-row">
							<span>temp_dev simulator</span>
							<Tag value="Running" severity="success" />
						</div>
						<div class="list-row">
							<span>temp_plc source</span>
							<Tag value="Disabled" severity="secondary" />
						</div>
						<div class="list-row">
							<span>valve_plc source</span>
							<Tag value="Disabled" severity="secondary" />
						</div>
					</div>
					<div class="button-row">
						<Button label="Apply Config" icon="pi pi-cog" disabled />
						<Button
							label="Reload Sources"
							severity="secondary"
							outlined
							disabled
						/>
					</div>
				</template>
			</Card>

			<Card class="feature-card">
				<template #title>Audit Queue</template>
				<template #content>
					<p class="section-copy">
						Recent admin actions and pending approvals will appear here.
					</p>
					<div class="stack-list">
						<div class="list-row">
							<span>No pending changes.</span>
							<Tag value="Clean" severity="success" />
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
	background: linear-gradient(135deg, #f8fafc 0%, #ecfccb 100%);
	border: 1px solid #bef264;
}

.eyebrow {
	margin: 0 0 0.4rem;
	font-size: 0.8rem;
	font-weight: 700;
	text-transform: uppercase;
	letter-spacing: 0.08em;
	color: #4d7c0f;
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

.list-row {
	display: flex;
	justify-content: space-between;
	align-items: center;
	gap: 1rem;
	padding: 0.9rem 1rem;
	border-radius: 10px;
	background-color: #f8fafc;
	border: 1px solid #e2e8f0;
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
