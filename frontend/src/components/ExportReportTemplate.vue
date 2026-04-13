<script setup lang="ts">
type SummaryMetricTone = "warning" | "info" | "alert" | "danger";
type AlertSeverity = "Critical" | "Warning" | "Info";

export type ReportTemplateMetric = {
	label: string;
	value: string;
	tone: SummaryMetricTone;
};

export type ReportTemplateHighlight = {
	title: string;
	detail: string;
};

export type ReportTemplateAlert = {
	time: string;
	severity: AlertSeverity;
	description: string;
};

const props = defineProps<{
	scopeLabel: string;
	windowLabel: string;
	generatedAtLabel: string;
	summaryMetrics: ReportTemplateMetric[];
	highlights: ReportTemplateHighlight[];
	alerts: ReportTemplateAlert[];
	temperaturePoints: number[];
	temperatureAxisLabels: string[];
	valveBars: number[];
	valveAxisLabels: string[];
	minTemperatureLabel: string;
	avgTemperatureLabel: string;
	maxTemperatureLabel: string;
	criticalAlertsLabel: string;
	valveOpenDurationLabel: string;
	averageOpenRateLabel: string;
	highValvePeriodsLabel: string;
	valveAverageSummaryLabel: string;
}>();

function metricToneClass(tone: SummaryMetricTone): string {
	return `tone-${tone}`;
}

function alertSeverityClass(severity: AlertSeverity): string {
	return `severity-${severity.toLowerCase()}`;
}

function buildTemperaturePath(values: number[]): string {
	if (values.length === 0) {
		return "";
	}

	const width = 100;
	const height = 100;
	const min = Math.min(...values);
	const max = Math.max(...values);
	const range = Math.max(1, max - min);

	return values
		.map((value, index) => {
			const x = (index / Math.max(1, values.length - 1)) * width;
			const y = height - ((value - min) / range) * height;
			return `${index === 0 ? "M" : "L"} ${x.toFixed(2)} ${y.toFixed(2)}`;
		})
		.join(" ");
}

const temperaturePath = buildTemperaturePath(props.temperaturePoints);
const peakTemperature = props.temperaturePoints.length
	? Math.max(...props.temperaturePoints)
	: 0;
</script>

