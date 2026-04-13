<script setup lang="ts">
import { computed, ref } from "vue";
import Card from "primevue/card";
import Button from "primevue/button";
import Tag from "primevue/tag";
import jsPDF from "jspdf";
import ExportReportTemplate, {
	type ReportTemplateAlert,
	type ReportTemplateHighlight,
	type ReportTemplateMetric,
} from "@/components/ExportReportTemplate.vue";
import { API_BASE_URL } from "@/utils/apiHelper";
import type {
	ExportKind,
	ExportWindow,
	HistoryEvent,
	HistoryResponse,
	ReportResponse,
} from "@/stores/systemTypes";

type ExportFormat = "pdf" | "text" | "csv";

type ReportTemplateData = {
	scopeLabel: string;
	windowLabel: string;
	generatedAtLabel: string;
	summaryMetrics: ReportTemplateMetric[];
	highlights: ReportTemplateHighlight[];
	alerts: ReportTemplateAlert[];
	temperaturePoints: number[];
	temperatureAxisLabels: [string, string, string];
	valveBars: number[];
	valveAxisLabels: [string, string, string];
	minTemperatureLabel: string;
	avgTemperatureLabel: string;
	maxTemperatureLabel: string;
	criticalAlertsLabel: string;
	valveOpenDurationLabel: string;
	averageOpenRateLabel: string;
	highValvePeriodsLabel: string;
	valveAverageSummaryLabel: string;
};

const selectedTime = ref<ExportWindow | null>(null);
const selectedFormat = ref<ExportFormat | null>(null);
const selectedScope = ref<ExportKind | null>(null);
const isLoading = ref(false);
const lastExportMessage = ref("No export jobs yet.");
const reportPreview = ref<ReportTemplateData | null>(null);

const canRequestExport = computed(
	() => Boolean(selectedTime.value && selectedFormat.value && selectedScope.value),
);

function selectTime(window: ExportWindow) {
	selectedTime.value = window;
}

function selectFormat(format: ExportFormat) {
	selectedFormat.value = format;
}

function selectScope(scope: ExportKind) {
	selectedScope.value = scope;
}

function formatWindowLabel(window: ExportWindow): string {
	switch (window) {
		case "hour":
			return "Last Hour";
		case "day":
			return "Last Day";
		case "week":
			return "Last Week";
	}
}

function formatScopeLabel(scope: ExportKind): string {
	return scope === "temp" ? "temperature" : "valve";
}

function safeNumber(value: number | undefined | null, fallback = 0): number {
	return Number.isFinite(value) ? Number(value) : fallback;
}

function clamp(value: number, min: number, max: number): number {
	return Math.min(max, Math.max(min, value));
}

function formatTemperature(value: number): string {
	return `${safeNumber(value).toFixed(1)} F`;
}

function formatPercent(value: number): string {
	return `${safeNumber(value).toFixed(1)}%`;
}

function formatDuration(seconds: number): string {
	const hours = safeNumber(seconds) / 3600;
	if (hours >= 1) {
		const rounded = hours >= 10 ? Math.round(hours) : Math.round(hours * 10) / 10;
		return `${rounded} hrs`;
	}
	const minutes = Math.max(1, Math.round(safeNumber(seconds) / 60));
	return `${minutes} min`;
}

function formatGeneratedAt(date = new Date()): string {
	return date.toLocaleString([], {
		month: "short",
		day: "numeric",
		year: "numeric",
		hour: "numeric",
		minute: "2-digit",
	});
}

function formatEventTime(timestamp: number): string {
	return new Date(timestamp * 1000).toLocaleTimeString([], {
		hour: "numeric",
		minute: "2-digit",
	});
}

function formatAxisTime(timestamp: number | undefined, fallback: string): string {
	if (!timestamp) {
		return fallback;
	}
	return formatEventTime(timestamp);
}

function normalizeValveRate(flowRate: number | undefined): number {
	const value = safeNumber(flowRate);
	if (value <= 1) {
		return clamp(value * 100, 0, 100);
	}
	return clamp(value, 0, 100);
}

function summarizeAlertDescription(item: HistoryEvent): string {
	const anomalies = item.anomalies.filter(Boolean);
	if (anomalies.length > 0) {
		return anomalies.join(", ");
	}
	if (item.source_kind === "temp") {
		return `Temperature reading ${formatTemperature(item.temperature ?? 0)} on sensor ${item.source_id}.`;
	}
	return `Valve ${item.source_id} open rate reached ${formatPercent(normalizeValveRate(item.flow_rate))}.`;
}

