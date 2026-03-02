<!-- Name: AboutView
	Description: The view describing the about page 
	Programmers: Adam Berry 
	Creation Date: 2/14
	Revision Dates: Adam Berry 2/14, Adam Berry 2/15
	Preconditions: Not Relevant
	Postconditions: Not Relevant
	Error Types: Not Relevant
	Invariants: Dependencies described in /Docs/web.md
	Known Faults: I really doubt we really need a seperate about component for this 
-->
<script setup lang="ts">
	import { ref, onMounted } from "vue";
import Card from "primevue/card";
</script>

<template>
	<main>
		<div class="card">
			<div class="project-description">
				<h2>About This Project</h2>
				<p>
				Modern industrial systems generate continuous streams of operational data including temperatures, actuator states, and power usage. Many smaller scale industrial environments lack accessible real-time insight into this data, particularly regarding the detection of abnormal behavior before it results in system failure.
				</p>
				<p>
				This capstone project delivers a full-stack industrial monitoring and anomaly detection platform designed to bridge that gap. The system integrates directly with a programmable logic controller using Modbus TCP to ingest live telemetry data. This data flows through a structured ingestion pipeline where it is stored efficiently, analyzed using machine learning models, and visualized through a responsive web dashboard.
				</p>
				<p>
				The result is an end-to-end monitoring solution that demonstrates how modern backend systems, streaming architecture, and applied machine learning work together in an industrial context.
				</p>

				<h3>Problem Identification</h3>
				<p>
				Industrial control systems traditionally rely on manual monitoring, threshold based alarms, and limited historical insight. While threshold alarms are useful, they often fail to capture subtle patterns that indicate emerging issues.
				</p>
				<p>
				Our project introduces anomaly detection based on data clustering to identify unusual system behavior even when hard limits are not exceeded. This approach allows for earlier detection of abnormal conditions, reduced reliance on fixed thresholds, and improved operational awareness.
				</p>

				<h3>System Architecture Overview</h3>
				<p>
				The platform consists of several coordinated components. A Click PLC or simulated OpenPLC runtime exposes operational values over Modbus TCP. These include temperature readings, fan state, heater power, and other industrial signals.
				</p>
				<p>
				A Go based ingestion service continuously polls the PLC, normalizes readings, and stores structured samples in a SQLite database. This service utilizes sqlc for type safe query generation and goose for database migrations alongside concurrent polling and batching logic.
				</p>
				<p>
				The machine learning layer uses a KMeans clustering model to analyze collected telemetry samples and classify system behavior. Based on patterns in temperature, fan activity, and heater power, the model flags anomalous states. This demonstrates how unsupervised learning applies to industrial telemetry without requiring labeled failure data.
				</p>
				<p>
				The web dashboard provides live telemetry visualization, historical trend analysis, anomaly indicators, and rule based validation alerts. It connects to backend services through HTTP and WebSocket endpoints for near real-time updates.
				</p>

				<h3>Technical Focus Areas</h3>
				<p>
				This project emphasizes real-time data ingestion pipelines and Modbus TCP protocol integration. Significant focus was placed on backend concurrency, batching, database schema design, and migration workflows.
				</p>
				<p>
				By applying machine learning in operational systems and focusing on end-to-end system architecture, the project demonstrates how distributed components integrate into a cohesive monitoring platform rather than focusing on a single algorithm or interface feature.
				</p>

				<h3>Project Significance</h3>
				<p>
				Industrial systems increasingly rely on data driven insight. By combining traditional PLC infrastructure with modern backend services and machine learning, this project illustrates how legacy control environments can be extended with intelligent monitoring
				</p>
			</div>
		</div>
	</main>
</template>
<style scoped>
.project-description {
  line-height: 1.7; /* Increased slightly for better readability */
  max-width: 800px;
  margin: 0 auto;
  color: #334155; /* Slate gray for softer contrast than pure black */
}

h2 {
  font-size: 1.875rem;
  color: #0f172a;
  border-bottom: 2px solid #e2e8f0;
  padding-bottom: 0.5rem;
  margin-bottom: 1.5rem;
}

h3 {
  font-size: 1.5rem;
  color: #1e293b;
  margin-top: 2.5rem; /* More space before new sections */
  margin-bottom: 1rem;
}

p {
  margin-bottom: 1.25rem;
}

.card {
  padding: 2rem; /* Increased padding for a more "premium" feel */
  grid-column: span 12;
  background-color: #ffffff;
  border-radius: 8px;
  box-shadow: 0 4px 6px -1px rgb(0 0 0 / 0.1);
}
</style>
