// Name: main.ts
//Description: The main entry point for declaring Dependencies and initializing the app
//Programmers: Adam Berry 
//Creation Date: 2/14
//Revision Dates: Adam Berry 2/14, Adam Berry 2/15
//Preconditions: Not Relevant
//Postconditions: Not Relevant
//Error Types: Not Relevant
//Invariants: Dependencies described in /Docs/web.md
//Known Faults: None

import "./assets/main.scss";

import { createApp } from "vue";
import { createPinia } from "pinia";

import PrimeVue from "primevue/config";
import Aura from "@primeuix/themes/aura";
import 'primeicons/primeicons.css'

import App from "./App.vue";
import router from "./router";

const app = createApp(App);

app.use(createPinia());
app.use(router);
app.use(PrimeVue, {
	theme: {
		preset: Aura,
		options: {
			darkModeSelector: '.app-dark'
		},
	}
});

app.mount("#app");