function deriveSeverity(item: HistoryEvent): ReportTemplateAlert["severity"] {
	if (item.alert_count >= 2 || item.anomalies.length >= 2) {
		return "Critical";
	}
	if (item.alert_count >= 1 || item.anomalies.length >= 1) {
		return "Warning";
	}
	return "Info";
}

function sampleHistory<T>(items: T[], targetCount: number): T[] {
	if (items.length <= targetCount) {
		return items;
	}

	const sampled: T[] = [];
	for (let index = 0; index < targetCount; index += 1) {
		const itemIndex = Math.round(
			(index * (items.length - 1)) / Math.max(1, targetCount - 1),
		);
		const item = items[itemIndex];
		if (typeof item !== "undefined") {
			sampled.push(item);
		}
	}
	return sampled;
}

function buildAlertRows(historyItems: HistoryEvent[]): ReportTemplateAlert[] {
	const relevantItems = historyItems
		.filter((item) => item.alert_count > 0 || item.anomalies.length > 0)
		.sort((left, right) => right.timestamp - left.timestamp);

	const sourceItems =
		relevantItems.length > 0
			? relevantItems
			: historyItems
					.slice()
					.sort((left, right) => right.timestamp - left.timestamp)
					.slice(0, 5);

	return sourceItems.slice(0, 5).map((item) => ({
		time: formatEventTime(item.timestamp),
		severity: deriveSeverity(item),
		description: summarizeAlertDescription(item),
	}));
}

function buildHighlights(
	scope: ExportKind,
	report: ReportResponse["summary"],
	alertRows: ReportTemplateAlert[],
	temperatureValues: number[],
	valveValues: number[],
): ReportTemplateHighlight[] {
	const highlights: ReportTemplateHighlight[] = [];

	if (temperatureValues.length > 0) {
		highlights.push({
			title: "Temperature Peak",
			detail: `${formatScopeLabel(scope)} reporting saw a temperature peak of ${formatTemperature(Math.max(...temperatureValues))}.`,
		});
	}

	if (valveValues.length > 0) {
		highlights.push({
			title: "High Valve Open Rate",
			detail: `Valve activity peaked at ${formatPercent(Math.max(...valveValues))} across the selected window.`,
		});
	}

	if (report.total_alerts > 0 || alertRows.length > 0) {
		highlights.push({
			title: "Alert Activity",
			detail: `${report.total_alerts} total alerts were reported, with ${alertRows.filter((item) => item.severity === "Critical").length} marked critical in the exported summary.`,
		});
	}

	if (highlights.length === 0) {
		highlights.push({
			title: "No Major Events",
			detail: "The selected time window returned minimal activity, so the summary is using fallback labels until more readings arrive.",
		});
	}

	return highlights.slice(0, 3);
}

