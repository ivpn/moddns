import { useEffect, useState } from "react";

export const ENTRANCE_MS = 600;

function reducedMotion(): boolean {
    return typeof window === "undefined" || typeof window.matchMedia !== "function" || window.matchMedia("(prefers-reduced-motion: reduce)").matches;
}

/**
 * True while a chart should play its entrance animation: once per `shapeKey` (the window the chart
 * shows), so a silent refetch of the same window never replays it. Off under reduced motion and
 * wherever `matchMedia` is missing (jsdom).
 */
export function useChartEntrance(shapeKey: string): boolean {
    const [doneKey, setDoneKey] = useState<string | null>(null);
    const active = doneKey !== shapeKey && !reducedMotion();
    useEffect(() => {
        if (!active) return;
        const t = setTimeout(() => setDoneKey(shapeKey), ENTRANCE_MS + 150);
        return () => clearTimeout(t);
    }, [active, shapeKey]);
    return active;
}
