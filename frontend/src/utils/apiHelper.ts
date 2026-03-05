// Name: auth.ts
// Description: This file acts as helper to hit the API
// Programmers: Adam Berry
// Creation Date: 3/4
// Revision Dates:
// Preconditions: None
// Postconditions: api is the URL that is created based on the vite enivorment variables
// Error Types: Not Relevant
// Invariants: Dependencies described in /Docs/web.md
// Known Faults: None
import axios from "axios";
import router from "@/router";

const API_BASE_URL =
	import.meta.env.VITE_API_BASE_URL || "http://localhost:8000/api";

const api = axios.create({
	baseURL: API_BASE_URL,
	headers: {
		"Content-Type": "application/json",
	},
});

// Attach auth token if logged in
api.interceptors.request.use((config) => {
	const auth = JSON.parse(localStorage.getItem("auth") || "{}");
	if (auth.token) {
		config.headers.Authorization = `Bearer ${auth.token}`;
	}
	return config;
});

// Handle the 401 response error
api.interceptors.response.use(
	(response) => response,
	(error) => {
		if (error.response?.status === 401) {
			// avoid importing the store here — use router directly
			router.push("/login");
		}
		return Promise.reject(error);
	},
);

export default {
	login: async (username: string, password: string) => {
		return api.post("/auth/login", { username, password });
	},
	logout: async () => {
		return api.post("/auth/logout"); // optional, depends on API
	},
	getUserProfile: async () => {
		return api.get("/auth/me");
	},
};