function buildReportTemplateData(
	scope: ExportKind,
	window: ExportWindow,
	reportResponse: ReportResponse,
	historyResponse: HistoryResponse,
): ReportTemplateData {
	const historyItems = historyResponse.items.slice().sort((left, right) => left.timestamp - right.timestamp);
	const temperatureItems = historyItems.filter((item) =>
		typeof item.temperature === "number",
	);
	const valveItems = historyItems.filter(
		(item) => typeof item.flow_rate === "number" || typeof item.is_open === "boolean",
	);

	const sampledTemperatureItems = sampleHistory(temperatureItems, 14);
	const sampledValveItems = sampleHistory(valveItems, 14);
	const temperaturePoints =
		sampledTemperatureItems.length > 0
			? sampledTemperatureItems.map((item) => safeNumber(item.temperature))
			: [0, 0, 0];
	const valveBars =
		sampledValveItems.length > 0
			? sampledValveItems.map((item) => normalizeValveRate(item.flow_rate))
			: [0, 0, 0];

	const temperatureAxisLabels = (() => {
		const first = sampledTemperatureItems[0]?.timestamp;
		const middle =
			sampledTemperatureItems[Math.floor(sampledTemperatureItems.length / 2)]?.timestamp;
		const last = sampledTemperatureItems[sampledTemperatureItems.length - 1]?.timestamp;
		return [
			formatAxisTime(first, "Start"),
			formatAxisTime(middle, "Mid"),
			formatAxisTime(last, "Latest"),
		] as [string, string, string];
	})();

	const valveAxisLabels = (() => {
		const first = sampledValveItems[0]?.timestamp;
		const middle = sampledValveItems[Math.floor(sampledValveItems.length / 2)]?.timestamp;
		const last = sampledValveItems[sampledValveItems.length - 1]?.timestamp;
		return [
			formatAxisTime(first, "Start"),
			formatAxisTime(middle, "Mid"),
			formatAxisTime(last, "Latest"),
		] as [string, string, string];
	})();

	const alertRows = buildAlertRows(historyItems);
	const criticalAlerts = alertRows.filter(
		(item) => item.severity === "Critical",
	).length;
	const highValvePeriods = valveBars.filter((value) => value >= 80).length;

	return {
		scopeLabel: formatScopeLabel(scope),
		windowLabel: formatWindowLabel(window),
		generatedAtLabel: formatGeneratedAt(),
		summaryMetrics: [
			{
				label: "Average Temperature",
				value: formatTemperature(reportResponse.summary.avg_temp),
				tone: "warning",
			},
			{
				label: "Max. Valve Open Rate",
				value: formatPercent(reportResponse.summary.max_valve_open_rate),
				tone: "info",
			},
			{
				label: "Total Alerts",
				value: String(reportResponse.summary.total_alerts),
				tone: "alert",
			},
			{
				label: "Critical Alerts",
				value: String(criticalAlerts),
				tone: "danger",
			},
		],
		highlights: buildHighlights(
			scope,
			reportResponse.summary,
			alertRows,
			temperaturePoints,
			valveBars,
		),
		alerts: alertRows,
		temperaturePoints,
		temperatureAxisLabels,
		valveBars,
		valveAxisLabels,
		minTemperatureLabel: formatTemperature(reportResponse.summary.min_temp),
		avgTemperatureLabel: formatTemperature(reportResponse.summary.avg_temp),
		maxTemperatureLabel: formatTemperature(reportResponse.summary.max_temp),
		criticalAlertsLabel: String(criticalAlerts),
		valveOpenDurationLabel: formatDuration(
			reportResponse.summary.valve_open_duration_seconds,
		),
		averageOpenRateLabel: formatPercent(
			reportResponse.summary.avg_valve_open_rate,
		),
		highValvePeriodsLabel: `${highValvePeriods} periods of high valve open rates (>80%)`,
		valveAverageSummaryLabel: `Average open rate for the ${window} is ${formatPercent(reportResponse.summary.avg_valve_open_rate)}`,
	};
}

function escapeCsvCell(value: unknown): string {
	const normalized = String(value ?? "");
	if (normalized.includes(",") || normalized.includes('"') || normalized.includes("\n")) {
		return `"${normalized.split('"').join('""')}"`;
	}
	return normalized;
}

function setFillColorTuple(doc: jsPDF, color: [number, number, number]) {
	doc.setFillColor(color[0], color[1], color[2]);
}

function setDrawColorTuple(doc: jsPDF, color: [number, number, number]) {
	doc.setDrawColor(color[0], color[1], color[2]);
}

function downloadBlob(content: BlobPart, mimeType: string, fileName: string) {
	const blob = new Blob([content], { type: mimeType });
	const downloadURL = URL.createObjectURL(blob);
	const anchor = document.createElement("a");
	anchor.href = downloadURL;
	anchor.download = fileName;
	anchor.click();
	URL.revokeObjectURL(downloadURL);
}

function addWrappedText(
	doc: jsPDF,
	text: string,
	x: number,
	y: number,
	maxWidth: number,
	lineHeight: number,
): number {
	const lines = doc.splitTextToSize(text, maxWidth) as string[];
	doc.text(lines, x, y);
	return y + lines.length * lineHeight;
}

function drawRoundedCard(
	doc: jsPDF,
	x: number,
	y: number,
	width: number,
	height: number,
	fillColor: [number, number, number],
	borderColor: [number, number, number],
) {
	setDrawColorTuple(doc, borderColor);
	setFillColorTuple(doc, fillColor);
	doc.roundedRect(x, y, width, height, 16, 16, "FD");
}

