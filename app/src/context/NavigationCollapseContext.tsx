import React, { createContext, useContext, useEffect, useState } from "react";
import { useScreenDetector } from "@/hooks/useScreenDetector";

interface NavigationCollapseContextType {
    collapsed: boolean;
    setCollapsed: (collapsed: boolean) => void;
}

const NavigationCollapseContext = createContext<NavigationCollapseContextType | undefined>(undefined);

export const NavigationCollapseProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
    const [collapsed, setCollapsed] = useState(false);
    const { navDesktop, width } = useScreenDetector();

    // Hysteresis + debounce to avoid flicker near breakpoints
    const EXPAND_WIDTH = 1440;
    const COLLAPSE_WIDTH = 1360;
    const DEBOUNCE_MS = 150;

    useEffect(() => {
        if (typeof window === 'undefined') return;

        // navDesktop false => always collapse
        const target = !navDesktop
            ? true
            : width >= EXPAND_WIDTH
                ? false
                : width < COLLAPSE_WIDTH
                    ? true
                    : collapsed; // within hysteresis band, keep state

        if (target === collapsed) return;

        const timer = window.setTimeout(() => setCollapsed(target), DEBOUNCE_MS);
        return () => window.clearTimeout(timer);
    }, [navDesktop, width, collapsed]);

    return (
        <NavigationCollapseContext.Provider value={{ collapsed, setCollapsed }}>
            {children}
        </NavigationCollapseContext.Provider>
    );
};

// eslint-disable-next-line react-refresh/only-export-components
export function useNavigationCollapse() {
    const ctx = useContext(NavigationCollapseContext);
    if (!ctx) throw new Error("useNavigationCollapse must be used within NavigationCollapseProvider");
    return ctx;
}
