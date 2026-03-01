// Name: layout.ts
// Description: This file acts as a store for the state of the layout (sidebar closed etc) 
// Programmers: Adam Berry 
// Creation Date: 3/1
// Revision Dates: 
// Preconditions: None
// Postconditions: Not Relevant
// Error Types: Not Relevant
// Invariants: Dependencies described in /Docs/web.md
// Known Faults: None

import { defineStore } from "pinia";
import { ref } from "vue";

export const useLayoutStore = defineStore("layout", () => {
	const sidebarOpen = ref(true);
	const mobileMenuOpen = ref(false);

	function toggleSidebar() {
		sidebarOpen.value = !sidebarOpen.value;
	}

	function toggleMobileMenu() {
		mobileMenuOpen.value = !mobileMenuOpen.value;
	}

	function closeMobileMenu() {
		mobileMenuOpen.value = false;
	}

	return {
		sidebarOpen,
		mobileMenuOpen,
		toggleSidebar,
		toggleMobileMenu,
		closeMobileMenu,
	};
});