function renderPdfReport(data: ReportTemplateData, scope: ExportKind, window: ExportWindow) {
	const doc = new jsPDF({
		unit: "pt",
		format: [820, 1320],
	});

	const pageWidth = 820;
	const margin = 24;
	const contentWidth = pageWidth - margin * 2;
	const columnGap = 16;
	const halfWidth = (contentWidth - columnGap) / 2;

	doc.setFillColor(244, 247, 251);
	doc.rect(0, 0, 820, 1320, "F");

	let y = margin;

	drawRoundedCard(doc, margin, y, contentWidth, 124, [237, 243, 251], [216, 224, 236]);
	doc.setTextColor(66, 123, 207);
	doc.setFont("helvetica", "bold");
	doc.setFontSize(16);
	doc.text("Sentinel", margin + 18, y + 28);
	doc.setFontSize(10);
	doc.text(`ACTIVITY SUMMARY: ${data.windowLabel.toUpperCase()}`, margin + 18, y + 50);
	doc.setTextColor(30, 42, 68);
	doc.setFontSize(28);
	doc.text("Summary Report", margin + 18, y + 82);
	doc.setFont("helvetica", "normal");
	doc.setFontSize(10);
	doc.setTextColor(95, 111, 139);
	doc.text(
		`Live export layout for ${data.scopeLabel} reporting. Generated ${data.generatedAtLabel}.`,
		margin + 18,
		y + 104,
	);
	y += 140;

	drawRoundedCard(doc, margin, y, contentWidth, 222, [255, 255, 255], [216, 224, 236]);
	doc.setFont("helvetica", "bold");
	doc.setFontSize(18);
	doc.setTextColor(30, 42, 68);
	doc.text("Quick Summary", margin + 16, y + 28);

	const metricTop = y + 46;
	const metricGap = 10;
	const metricWidth = (contentWidth - metricGap * 3 - 32) / 4;
	const metricColors: Record<ReportTemplateMetric["tone"], [number, number, number]> = {
		warning: [216, 146, 52],
		info: [76, 123, 221],
		alert: [217, 162, 63],
		danger: [216, 86, 86],
	};

	data.summaryMetrics.forEach((metric, index) => {
		const cardX = margin + 16 + index * (metricWidth + metricGap);
		drawRoundedCard(doc, cardX, metricTop, metricWidth, 64, [247, 249, 252], [221, 229, 240]);
		setFillColorTuple(doc, metricColors[metric.tone]);
		doc.roundedRect(cardX + 10, metricTop + 10, 24, 4, 2, 2, "F");
		doc.setFont("helvetica", "normal");
		doc.setFontSize(9);
		doc.setTextColor(77, 95, 125);
		doc.text(metric.label, cardX + 10, metricTop + 28);
		doc.setFont("helvetica", "bold");
		doc.setFontSize(18);
		doc.setTextColor(30, 42, 68);
		doc.text(metric.value, cardX + 10, metricTop + 50);
	});

	doc.setFont("helvetica", "bold");
	doc.setFontSize(13);
	doc.text("Key Highlights", margin + 16, y + 132);
	drawRoundedCard(doc, margin + 16, y + 142, contentWidth - 32, 64, [245, 248, 253], [221, 229, 240]);
	let highlightY = y + 162;
	doc.setFont("helvetica", "normal");
	doc.setFontSize(10);
	doc.setTextColor(80, 98, 127);
	data.highlights.forEach((highlight, index) => {
		doc.setFillColor(95, 143, 226);
		doc.circle(margin + 28, highlightY - 4, 2.5, "F");
		doc.setFont("helvetica", "bold");
		doc.text(`${highlight.title}:`, margin + 38, highlightY);
		doc.setFont("helvetica", "normal");
		addWrappedText(
			doc,
			highlight.detail,
			margin + 105,
			highlightY,
			contentWidth - 150,
			12,
		);
		highlightY += index === data.highlights.length - 1 ? 0 : 18;
	});
	y += 246;

	const panelHeight = 284;
	drawRoundedCard(doc, margin, y, halfWidth, panelHeight, [255, 255, 255], [216, 224, 236]);
	drawRoundedCard(
		doc,
		margin + halfWidth + columnGap,
		y,
		halfWidth,
		panelHeight,
		[255, 255, 255],
		[216, 224, 236],
	);

	doc.setFont("helvetica", "bold");
	doc.setFontSize(18);
	doc.setTextColor(30, 42, 68);
	doc.text("Temperature Trends", margin + 16, y + 28);
	doc.text("Valve Activity Summary", margin + halfWidth + columnGap + 16, y + 28);

	const chartY = y + 42;
	drawRoundedCard(doc, margin + 16, chartY, halfWidth - 32, 146, [247, 249, 252], [221, 229, 240]);
	drawRoundedCard(
		doc,
		margin + halfWidth + columnGap + 16,
		chartY,
		halfWidth - 32,
		146,
		[247, 249, 252],
		[221, 229, 240],
	);

	doc.setFont("helvetica", "normal");
	doc.setFontSize(9);
	doc.setTextColor(106, 123, 151);
	doc.text(data.temperatureAxisLabels[0], margin + 24, chartY + 16);
	doc.text(data.temperatureAxisLabels[1], margin + halfWidth / 2 - 12, chartY + 16);
	doc.text(data.temperatureAxisLabels[2], margin + halfWidth - 72, chartY + 16);

	const chartInnerX = margin + 24;
	const chartInnerY = chartY + 24;
	const chartInnerWidth = halfWidth - 48;
	const chartInnerHeight = 102;
	const tempMin = Math.min(...data.temperaturePoints);
	const tempMax = Math.max(...data.temperaturePoints);
	const tempRange = Math.max(1, tempMax - tempMin);
	doc.setDrawColor(129, 147, 182);
	doc.setLineWidth(0.6);
	[0.18, 0.5, 0.82].forEach((ratio) => {
		const lineY = chartInnerY + chartInnerHeight * ratio;
		doc.line(chartInnerX, lineY, chartInnerX + chartInnerWidth, lineY);
	});
	doc.setDrawColor(79, 130, 223);
	doc.setLineWidth(2);
	data.temperaturePoints.forEach((value, index) => {
		if (index === 0) {
			return;
		}
		const previous = data.temperaturePoints[index - 1] ?? value;
		const x1 = chartInnerX + ((index - 1) / Math.max(1, data.temperaturePoints.length - 1)) * chartInnerWidth;
		const y1 = chartInnerY + chartInnerHeight - ((previous - tempMin) / tempRange) * chartInnerHeight;
		const x2 = chartInnerX + (index / Math.max(1, data.temperaturePoints.length - 1)) * chartInnerWidth;
		const y2 = chartInnerY + chartInnerHeight - ((value - tempMin) / tempRange) * chartInnerHeight;
		doc.line(x1, y1, x2, y2);
	});
	doc.setFont("helvetica", "bold");
	doc.setFontSize(10);
	doc.setTextColor(31, 45, 73);
	doc.text(
		`${Math.max(...data.temperaturePoints).toFixed(1)} F`,
		margin + halfWidth - 78,
		chartY + 44,
	);
	doc.setFont("helvetica", "normal");
	doc.setFontSize(8);
	doc.text(data.temperatureAxisLabels[2], margin + halfWidth - 74, chartY + 56);

	const statTop = y + 198;
	const statGap = 8;
	const statWidth = (halfWidth - 32 - statGap * 2) / 3;
	([
		["Min Temperature", data.minTemperatureLabel],
		["Avg Temperature", data.avgTemperatureLabel],
		["Max Temperature", data.maxTemperatureLabel],
	] as [string, string][]).forEach(([label, value], index) => {
		const statX = margin + 16 + index * (statWidth + statGap);
		drawRoundedCard(doc, statX, statTop, statWidth, 58, [247, 249, 252], [221, 229, 240]);
		doc.setFont("helvetica", "normal");
		doc.setFontSize(8);
		doc.setTextColor(100, 116, 142);
		doc.text(label, statX + 8, statTop + 18);
		doc.setFont("helvetica", "bold");
		doc.setFontSize(14);
		doc.setTextColor(30, 42, 68);
		doc.text(value, statX + 8, statTop + 40);
	});

	const valveX = margin + halfWidth + columnGap + 24;
	const valveChartWidth = halfWidth - 48;
	const valveChartHeight = 100;
	const barGap = 5;
	const barWidth =
		(valveChartWidth - barGap * (Math.max(1, data.valveBars.length) - 1)) /
		Math.max(1, data.valveBars.length);
	data.valveBars.forEach((value, index) => {
		const x = valveX + index * (barWidth + barGap);
		const barHeight = Math.max(10, (value / 100) * valveChartHeight);
		doc.setFillColor(191, 207, 233);
		doc.roundedRect(x, chartY + 20, barWidth, valveChartHeight, 4, 4, "F");
		doc.setFillColor(68, 120, 218);
		doc.roundedRect(x, chartY + 20 + valveChartHeight - barHeight, barWidth, barHeight, 4, 4, "F");
	});
	doc.setFont("helvetica", "normal");
	doc.setFontSize(9);
	doc.setTextColor(106, 123, 151);
	doc.text(data.valveAxisLabels[0], valveX, chartY + 136);
	doc.text(data.valveAxisLabels[1], valveX + valveChartWidth / 2 - 10, chartY + 136);
	doc.text(data.valveAxisLabels[2], valveX + valveChartWidth - 28, chartY + 136);
	doc.setFont("helvetica", "normal");
	doc.setFontSize(10);
	doc.setTextColor(82, 100, 130);
	doc.text(`- ${data.highValvePeriodsLabel}`, margin + halfWidth + columnGap + 16, y + 212);
	doc.setFont("helvetica", "bold");
	doc.text(`- ${data.valveAverageSummaryLabel}`, margin + halfWidth + columnGap + 16, y + 232);
	y += panelHeight + 14;

	const alertPanelHeight = 324;
	drawRoundedCard(doc, margin, y, contentWidth, alertPanelHeight, [255, 255, 255], [216, 224, 236]);
	doc.setFont("helvetica", "bold");
	doc.setFontSize(18);
	doc.setTextColor(30, 42, 68);
	doc.text("Alerts Summary", margin + 16, y + 28);

	const tableX = margin + 16;
	const tableY = y + 42;
	const tableWidth = contentWidth * 0.62;
	const sideX = tableX + tableWidth + 12;
	const sideWidth = contentWidth - 28 - tableWidth - 12;

	drawRoundedCard(doc, tableX, tableY, tableWidth, 240, [247, 249, 252], [221, 229, 240]);
	doc.setFillColor(237, 243, 251);
	doc.roundedRect(tableX, tableY, tableWidth, 26, 12, 12, "F");
	doc.setFont("helvetica", "bold");
	doc.setFontSize(9);
	doc.setTextColor(79, 98, 130);
	doc.text("Time", tableX + 10, tableY + 16);
	doc.text("Severity", tableX + 72, tableY + 16);
	doc.text("Description", tableX + 142, tableY + 16);

	const severityFill: Record<ReportTemplateAlert["severity"], [number, number, number]> = {
		Critical: [251, 228, 228],
		Warning: [250, 237, 216],
		Info: [227, 237, 249],
	};
	const severityText: Record<ReportTemplateAlert["severity"], [number, number, number]> = {
		Critical: [186, 61, 61],
		Warning: [182, 122, 23],
		Info: [63, 104, 177],
	};

	data.alerts.forEach((alert, index) => {
		const rowY = tableY + 26 + index * 42;
		doc.setDrawColor(227, 233, 243);
		doc.line(tableX, rowY, tableX + tableWidth, rowY);
		doc.setFont("helvetica", "normal");
		doc.setFontSize(9);
		doc.setTextColor(49, 65, 95);
		doc.text(alert.time, tableX + 10, rowY + 24);
		setFillColorTuple(doc, severityFill[alert.severity]);
		doc.roundedRect(tableX + 60, rowY + 11, 52, 18, 8, 8, "F");
		doc.setTextColor(
			severityText[alert.severity][0],
			severityText[alert.severity][1],
			severityText[alert.severity][2],
		);
		doc.setFont("helvetica", "bold");
		doc.setFontSize(8);
		doc.text(alert.severity, tableX + 69, rowY + 23);
		doc.setTextColor(49, 65, 95);
		doc.setFont("helvetica", "normal");
		doc.setFontSize(8.5);
		addWrappedText(doc, alert.description, tableX + 124, rowY + 19, tableWidth - 136, 10);
	});

	const sideCardHeight = 64;
	([
		["Critical Alerts", data.criticalAlertsLabel],
		["Valve Open Duration", data.valveOpenDurationLabel],
		["Average Open Rate", data.averageOpenRateLabel],
	] as [string, string][]).forEach(([label, value], index) => {
		const cardY = tableY + index * 78;
		drawRoundedCard(doc, sideX, cardY, sideWidth, sideCardHeight, [247, 249, 252], [221, 229, 240]);
		doc.setFont("helvetica", "normal");
		doc.setFontSize(9);
		doc.setTextColor(93, 112, 144);
		doc.text(label, sideX + 10, cardY + 18);
		doc.setFont("helvetica", "bold");
		doc.setFontSize(18);
		doc.setTextColor(30, 42, 68);
		doc.text(value, sideX + 10, cardY + 42);

		if (index > 0) {
			doc.setFillColor(217, 228, 242);
			doc.roundedRect(sideX + 84, cardY + 33, sideWidth - 94, 10, 5, 5, "F");
			doc.setFillColor(index === 1 ? 76 : 62, index === 1 ? 135 : 151, index === 1 ? 214 : 161);
			doc.roundedRect(
				sideX + 84,
				cardY + 33,
				((sideWidth - 94) * safeNumber(parseFloat(data.averageOpenRateLabel))) / 100,
				10,
				5,
				5,
				"F",
			);
		}
	});

	doc.setFillColor(66, 123, 207);
	doc.roundedRect(sideX, tableY + 246, sideWidth, 28, 10, 10, "F");
	doc.setFont("helvetica", "bold");
	doc.setFontSize(10);
	doc.setTextColor(255, 255, 255);
	doc.text("View All Alerts", sideX + sideWidth / 2 - 30, tableY + 264);

	doc.save(`report_${scope}_${window}.pdf`);
}

