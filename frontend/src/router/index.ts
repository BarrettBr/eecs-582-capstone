import { createRouter, createWebHistory } from "vue-router";
import HomeView from "../views/HomeView.vue";
import DashboardView from "../views/DashboardView.vue";
import AboutView from "../views/AboutView.vue";

const router = createRouter({
	history: createWebHistory(import.meta.env.BASE_URL),
	routes: [
		{
			path: "/",
			name: "home",
			component: HomeView
		},
		{
			path: "/about",
			name: "about",
			// Lazy load the about section since it is unlikely to be used often
			//component: () => import("../views/AboutView.vue")
			component: AboutView
		},
		{
			path: "/dashboard",
			name: "dashboard",
			component: DashboardView
		},
	]
});

export default router;
