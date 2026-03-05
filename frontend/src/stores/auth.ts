// Name: auth.ts
// Description: This file acts as a store for the state of authentication in the user's session
// Programmers: Adam Berry
// Creation Date: 3/4
// Revision Dates:
// Preconditions: None
// Postconditions: The users knowledge of current auth state
// Error Types: Not Relevant
// Invariants: Dependencies described in /Docs/web.md
// Known Faults: None

import { defineStore } from "pinia";
import apiHelper from "@/utils/apiHelper";
import router from "@/router";

interface AuthState {
	username: string | null;
	token: string | null;
	error: string | null;
}

export const useAuthStore = defineStore("auth", {
	state: (): AuthState => ({
		username: null,
		token: null,
		error: null,
	}),

	actions: {
		async login(username: string, password: string) {
			this.error = null;
			try {
				const response = await apiHelper.login(username, password);
				const token = response.data.token; // adjust to your API
				this.username = username;
				this.token = token;

				router.push("/dashboard"); // optional, can be in component
			} catch (err: any) {
				this.error = err.response?.data?.message || "Login failed";
				throw err; // allow component to react too
			}
		},

		async logout() {
			try {
				await apiHelper.logout?.(); // optional
			} catch (e) {
				console.warn("Logout API failed", e);
			}
			this.username = null;
			this.token = null;
			router.push("/login");
		},
	},

	getters: {
		isLoggedIn: (state) => !!state.token,
		authHeader: (state) => (state.token ? `Bearer ${state.token}` : null),
	},

	persist: {
		pick: ["token", "username"], // don't persist 'error'
	},
});