async function fetchHistory(scope: ExportKind, window: ExportWindow) {
	const response = await fetch(`${API_BASE_URL}/history/${scope}/${window}`);
	if (!response.ok) {
		throw new Error(`History request failed with ${response.status}`);
	}
	return (await response.json()) as HistoryResponse;
}

async function fetchReport(scope: ExportKind, window: ExportWindow) {
	const response = await fetch(`${API_BASE_URL}/report/${scope}/${window}`);
	if (!response.ok) {
		throw new Error(`Report request failed with ${response.status}`);
	}
	return (await response.json()) as ReportResponse;
}

async function requestExport() {
	if (!selectedScope.value || !selectedTime.value || !selectedFormat.value) {
		window.alert("Select time, format, and scope");
		return;
	}

	isLoading.value = true;
	lastExportMessage.value = "Requesting export data...";

	try {
		const scope = selectedScope.value;
		const windowRange = selectedTime.value;

		if (selectedFormat.value === "pdf") {
			const [reportResponse, historyResponse] = await Promise.all([
				fetchReport(scope, windowRange),
				fetchHistory(scope, windowRange),
			]);
			const templateData = buildReportTemplateData(
				scope,
				windowRange,
				reportResponse,
				historyResponse,
			);

			reportPreview.value = templateData;
			renderPdfReport(templateData, scope, windowRange);
			lastExportMessage.value = `Downloaded PDF summary for ${formatScopeLabel(scope)} over the ${windowRange}.`;
			return;
		}

		const historyResponse = await fetchHistory(scope, windowRange);
		if (historyResponse.items.length === 0) {
			window.alert("No row data available to export");
			lastExportMessage.value = "The selected export returned no history rows.";
			return;
		}

		if (selectedFormat.value === "csv") {
			const firstRow = historyResponse.items[0];
			if (!firstRow) {
				window.alert("No row data available to export");
				lastExportMessage.value = "The selected export returned no history rows.";
				return;
			}

			const headers = Object.keys(firstRow);
			const csvRows = [
				headers.join(","),
				...historyResponse.items.map((row) =>
					headers.map((header) => escapeCsvCell(row[header as keyof HistoryEvent])).join(","),
				),
			];
			downloadBlob(
				csvRows.join("\n"),
				"text/csv",
				`report_${scope}_${windowRange}.csv`,
			);
			lastExportMessage.value = `Downloaded CSV history for ${formatScopeLabel(scope)} over the ${windowRange}.`;
			return;
		}

		const textRows = historyResponse.items.map((row) =>
			[
				`timestamp: ${new Date(row.timestamp * 1000).toLocaleString()}`,
				`source: ${row.source_kind} ${row.source_id}`,
				`alerts: ${row.alert_count}`,
				`anomalies: ${row.anomalies.join(", ") || "none"}`,
				typeof row.temperature === "number"
					? `temperature: ${formatTemperature(row.temperature)}`
					: `flow_rate: ${formatPercent(normalizeValveRate(row.flow_rate))}`,
			].join(" | "),
		);
		downloadBlob(
			textRows.join("\n"),
			"text/plain",
			`report_${scope}_${windowRange}.txt`,
		);
		lastExportMessage.value = `Downloaded text history for ${formatScopeLabel(scope)} over the ${windowRange}.`;
	} catch (error) {
		console.error("Export failed:", error);
		window.alert("Export failed. Check the console for details.");
		lastExportMessage.value = "The export request failed.";
	} finally {
		isLoading.value = false;
	}
}
</script>

