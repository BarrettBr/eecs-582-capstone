<!-- Name: Login
	Description: This file is the entry to the web app, it sets up auth
	Programmers: Adam Berry 
	Creation Date: 3/4
	Revision Dates:
	Preconditions: Not Relevant
	Postconditions: The state of user login is stored in stores/Auth
	Error Types: Not Relevant
	Invariants: Dependencies described in /Docs/web.md
	Known Faults: I really doubt we really need a seperate about component for this 
-->
<script setup>
import { ref } from "vue";
import { useAuthStore } from "@/stores/auth";

import Card from "primevue/card";
import Button from "primevue/button";
import InputText from "primevue/inputtext";
import Password from "primevue/password";

const authStore = useAuthStore();

const username = ref("");
const password = ref("");

const handleLogin = async () => {
	try {
		await authStore.login(username.value, password.value);
	} catch (e) {
		console.error(e);
	}
};
</script>

<template>
	<div class="page">
		<Card class="login-card">
			<template #title>
				<div class="title">Login</div>
			</template>

			<template #content>
				<form class="form" @submit.prevent="handleLogin">
					<div class="field">
						<label for="username">Username</label>
						<InputText id="username" v-model="username" />
					</div>

					<div class="field">
						<label for="password">Password</label>
						<Password
							id="password"
							v-model="password"
							:feedback="false"
							toggleMask
						/>
					</div>

					<Button type="submit" label="Login" class="submit" />

					<small v-if="authStore.error" class="error">
						{{ authStore.error }}
					</small>
				</form>
			</template>
		</Card>
	</div>
</template>

<style scoped>
.page {
	display: flex;
	align-items: center;
	justify-content: center;
	height: 100vh;

	background: #f5f7fa;
}

.login-card {
	width: 360px;
	padding: 1rem;
	border-radius: 10px;
}

.title {
	text-align: center;
	font-size: 1.5rem;
	font-weight: 600;
}

.form {
	display: flex;
	flex-direction: column;
	gap: 1.25rem;
	margin-top: 0.5rem;
}

.field {
	display: flex;
	flex-direction: column;
	gap: 0.35rem;
}

label {
	font-size: 0.9rem;
	font-weight: 500;
}

.submit {
	width: 100%;
}

.error {
	color: #ef4444;
	font-size: 0.85rem;
}
:deep(.p-password) {
	width: 100%;
}

:deep(.p-password-input) {
	width: 100%;
	padding-right: 2.5rem; /* space for the eye icon */
}
</style>