<template>
	<section class="report-shell">
		<header class="report-header">
			<div class="brand-row">
				<i class="pi pi-chart-bar logo-icon" />
				<span class="brand-text">Sentinel</span>
			</div>

			<div class="header-copy">
				<p class="eyebrow">Activity Summary: {{ props.windowLabel }}</p>
				<h1>Summary Report</h1>
				<p class="subtitle">
					Live export layout for {{ props.scopeLabel }} reporting. Generated
					{{ props.generatedAtLabel }}.
				</p>
			</div>
		</header>

		<section class="panel">
			<div class="section-title">Quick Summary</div>

			<div class="metric-grid">
				<article
					v-for="metric in props.summaryMetrics"
					:key="metric.label"
					class="metric-card"
					:class="metricToneClass(metric.tone)"
				>
					<div class="metric-accent" />
					<div class="metric-label">{{ metric.label }}</div>
					<div class="metric-value">{{ metric.value }}</div>
				</article>
			</div>

			<div class="section-subtitle">Key Highlights</div>
			<div class="highlight-list">
				<div
					v-for="item in props.highlights"
					:key="item.title"
					class="highlight-row"
				>
					<div class="highlight-dot" />
					<p>
						<strong>{{ item.title }}:</strong>
						{{ item.detail }}
					</p>
				</div>
			</div>
		</section>

		<section class="dual-grid">
			<article class="panel">
				<div class="section-title">Temperature Trends</div>
				<div class="chart-card">
					<div class="chart-meta">
						<span>{{ props.temperatureAxisLabels[0] }}</span>
						<span>{{ props.temperatureAxisLabels[1] }}</span>
						<span>{{ props.temperatureAxisLabels[2] }}</span>
					</div>
					<div class="temperature-chart">
						<div class="grid-line top" />
						<div class="grid-line mid" />
						<div class="grid-line bottom" />
						<svg
							class="line-chart"
							viewBox="0 0 100 100"
							preserveAspectRatio="none"
							aria-hidden="true"
						>
							<path :d="temperaturePath" pathLength="100" />
						</svg>
						<div class="peak-marker">
							<span class="peak-value">{{ peakTemperature.toFixed(1) }} F</span>
							<span class="peak-time">{{ props.temperatureAxisLabels[2] }}</span>
						</div>
					</div>
				</div>

				<div class="stat-strip">
					<div class="mini-stat">
						<span class="mini-label">Min Temperature</span>
						<strong>{{ props.minTemperatureLabel }}</strong>
					</div>
					<div class="mini-stat">
						<span class="mini-label">Avg Temperature</span>
						<strong>{{ props.avgTemperatureLabel }}</strong>
					</div>
					<div class="mini-stat">
						<span class="mini-label">Max Temperature</span>
						<strong>{{ props.maxTemperatureLabel }}</strong>
					</div>
				</div>
			</article>

			<article class="panel">
				<div class="section-title">Valve Activity Summary</div>
				<div class="chart-card">
					<div class="bar-chart">
						<div
							v-for="(value, index) in props.valveBars"
							:key="index"
							class="bar-shell"
						>
							<div class="bar-fill" :style="{ height: `${value}%` }" />
						</div>
					</div>
					<div class="chart-axis">
						<span>{{ props.valveAxisLabels[0] }}</span>
						<span>{{ props.valveAxisLabels[1] }}</span>
						<span>{{ props.valveAxisLabels[2] }}</span>
					</div>
				</div>

				<div class="bullet-list">
					<p>{{ props.highValvePeriodsLabel }}</p>
					<p><strong>{{ props.valveAverageSummaryLabel }}</strong></p>
				</div>
			</article>
		</section>

		<section class="panel">
			<div class="section-title">Alerts Summary</div>

			<div class="alerts-grid">
				<div class="table-shell">
					<table>
						<thead>
							<tr>
								<th>Time</th>
								<th>Severity</th>
								<th>Description</th>
							</tr>
						</thead>
						<tbody>
							<tr
								v-for="row in props.alerts"
								:key="`${row.time}-${row.description}`"
							>
								<td>{{ row.time }}</td>
								<td>
									<span
										class="severity-chip"
										:class="alertSeverityClass(row.severity)"
									>
										{{ row.severity }}
									</span>
								</td>
								<td>{{ row.description }}</td>
							</tr>
						</tbody>
					</table>
				</div>

				<div class="side-cards">
					<div class="side-card">
						<div class="side-label">Critical Alerts</div>
						<div class="side-value">{{ props.criticalAlertsLabel }}</div>
					</div>

					<div class="side-card">
						<div class="side-label">Valve Open Duration</div>
						<div class="progress-row">
							<span class="progress-value">{{ props.valveOpenDurationLabel }}</span>
							<div class="progress-track">
								<div
									class="progress-fill progress-fill-blue"
									:style="{ width: props.averageOpenRateLabel }"
								/>
							</div>
						</div>
					</div>

					<div class="side-card">
						<div class="side-label">Average Open Rate</div>
						<div class="progress-row">
							<span class="progress-value">{{ props.averageOpenRateLabel }}</span>
							<div class="progress-track">
								<div
									class="progress-fill progress-fill-teal"
									:style="{ width: props.averageOpenRateLabel }"
								/>
							</div>
						</div>
					</div>

					<button type="button" class="cta-button">View All Alerts</button>
				</div>
			</div>
		</section>
	</section>
</template>