<template>
	<main class="page-shell">
		<section class="page-hero">
			<div>
				<p class="eyebrow">Export</p>
				<h1>Export Dashboard</h1>
				<p class="page-copy">
					Build live report downloads from the backend history and report
					endpoints, then preview the PDF layout before sharing it.
				</p>
			</div>

			<div class="hero-tags">
				<Tag
					:value="selectedScope ? `Scope: ${formatScopeLabel(selectedScope)}` : 'Scope: not set'"
					severity="secondary"
				/>
				<Tag
					:value="selectedTime ? `Window: ${formatWindowLabel(selectedTime)}` : 'Window: not set'"
					severity="contrast"
				/>
			</div>
		</section>

		<section class="page-grid">
			<Card class="feature-card feature-card-wide">
				<template #title>Export Window</template>
				<template #content>
					<p class="section-copy">Choose how much recent data to include.</p>
					<div class="button-row">
						<Button
							label="Last Hour"
							:severity="selectedTime === 'hour' ? 'primary' : 'secondary'"
							outlined
							@click="selectTime('hour')"
						/>
						<Button
							label="Last Day"
							:severity="selectedTime === 'day' ? 'primary' : 'secondary'"
							outlined
							@click="selectTime('day')"
						/>
						<Button
							label="Last Week"
							:severity="selectedTime === 'week' ? 'primary' : 'secondary'"
							outlined
							@click="selectTime('week')"
						/>
					</div>
				</template>
			</Card>

			<Card class="feature-card">
				<template #title>Output Format</template>
				<template #content>
					<p class="section-copy">Pick the file type you want to generate.</p>
					<div class="stack-list">
						<Button
							label="PDF summary report"
							:severity="selectedFormat === 'pdf' ? 'primary' : 'secondary'"
							outlined
							@click="selectFormat('pdf')"
						/>
						<Button
							label="Plain text export"
							:severity="selectedFormat === 'text' ? 'primary' : 'secondary'"
							outlined
							@click="selectFormat('text')"
						/>
						<Button
							label="CSV / raw data export"
							:severity="selectedFormat === 'csv' ? 'primary' : 'secondary'"
							outlined
							@click="selectFormat('csv')"
						/>
					</div>
				</template>
			</Card>

			<Card class="feature-card">
				<template #title>Data Scope</template>
				<template #content>
					<p class="section-copy">Choose which backend history route to hit.</p>
					<div class="stack-list">
						<Button
							label="Temperature events"
							:severity="selectedScope === 'temp' ? 'success' : 'secondary'"
							outlined
							@click="selectScope('temp')"
						/>
						<Button
							label="Valve events"
							:severity="selectedScope === 'valve' ? 'success' : 'secondary'"
							outlined
							@click="selectScope('valve')"
						/>
						<Button
							label="ML alerts coming soon"
							severity="secondary"
							outlined
							disabled
						/>
					</div>
				</template>
			</Card>

			<Card class="feature-card feature-card-wide">
				<template #title>Export Queue</template>
				<template #content>
					<p class="section-copy">
						Request a live export from the API and reuse the same response for
						the preview panel below.
					</p>
					<div class="queue-shell">
						<div class="queue-row">
							<span>{{ isLoading ? "Preparing export..." : lastExportMessage }}</span>
							<Button
								label="Request Export"
								icon="pi pi-download"
								@click="requestExport"
								:loading="isLoading"
								:disabled="!canRequestExport"
							/>
						</div>
					</div>
				</template>
			</Card>
		</section>

		<section v-if="reportPreview" class="preview-shell">
			<div class="preview-header">
				<div>
					<p class="preview-eyebrow">Report Preview</p>
					<h2>Live Summary Layout</h2>
				</div>
				<Tag value="Uses real API data" severity="success" />
			</div>

			<ExportReportTemplate v-bind="reportPreview" />
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
	background: linear-gradient(135deg, #f8fafc 0%, #e0f2fe 100%);
	border: 1px solid #cbd5e1;
}

.hero-tags {
	display: flex;
	flex-wrap: wrap;
	gap: 0.75rem;
}

.eyebrow,
.preview-eyebrow {
	margin: 0 0 0.4rem;
	font-size: 0.8rem;
	font-weight: 700;
	text-transform: uppercase;
	letter-spacing: 0.08em;
	color: #0369a1;
}

h1,
h2 {
	margin: 0;
	line-height: 1.15;
	color: #0f172a;
}

h1 {
	font-size: 1.85rem;
}

h2 {
	font-size: 1.45rem;
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

.preview-shell {
	margin-top: 1.75rem;
	padding: 1.5rem;
	border-radius: 18px;
	border: 1px solid #d8e0ec;
	background: linear-gradient(180deg, #f8fbff 0%, #eff5fb 100%);
}

.preview-header {
	display: flex;
	justify-content: space-between;
	align-items: flex-start;
	gap: 1rem;
	margin-bottom: 1rem;
}

@media (max-width: 899px) {
	.page-shell {
		padding: 1rem;
	}

	.page-hero,
	.preview-header,
	.queue-row {
		flex-direction: column;
		align-items: stretch;
	}
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
