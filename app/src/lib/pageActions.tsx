import { createContext, useContext, useMemo, useState } from "react";
import { createPortal } from "react-dom";
import { cn } from "@/lib/utils";

// A page's own view controls rendered into the app header's title row, beside the page
// title. The header owns the slot; the page owns the controls and their state.

interface PageActionsContextValue {
    target: HTMLElement | null;
    setTarget: (el: HTMLElement | null) => void;
}

const PageActionsContext = createContext<PageActionsContextValue | null>(null);

export function PageActionsProvider({ children }: { children: React.ReactNode }) {
    const [target, setTarget] = useState<HTMLElement | null>(null);
    const value = useMemo(() => ({ target, setTarget }), [target]);
    return <PageActionsContext.Provider value={value}>{children}</PageActionsContext.Provider>;
}

/** The header's slot; collapses while no page fills it. */
export function PageActionsSlot({ className }: { className?: string }) {
    const ctx = useContext(PageActionsContext);
    return <div ref={ctx?.setTarget} data-testid="page-actions" className={cn("flex items-center gap-2 empty:hidden", className)} />;
}

/** Renders `children` into the header slot; inline when there is no header (e.g. a page mounted on its own). */
export function PageActions({ children }: { children: React.ReactNode }) {
    const ctx = useContext(PageActionsContext);
    if (!ctx) return <>{children}</>;
    return ctx.target ? createPortal(children, ctx.target) : null;
}
