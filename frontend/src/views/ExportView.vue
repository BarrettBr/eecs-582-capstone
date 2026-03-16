<script setup lang="ts">
import Card from "primevue/card";
import Button from "primevue/button";
import Tag from "primevue/tag";
import { ref } from 'vue';
import jsPDF from "jspdf";

// --------------------- User Selections ---------------------
const selectedTime = ref<'hour' | 'day' | 'week' | null>(null);
const selectedFormat = ref<'pdf' | 'text' | 'csv' | 'json' | null>(null);
type ScopeType = 'temp' | 'valve' | 'ml';

const selectedScope = ref<Record<ScopeType, boolean>>({
  temp: false,
  valve: false,
  ml: false
});

// --------------------- Button Handlers ---------------------
function selectTime(window: 'hour' | 'day' | 'week') {
    selectedTime.value = window;
}

function selectFormat(format: 'pdf' | 'text' | 'csv') {
    selectedFormat.value = format;
}

function toggleScope(scope: 'temp' | 'valve' | 'ml') {
    selectedScope.value[scope] = !selectedScope.value[scope];
}

// --------------------- Export Request ---------------------
async function requestExport() {

	const scope = (Object.keys(selectedScope.value) as ScopeType[])
	.find((k) => selectedScope.value[k]);
	
    if (!scope || !selectedTime.value || !selectedFormat.value) {
        alert("Select time, format, and scope");
        return;
    }

    try {

        const url = `http://localhost:8080/api/v1/report/${scope}/${selectedTime.value}`;
        const res = await fetch(url);

        if (!res.ok) throw new Error("API request failed");

        const data = await res.json();

        let fileContent: string | Blob = "";
        let fileType = "";
        let extension = "";

        // PDF export
        if (selectedFormat.value === "pdf") {
    const summary = data.summary;

    const doc = new jsPDF();
    doc.setFontSize(12);
    doc.text(`Report Export`, 10, 10);
    doc.text(`Scope: ${scope}`, 10, 20);
    doc.text(`Time Window: ${selectedTime.value}`, 10, 30);
    doc.text(`Average Temperature: ${summary.avg_temp ?? "N/A"}`, 10, 40);
    doc.text(`Minimum Temperature: ${summary.min_temp ?? "N/A"}`, 10, 50);
    doc.text(`Maximum Temperature: ${summary.max_temp ?? "N/A"}`, 10, 60);
    doc.text(`Generated: ${new Date().toLocaleString()}`, 10, 70);

    // Output as Blob
    const pdfBlob = doc.output("blob");

    fileContent = pdfBlob;
    fileType = "application/pdf";
    extension = "pdf";
}

        // Plain text export
       if (selectedFormat.value === "text") {
			const rows: Record<string, any>[] = data.data;

			if (!rows || rows.length === 0) {
				alert("No row data available to export");
				return;
			}

			fileContent = rows.map(r => JSON.stringify(r)).join("\n");
			fileType = "text/plain";
			extension = "txt";
		}
        // CSV export
        if (selectedFormat.value === "csv") {
			const rows: Record<string, any>[] = data.data;

			if (!rows || rows.length === 0) {
				alert("No row data available to export");
				return;
			}

			const headers = Object.keys(rows[0]);
			const csvRows = [
				headers.join(","),
				...rows.map(r => headers.map(h => r[h]).join(","))
			];

			fileContent = csvRows.join("\n");
			fileType = "text/csv";
			extension = "csv";
		}

        const blob = new Blob([fileContent], { type: fileType });

        const downloadURL = URL.createObjectURL(blob);

        const a = document.createElement("a");
        a.href = downloadURL;
        a.download = `report_${scope}_${selectedTime.value}.${extension}`;
        a.click();

        URL.revokeObjectURL(downloadURL);

    } catch (err) {
        console.error("Export failed:", err);
        alert("Export failed, check console");
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
                Manage saved reports and build exports from recent system activity.
            </p>
        </div>
    </section>

    <section class="page-grid">
        <!-- Export Window -->
        <Card class="feature-card feature-card-wide">
            <template #title>Export Window</template>
            <template #content>
                <p class="section-copy">Choose how much recent data to include.</p>
                <div class="button-row">
                    <Button 
                        label="Last Hour" 
                        :severity="selectedTime==='hour'?'primary':'secondary'" 
                        outlined 
                        @click="selectTime('hour')"
                    ></Button>
                    <Button 
                        label="Last Day" 
                        :severity="selectedTime==='day'?'primary':'secondary'" 
                        outlined 
                        @click="selectTime('day')"
                    ></Button>
                    <Button 
                        label="Last Week" 
                        :severity="selectedTime==='week'?'primary':'secondary'" 
                        outlined 
                        @click="selectTime('week')"
                    ></Button>
                </div>
            </template>
        </Card>

        <!-- Output Format -->
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
					></Button>
					<Button
						label="Plain text export"
						:severity="selectedFormat === 'text' ? 'primary' : 'secondary'"
						outlined
						@click="selectFormat('text')"
					></Button>
					<Button
						label="CSV / raw data export"
						:severity="selectedFormat === 'csv' ? 'primary' : 'secondary'"
						outlined
						@click="selectFormat('csv')"
					></Button>
                </div>
            </template>
        </Card>

        <!-- Data Scope -->
        <Card class="feature-card">
            <template #title>Data Scope</template>
            <template #content>
                <p class="section-copy">Choose which parts of the system to include.</p>
                <div class="stack-list">
                    <Button
						label="Temperature events"
						:severity="selectedScope.temp ? 'success' : 'secondary'"
						outlined
						@click="toggleScope('temp')"
					></Button>
					<Button
						label="Valve events"
						:severity="selectedScope.valve ? 'success' : 'secondary'"
						outlined
						@click="toggleScope('valve')"
					></Button>
					<Button
						label="ML alerts"
						:severity="selectedScope.ml ? 'success' : 'secondary'"
						outlined
						@click="toggleScope('ml')"
					></Button>
                </div>
            </template>
        </Card>

        <!-- Export Queue -->
        <Card class="feature-card feature-card-wide">
            <template #title>Export Queue</template>
            <template #content>
                <p class="section-copy">Track current export requests and finished downloads here.</p>
                <div class="queue-shell">
                    <div class="queue-row">
                        <span>No export jobs yet.</span>
                        <Button 
                            label="Request Export" 
                            icon="pi pi-download" 
                            @click="requestExport"
                            :disabled="!selectedTime || !selectedFormat"
                        ></Button>
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
	background: linear-gradient(135deg, #f8fafc 0%, #e0f2fe 100%);
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
