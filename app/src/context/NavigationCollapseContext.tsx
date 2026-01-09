import React, { createContext, useContext, useEffect, useState } from "react";
import { useScreenDetector } from "@/hooks/useScreenDetector";

interface NavigationCollapseContextType {
    collapsed: boolean;
    toggleCollapse: () => void;
    setCollapsed: (collapsed: boolean) => void;
}

const NavigationCollapseContext = createContext<NavigationCollapseContextType | undefined>(undefined);

export const NavigationCollapseProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
    const [collapsed, setCollapsed] = useState(false);
    const { navDesktop, width } = useScreenDetector();

    // Auto-collapse on narrower desktops, expand on wider; keep sidebar collapsed when navDesktop is disabled
    const AUTO_EXPAND_WIDTH = 1440;

    useEffect(() => {
        if (!navDesktop) {
            setCollapsed(true);
            return;
        }
        setCollapsed(width < AUTO_EXPAND_WIDTH);
    }, [navDesktop, width]);

    const toggleCollapse = () => setCollapsed((prev) => !prev);

    return (
        <NavigationCollapseContext.Provider value={{ collapsed, toggleCollapse, setCollapsed }}>
            {children}
        </NavigationCollapseContext.Provider>
    );
};

export function useNavigationCollapse() {
    const ctx = useContext(NavigationCollapseContext);
    if (!ctx) throw new Error("useNavigationCollapse must be used within NavigationCollapseProvider");
    return ctx;
}