<style scoped>
.report-shell {
	max-width: 1040px;
	margin: 0 auto;
	padding: 2rem;
	background:
		radial-gradient(
			circle at top right,
			rgba(147, 197, 253, 0.2),
			transparent 30%
		),
		linear-gradient(180deg, #f4f7fb 0%, #eef3f8 100%);
	color: #1e2a44;
}

.report-header,
.panel {
	background: rgba(255, 255, 255, 0.9);
	border: 1px solid #d8e0ec;
	border-radius: 20px;
	box-shadow: 0 8px 20px rgba(36, 57, 89, 0.08);
}

.report-header {
	padding: 2rem;
	background: linear-gradient(180deg, #edf3fb 0%, #f7fafe 100%);
}

.brand-row {
	display: flex;
	align-items: center;
	gap: 0.75rem;
	margin-bottom: 1.75rem;
}

.brand-text {
	font-size: 1.8rem;
	font-weight: 700;
	letter-spacing: -0.02em;
}

.eyebrow {
	margin: 0 0 0.5rem;
	font-size: 0.95rem;
	font-weight: 700;
	letter-spacing: 0.06em;
	text-transform: uppercase;
	color: #5670aa;
}

h1 {
	margin: 0;
	font-size: clamp(2.25rem, 5vw, 3.7rem);
	line-height: 1;
	letter-spacing: -0.03em;
}

.subtitle {
	margin: 0.85rem 0 0;
	max-width: 36rem;
	font-size: 1rem;
	line-height: 1.6;
	color: #5f6f8b;
}

.panel {
	margin-top: 1.25rem;
	padding: 1.5rem;
}

.section-title {
	margin-bottom: 1.1rem;
	font-size: 1.55rem;
	font-weight: 700;
	letter-spacing: -0.02em;
}

.section-subtitle {
	margin: 1.5rem 0 0.9rem;
	font-size: 1.15rem;
	font-weight: 700;
}

.metric-grid,
.dual-grid,
.alerts-grid {
	display: grid;
	gap: 1rem;
}

.metric-grid {
	grid-template-columns: repeat(4, minmax(0, 1fr));
}

.metric-card {
	position: relative;
	padding: 1.25rem;
	border-radius: 16px;
	border: 1px solid #dde5f0;
	background: #f7f9fc;
}

.metric-accent {
	width: 44px;
	height: 6px;
	margin-bottom: 0.9rem;
	border-radius: 999px;
	background: #7a8caf;
}

.tone-warning .metric-accent {
	background: #d89234;
}

.tone-info .metric-accent {
	background: #4c7bdd;
}

.tone-alert .metric-accent {
	background: #d9a23f;
}

.tone-danger .metric-accent {
	background: #d85656;
}

.metric-label {
	font-size: 0.95rem;
	color: #4d5f7d;
}

.metric-value {
	margin-top: 0.6rem;
	font-size: 2rem;
	font-weight: 800;
	letter-spacing: -0.03em;
}

.highlight-list {
	padding: 0.35rem 0;
	border-radius: 16px;
	background: #f5f8fd;
	border: 1px solid #dde5f0;
}

.highlight-row {
	display: flex;
	align-items: flex-start;
	gap: 0.9rem;
	padding: 0.9rem 1rem;
}

.highlight-row + .highlight-row {
	border-top: 1px solid #e1e8f2;
}

.highlight-row p {
	margin: 0;
	line-height: 1.6;
	color: #50627f;
}

.highlight-dot {
	width: 10px;
	height: 10px;
	margin-top: 0.45rem;
	border-radius: 999px;
	flex: 0 0 auto;
	background: #5f8fe2;
}

.dual-grid {
	grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
}

.chart-card {
	padding: 1rem;
	border-radius: 16px;
	background: #f7f9fc;
	border: 1px solid #dde5f0;
}

.chart-meta,
.chart-axis {
	display: flex;
	justify-content: space-between;
	font-size: 0.82rem;
	color: #6a7b97;
}

.chart-meta {
	margin-bottom: 0.75rem;
}

.temperature-chart {
	position: relative;
	height: 230px;
	border-radius: 14px;
	background: linear-gradient(
		180deg,
		rgba(84, 126, 214, 0.08) 0%,
		rgba(84, 126, 214, 0.02) 100%
	);
	overflow: hidden;
}

.grid-line {
	position: absolute;
	left: 0;
	right: 0;
	height: 1px;
	background: rgba(129, 147, 182, 0.26);
}

.grid-line.top {
	top: 18%;
}

.grid-line.mid {
	top: 50%;
}

.grid-line.bottom {
	top: 82%;
}

.line-chart {
	position: absolute;
	inset: 0;
	width: 100%;
	height: 100%;
}

.logo-icon {
	font-size: 1.5rem;
	color: var(--p-primary-color);
}

.line-chart path {
	fill: none;
	stroke: #4f82df;
	stroke-width: 2;
	filter: drop-shadow(0 4px 8px rgba(79, 130, 223, 0.2));
}

.peak-marker {
	position: absolute;
	right: 1rem;
	top: 1.1rem;
	display: flex;
	flex-direction: column;
	align-items: flex-end;
	font-size: 0.82rem;
	color: #1f2d49;
}

.peak-value {
	font-size: 1.1rem;
	font-weight: 700;
}

.stat-strip {
	display: grid;
	grid-template-columns: repeat(3, minmax(0, 1fr));
	gap: 0.75rem;
	margin-top: 1rem;
}

.mini-stat {
	padding: 0.85rem;
	border-radius: 14px;
	background: #f7f9fc;
	border: 1px solid #dde5f0;
}

.mini-label {
	display: block;
	margin-bottom: 0.35rem;
	font-size: 0.8rem;
	color: #64748e;
}

.mini-stat strong {
	font-size: 1.25rem;
	letter-spacing: -0.02em;
}

.bar-chart {
	display: flex;
	align-items: flex-end;
	gap: 0.45rem;
	height: 230px;
	padding: 0.75rem 0;
}

.bar-shell {
	display: flex;
	align-items: flex-end;
	flex: 1 1 0;
	height: 100%;
	padding: 0 1px;
	border-radius: 999px 999px 4px 4px;
	background: linear-gradient(
		180deg,
		rgba(191, 207, 233, 0.55),
		rgba(191, 207, 233, 0.25)
	);
}

.bar-fill {
	width: 100%;
	border-radius: 999px 999px 4px 4px;
	background: linear-gradient(180deg, #7fb2f2 0%, #4478da 100%);
}

.chart-axis {
	margin-top: 0.35rem;
}

.bullet-list {
	margin-top: 1rem;
}

.bullet-list p {
	position: relative;
	margin: 0.65rem 0 0;
	padding-left: 1rem;
	line-height: 1.6;
	color: #526482;
}

.bullet-list p::before {
	content: "";
	position: absolute;
	left: 0;
	top: 0.7rem;
	width: 6px;
	height: 6px;
	border-radius: 999px;
	background: #9aa9c3;
}

.alerts-grid {
	grid-template-columns: minmax(0, 1.6fr) minmax(280px, 0.9fr);
	align-items: start;
}

.table-shell {
	overflow-x: auto;
	border: 1px solid #dde5f0;
	border-radius: 16px;
	background: #f7f9fc;
}

table {
	width: 100%;
	border-collapse: collapse;
}

th,
td {
	padding: 0.9rem 1rem;
	text-align: left;
	border-bottom: 1px solid #e3e9f3;
}

th {
	font-size: 0.85rem;
	font-weight: 700;
	color: #4f6282;
	background: #edf3fb;
}

tbody tr:last-child td {
	border-bottom: 0;
}

td {
	font-size: 0.95rem;
	color: #31415f;
}

.severity-chip {
	display: inline-flex;
	padding: 0.35rem 0.6rem;
	border-radius: 999px;
	font-size: 0.78rem;
	font-weight: 700;
	letter-spacing: 0.02em;
}

.severity-critical {
	background: #fbe4e4;
	color: #ba3d3d;
}

.severity-warning {
	background: #faedd8;
	color: #b67a17;
}

.severity-info {
	background: #e3edf9;
	color: #3f68b1;
}

.side-cards {
	display: grid;
	gap: 0.85rem;
}

.side-card {
	padding: 1rem;
	border-radius: 16px;
	border: 1px solid #dde5f0;
	background: #f7f9fc;
}

.side-label {
	font-size: 0.9rem;
	color: #5d7090;
}

.side-value {
	margin-top: 0.45rem;
	font-size: 2rem;
	font-weight: 800;
	letter-spacing: -0.03em;
}

.progress-row {
	display: grid;
	grid-template-columns: auto 1fr;
	gap: 0.85rem;
	align-items: center;
	margin-top: 0.7rem;
}

.progress-value {
	font-size: 1.45rem;
	font-weight: 700;
	letter-spacing: -0.03em;
}

.progress-track {
	height: 14px;
	border-radius: 999px;
	background: #d9e4f2;
	overflow: hidden;
}

.progress-fill {
	height: 100%;
	border-radius: 999px;
}

.progress-fill-blue {
	background: linear-gradient(90deg, #78aeea 0%, #4c87d6 100%);
}

.progress-fill-teal {
	background: linear-gradient(90deg, #64b8c0 0%, #3e97a1 100%);
}

.cta-button {
	padding: 0.95rem 1.1rem;
	border: 0;
	border-radius: 14px;
	background: linear-gradient(135deg, #5c95df 0%, #427bcf 100%);
	color: #fff;
	font: inherit;
	font-weight: 700;
	cursor: default;
}

@media (max-width: 960px) {
	.metric-grid,
	.dual-grid,
	.alerts-grid {
		grid-template-columns: 1fr;
	}
}

@media (max-width: 700px) {
	.report-shell {
		padding: 1rem;
	}

	.report-header,
	.panel {
		padding: 1rem;
		border-radius: 16px;
	}

	.brand-text {
		font-size: 1.4rem;
	}

	.stat-strip {
		grid-template-columns: 1fr;
	}

	th,
	td {
		padding: 0.75rem;
	}

	.progress-row {
		grid-template-columns: 1fr;
	}
}
</style>
