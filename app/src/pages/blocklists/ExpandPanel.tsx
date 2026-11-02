import { useState, useRef, useEffect, useCallback } from "react";
import type { LucideIcon } from "lucide-react";

/* ------------------------------------------------------------------ */
/*  Expand panel — full-width with two-phase animation                 */
/* ------------------------------------------------------------------ */

interface ExpandPanelProps {
    expanded: boolean;
    categoryLabel: string;
    categoryIcon: LucideIcon;
    children: React.ReactNode;
}

export default function ExpandPanel({ expanded, categoryLabel, categoryIcon: Icon, children }: ExpandPanelProps) {
    const innerRef = useRef<HTMLDivElement>(null);
    const [contentHeight, setContentHeight] = useState(0);
    const [spread, setSpread] = useState(false);

    useEffect(() => {
        if (expanded && innerRef.current) {
            setContentHeight(innerRef.current.scrollHeight);
        }
        if (!expanded) {
            setSpread(false);
        }
    }, [expanded, children]);

    const handleTransitionEnd = useCallback(
        (e: React.TransitionEvent) => {
            if (e.propertyName === "max-height" && expanded) {
                setSpread(true);
            }
        },
        [expanded],
    );

    return (
        <div
            className="w-full overflow-hidden transition-[max-height] duration-300 ease-out"
            // The measured height only drives the open animation; once it ends the cap is
            // lifted so later reflows (resize, tab remount) cannot clip the content.
            style={{ maxHeight: expanded ? (spread ? "none" : `${contentHeight + 48}px`) : "0px" }}
            onTransitionEnd={handleTransitionEnd}
        >
            <div ref={innerRef}>
                {/* Connector bar */}
                <div className="flex items-center gap-2 pt-1 pb-3 px-1">
                    <div className="h-px flex-1 bg-gradient-to-r from-transparent via-[var(--tailwind-colors-rdns-600)]/30 to-transparent" />
                    <div className="flex items-center gap-1.5 text-[11px] font-medium text-[var(--tailwind-colors-slate-400)] uppercase tracking-wider select-none">
                        <Icon className="h-3 w-3 text-[var(--tailwind-colors-rdns-600)]/60" />
                        {categoryLabel}
                    </div>
                    <div className="h-px flex-1 bg-gradient-to-r from-transparent via-[var(--tailwind-colors-rdns-600)]/30 to-transparent" />
                </div>

                {/* Blocklist cards with horizontal spread animation */}
                <div
                    className={`grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 xl:grid-cols-4 gap-4 pb-3 transition-all duration-300 ease-out ${
                        spread
                            ? "opacity-100 scale-x-100"
                            : "opacity-0 scale-x-95"
                    }`}
                    style={{ transformOrigin: "top center" }}
                >
                    {children}
                </div>

                {/* Bottom separator — closes off the expanded section */}
                <div className="flex items-center gap-2 pb-2 px-1">
                    <div className="h-px flex-1 bg-gradient-to-r from-transparent via-[var(--tailwind-colors-rdns-600)]/30 to-transparent" />
                    <div className="flex items-center gap-1.5 text-[11px] font-medium text-[var(--tailwind-colors-slate-400)] select-none">
                        <Icon className="h-3 w-3 text-[var(--tailwind-colors-rdns-600)]/60" />
                    </div>
                    <div className="h-px flex-1 bg-gradient-to-r from-transparent via-[var(--tailwind-colors-rdns-600)]/30 to-transparent" />
                </div>
            </div>
        </div>
    );
}
