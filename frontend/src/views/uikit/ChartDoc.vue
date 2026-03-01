<!-- Name: main.scss
Description: This is a demo of the charts on the site with the new styling plans
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
import Chart from "primevue/chart";
import Fluid from "primevue/fluid";
import { onMounted, ref, watch } from "vue";

const { layoutConfig, isDarkTheme } = useLayout();
const lineData = ref(null);
const pieData = ref(null);
const polarData = ref(null);
const barData = ref(null);
const radarData = ref(null);
const lineOptions = ref(null);
const pieOptions = ref(null);
const polarOptions = ref(null);
const barOptions = ref(null);
const radarOptions = ref(null);

onMounted(() => {
	setColorOptions();
});

function setColorOptions() {
	const documentStyle = getComputedStyle(document.documentElement);
	const textColor = "#334155"; // Slate-700
	const textColorSecondary = "#94a3b8"; // Slate-400
	const surfaceBorder = "transparent"; // Remove grid lines for a cleaner look

	lineData.value = {
		labels: ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul"],
		datasets: [
			{
				label: "Primary Metrics",
				data: [65, 59, 80, 81, 56, 55, 40],
				fill: true,
				backgroundColor: "rgba(99, 102, 241, 0.1)", // Light Indigo tint
				borderColor: "#6366f1",
				tension: 0.4,
				pointRadius: 2,
				pointBackgroundColor: "#6366f1",
				borderWidth: 3,
			},
			{
				label: "Secondary Metrics",
				data: [28, 48, 40, 19, 86, 27, 90],
				fill: true,
				backgroundColor: "rgba(148, 163, 184, 0.05)", // Very faint Slate tint
				borderColor: "#94a3b8", // Sophisticated Slate grey
				borderDash: [5, 5], // MODERN: Dashed line for the "second guy" look
				tension: 0.4,
				pointRadius: 2,
				pointBackgroundColor: "#94a3b8",
				borderWidth: 2,
			},
		],
	};

	lineOptions.value = {
		plugins: { legend: { display: false } }, // MODERN: Minimalist (remove legend)
		scales: {
			x: {
				grid: { display: false },
				ticks: { color: textColorSecondary },
			},
			y: {
				grid: { color: "#f1f5f9" },
				border: { display: false },
				ticks: { color: textColorSecondary },
			},
		},
	};

	barData.value = {
		labels: ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul"],
		datasets: [
			{
				label: "Sales",
				backgroundColor: "#3b82f6",
				hoverBackgroundColor: "#2563eb",
				borderRadius: 8, // MODERN: Rounded bars
				data: [65, 59, 80, 81, 56, 55, 40],
				barThickness: 20,
			},
		],
	};

	barOptions.value = {
		plugins: { legend: { display: false } },
		scales: {
			x: {
				grid: { display: false },
				ticks: { color: textColorSecondary },
			},
			y: {
				grid: { display: false },
				border: { display: false },
				ticks: { display: false },
			},
		},
	};

	pieData.value = {
		labels: ["A", "B", "C"],
		datasets: [
			{
				data: [540, 325, 702],
				backgroundColor: [
					documentStyle.getPropertyValue("--p-indigo-500"),
					documentStyle.getPropertyValue("--p-purple-500"),
					documentStyle.getPropertyValue("--p-teal-500"),
				],
				hoverBackgroundColor: [
					documentStyle.getPropertyValue("--p-indigo-400"),
					documentStyle.getPropertyValue("--p-purple-400"),
					documentStyle.getPropertyValue("--p-teal-400"),
				],
			},
		],
	};

	pieOptions.value = {
		plugins: {
			legend: {
				labels: {
					usePointStyle: true,
					color: textColor,
				},
			},
		},
	};

	polarData.value = {
		datasets: [
			{
				data: [11, 16, 7, 3],
				backgroundColor: [
					documentStyle.getPropertyValue("--p-indigo-500"),
					documentStyle.getPropertyValue("--p-purple-500"),
					documentStyle.getPropertyValue("--p-teal-500"),
					documentStyle.getPropertyValue("--p-orange-500"),
				],
				label: "My dataset",
			},
		],
		labels: ["Indigo", "Purple", "Teal", "Orange"],
	};

	polarOptions.value = {
		plugins: {
			legend: {
				labels: {
					color: textColor,
				},
			},
		},
		scales: {
			r: {
				grid: {
					color: surfaceBorder,
				},
			},
		},
	};

	radarData.value = {
		labels: [
			"Eating",
			"Drinking",
			"Sleeping",
			"Designing",
			"Coding",
			"Cycling",
			"Running",
		],
		datasets: [
			{
				label: "My First dataset",
				borderColor: documentStyle.getPropertyValue("--p-indigo-400"),
				pointBackgroundColor: documentStyle.getPropertyValue("--p-indigo-400"),
				pointBorderColor: documentStyle.getPropertyValue("--p-indigo-400"),
				pointHoverBackgroundColor: textColor,
				pointHoverBorderColor: documentStyle.getPropertyValue("--p-indigo-400"),
				data: [65, 59, 90, 81, 56, 55, 40],
			},
			{
				label: "My Second dataset",
				borderColor: documentStyle.getPropertyValue("--p-purple-400"),
				pointBackgroundColor: documentStyle.getPropertyValue("--p-purple-400"),
				pointBorderColor: documentStyle.getPropertyValue("--p-purple-400"),
				pointHoverBackgroundColor: textColor,
				pointHoverBorderColor: documentStyle.getPropertyValue("--p-purple-400"),
				data: [28, 48, 40, 19, 96, 27, 100],
			},
		],
	};

	radarOptions.value = {
		plugins: {
			legend: {
				labels: {
					fontColor: textColor,
				},
			},
		},
		scales: {
			r: {
				grid: {
					color: textColorSecondary,
				},
			},
		},
	};
}

