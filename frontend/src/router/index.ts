//Name: index.ts
//Description: Used to configure the routes to convert from a URL to a view component for App.vue
//Programmers: Adam Berry
//Creation Date: 2/14
//Revision Dates: Adam Berry 2/14, Adam Berry 2/15
//Preconditions: Not Relevant
//Postconditions: Not Relevant
//Error Types: Not Relevant
//Invariants: Dependencies described in /Docs/web.md
//Known Faults: None

import { createRouter, createWebHistory } from "vue-router";
import DashboardView from "@/views/DashboardView.vue";
import AboutView from "@/views/AboutView.vue";
import ExportView from "@/views/ExportView.vue";
import AdminView from "@/views/AdminView.vue";
import AppLayout from "@/layout/AppLayout.vue";

const router = createRouter({
	history: createWebHistory(import.meta.env.BASE_URL),
	routes: [
		{
			path: "/",
			component: AppLayout,
			children: [
				{
					path: "/",
					name: "home",
					component: DashboardView,
				},
				{
					path: "/about",
					name: "about",
					component: AboutView,
				},
				{
					path: "/dashboard",
					name: "dashboard",
					component: DashboardView,
				},
				{
					path: "/export",
					name: "export",
					component: ExportView,
				},
				{
					path: "/admin",
					name: "admin",
					component: AdminView,
				},
			],
		},
	],
});

export default router;
