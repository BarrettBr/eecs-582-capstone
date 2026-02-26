// Name: layout.js
// Description: Used to store state in the layout section
// Programmers: Adam Berry 
// Creation Date: 2/25
// Revision Dates: 
// Preconditions: Not Relevant
// Postconditions: Not Relevant
// Error Types: Not Relevant
// Invariants: Dependencies described in /Docs/web.md
// Known Faults: None

import { computed, reactive } from 'vue';

const layoutConfig = reactive({
    darkTheme: false,
    menuMode: 'static'
});

const layoutState = reactive({
    staticMenuInactive: false,
    mobileMenuActive: false 
});

export function useLayout() {
    
    const toggleMenu = () => {
        if (window.innerWidth > 991) {
            layoutState.staticMenuInactive = !layoutState.staticMenuInactive;
        } else {
            layoutState.mobileMenuActive = !layoutState.mobileMenuActive;
        }
    };

    const hideMobileMenu = () => {
        layoutState.mobileMenuActive = false;
    };

    const isDarkTheme = computed(() => layoutConfig.darkTheme);
    const isDesktop = () => window.innerWidth > 991;

    return {
        layoutConfig,
        layoutState,
        isDarkTheme,
        toggleMenu,
        hideMobileMenu,
        isDesktop
    };
}