watch(
	[() => layoutConfig.primary, () => layoutConfig.surface, isDarkTheme],
	() => {
		setColorOptions();
	},
	{ immediate: true },
);
</script>

<template>
	<div class="dashboard-grid">
		<div class="card chart-card">
			<div class="font-semibold text-xl mb-4">Linear</div>
			<Chart
				type="line"
				:data="lineData"
				:options="lineOptions"
				class="chart-container"
			/>
		</div>

		<div class="card chart-card">
			<div class="font-semibold text-xl mb-4">Bar</div>
			<Chart
				type="bar"
				:data="barData"
				:options="barOptions"
				class="chart-container"
			/>
		</div>

		<div class="card chart-card center-chart">
			<div class="font-semibold text-xl mb-4">Pie</div>
			<Chart
				type="pie"
				:data="pieData"
				:options="pieOptions"
				class="pie-container"
			/>
		</div>

		<div class="card chart-card center-chart">
			<div class="font-semibold text-xl mb-4">Doughnut</div>
			<Chart
				type="doughnut"
				:data="pieData"
				:options="pieOptions"
				class="pie-container"
			/>
		</div>

		<div class="card chart-card center-chart">
			<div class="font-semibold text-xl mb-4">Polar Area</div>
			<Chart
				type="polarArea"
				:data="polarData"
				:options="polarOptions"
				class="pie-container"
			/>
		</div>
	</div>
</template>

<style scoped>
.dashboard-grid {
	display: grid !important;
	grid-template-columns: 1fr !important;
	gap: 1.5rem !important;
	padding: 1rem;
	background-color: #f8fafc; /* Ultra light slate background */
}

@media (min-width: 1024px) {
	.dashboard-grid {
		grid-template-columns: 1fr 1fr !important;
	}
}

.chart-card {
	background: #ffffff !important;
	border: 1px solid #e2e8f0 !important;
	padding: 2rem !important;
	border-radius: 20px !important; /* Extra rounded */
	box-shadow: 0 10px 15px -3px rgba(0, 0, 0, 0.05) !important;
	transition: transform 0.2s ease-in-out;
}

.chart-card:hover {
	transform: translateY(-4px); /* Modern interaction */
}

.font-semibold {
	color: #1e293b !important; /* Darker Slate */
	letter-spacing: -0.025em; /* Tighter modern typography */
	font-size: 1.25rem;
}

.chart-container {
	height: 300px !important;
}

.pie-container {
	max-width: 260px !important;
}
</style>
