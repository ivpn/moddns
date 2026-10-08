import React, { useId, useState } from "react";
import { ChartColumn, Table2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { LIMITED_ACCESS_TEXT } from "@/components/data-collection/model";

export const titleText = "text-[var(--tailwind-colors-slate-50)]";
export const mutedText = "text-[var(--tailwind-colors-slate-200)]";

export function StatsCard({ className, ...props }: React.ComponentProps<"div">) {
    return (
        <div
            className={cn(
                "rounded-lg border border-[var(--tailwind-colors-slate-light-300)] dark:border-transparent bg-transparent dark:bg-[var(--variable-collection-surface)] p-4 md:p-6 min-w-0",
                className,
            )}
            {...props}
        />
    );
}

export function GroupHeading({ children, caption }: { children: React.ReactNode; caption?: React.ReactNode }) {
    return (
        <div className="flex flex-col gap-1">
            <h2 className="font-['Roboto_Mono-Bold',Helvetica] font-bold text-[var(--tailwind-colors-rdns-600)] text-base uppercase tracking-wide leading-4">
                {children}
            </h2>
            {caption && <p className={cn("text-[13px] leading-[18px]", mutedText)}>{caption}</p>}
        </div>
    );
}

export type PanelView = "chart" | "table";

export function ViewToggle({ title, view, onChange }: { title: string; view: PanelView; onChange: (v: PanelView) => void }) {
    const btn = (v: PanelView, label: string, Icon: typeof Table2) => (
        <button
            type="button"
            aria-pressed={view === v}
            title={label}
            onClick={() => onChange(v)}
            className={cn(
                "inline-flex items-center justify-center cursor-pointer min-h-11 min-w-11 lg:min-h-8 lg:min-w-8 transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-[var(--tailwind-colors-rdns-600)]",
                view === v
                    ? "bg-[var(--tailwind-colors-rdns-600)]/15 text-[var(--tailwind-colors-rdns-600)]"
                    : cn("hover:bg-muted", mutedText),
            )}
        >
            <Icon className="w-4 h-4" aria-hidden />
            <span className="sr-only">{label}</span>
        </button>
    );
    return (
        <div
            role="group"
            aria-label={`${title} view`}
            className="flex items-center overflow-hidden rounded-md border border-[var(--tailwind-colors-slate-light-300)] dark:border-[var(--tailwind-colors-slate-600)] [&>button+button]:border-l [&>button+button]:border-[var(--tailwind-colors-slate-light-300)] dark:[&>button+button]:border-[var(--tailwind-colors-slate-600)]"
        >
            {btn("chart", "Chart", ChartColumn)}
            {btn("table", "Table", Table2)}
        </div>
    );
}

/** X1, X3: a labelled section with a per-panel Chart | Table toggle. */
export function PanelShell({
    title,
    toggle = true,
    defaultView = "chart",
    className,
    children,
}: {
    title: string;
    toggle?: boolean;
    defaultView?: PanelView;
    className?: string;
    children: (view: PanelView) => React.ReactNode;
}) {
    const id = useId();
    const [view, setView] = useState<PanelView>(defaultView);
    return (
        <StatsCard className={className}>
            <section aria-labelledby={id} className="flex flex-col gap-4 min-w-0">
                <div className="flex items-center justify-between gap-2">
                    <h3 id={id} className={cn("font-semibold text-base leading-6", titleText)}>
                        {title}
                    </h3>
                    {toggle && <ViewToggle title={title} view={view} onChange={setView} />}
                </div>
                {children(view)}
            </section>
        </StatsCard>
    );
}

export interface TableColumn {
    label: string;
    align?: "right";
}

export function StatsTable({
    caption,
    columns,
    rows,
    scroll,
}: {
    caption: string;
    columns: TableColumn[];
    rows: React.ReactNode[][];
    scroll?: boolean;
}) {
    return (
        <div
            className={cn("w-full overflow-x-auto", scroll && "max-h-80 overflow-y-auto")}
            {...(scroll ? { tabIndex: 0, role: "region", "aria-label": caption } : {})}
        >
            <table className="w-full text-sm tabular-nums">
                <caption className={cn("text-left text-[13px] pb-2", mutedText)}>{caption}</caption>
                <thead className={cn(scroll && "sticky top-0 bg-[var(--shadcn-ui-app-background)] dark:bg-[var(--variable-collection-surface)]")}>
                    <tr className="border-b border-[var(--tailwind-colors-slate-600)]">
                        {columns.map(c => (
                            <th
                                key={c.label}
                                scope="col"
                                className={cn("py-2 pr-3 font-medium", mutedText, c.align === "right" ? "text-right" : "text-left")}
                            >
                                {c.label}
                            </th>
                        ))}
                    </tr>
                </thead>
                <tbody>
                    {rows.map((cells, i) => (
                        <tr key={i} className="border-b border-[var(--tailwind-colors-slate-600)]/40 last:border-0">
                            {cells.map((cell, j) => (
                                <td key={j} className={cn("py-2 pr-3", titleText, columns[j]?.align === "right" && "text-right")}>
                                    {cell}
                                </td>
                            ))}
                        </tr>
                    ))}
                </tbody>
            </table>
        </div>
    );
}

export interface BarRow {
    id: string;
    label: React.ReactNode;
    value: number;
    /** CSS color for a single-segment bar. */
    color?: string;
    /** A red segment inside the bar (devices). */
    blocked?: number;
    meta: React.ReactNode;
}

/**
 * X6, X7: label · bar · text; every row carries text, never colour alone.
 * "share" scales each bar to its part of all rows, so the track means 100 %;
 * "leader" scales to the largest row, ranking the rows against each other.
 */
export function BarList({
    rows,
    scale,
    collapseAfter = 5,
    noun = "rows",
}: {
    rows: BarRow[];
    scale: "share" | "leader";
    collapseAfter?: number;
    noun?: string;
}) {
    const [expanded, setExpanded] = useState(false);
    const denominator = scale === "share" ? rows.reduce((s, r) => s + r.value, 0) : Math.max(...rows.map(r => r.value));
    const many = rows.length > collapseAfter;
    const shown = many && !expanded ? rows.slice(0, collapseAfter) : rows;
    const seg = (value: number, color: string | undefined) =>
        value > 0 && denominator > 0 ? (
            <span data-testid="bar-seg" style={{ width: `${(value / denominator) * 100}%`, minWidth: 2, background: color }} />
        ) : null;
    return (
        <>
            <ul className="flex flex-col gap-3">
                {shown.map(r => (
                    <li key={r.id} className="grid grid-cols-[minmax(0,1fr)_auto] gap-x-3 gap-y-1 items-center">
                        <span className={cn("min-w-0 truncate text-sm flex items-center gap-2", titleText)}>{r.label}</span>
                        <span className={cn("text-sm tabular-nums text-right", titleText)}>{r.meta}</span>
                        <span
                            aria-hidden
                            data-testid="bar-row"
                            className="col-span-2 flex h-2.5 rounded-[3px] overflow-hidden bg-[var(--stats-track)]"
                        >
                            {r.blocked !== undefined ? (
                                <>
                                    {seg(r.value - r.blocked, "var(--stats-all)")}
                                    {seg(r.blocked, "var(--stats-blocked)")}
                                </>
                            ) : (
                                seg(r.value, r.color)
                            )}
                        </span>
                    </li>
                ))}
            </ul>
            {many && (
                <Button
                    type="button"
                    variant="outline"
                    className="self-start min-h-11 lg:min-h-9"
                    aria-expanded={expanded}
                    onClick={() => setExpanded(e => !e)}
                >
                    {expanded ? `Show top ${collapseAfter}` : `Show ${rows.length} ${noun}`}
                </Button>
            )}
        </>
    );
}

/** G-* gate blocks and the one-card states: one panel's height, never a tall empty frame. */
export function MessageCard({
    icon,
    title,
    titleId,
    children,
    action,
    headingRef,
}: {
    icon: React.ReactNode;
    title: string;
    titleId?: string;
    children?: React.ReactNode;
    action?: React.ReactNode;
    headingRef?: React.Ref<HTMLHeadingElement>;
}) {
    const auto = useId();
    const id = titleId ?? auto;
    return (
        <StatsCard>
            <section aria-labelledby={id} className="flex flex-col sm:flex-row sm:items-center gap-4">
                <span aria-hidden className="text-[var(--tailwind-colors-rdns-600)] flex-none">
                    {icon}
                </span>
                <div className="flex flex-col gap-1 min-w-0 flex-1">
                    <h3 id={id} ref={headingRef} tabIndex={-1} className={cn("font-semibold text-base leading-6 outline-none", titleText)}>
                        {title}
                    </h3>
                    {children}
                </div>
                {action}
            </section>
        </StatsCard>
    );
}

export function GateAction({
    label,
    onClick,
    restricted,
    describedBy,
}: {
    label: string;
    onClick: () => void;
    restricted: boolean;
    describedBy?: string;
}) {
    return (
        <Button
            type="button"
            className="min-h-11 lg:min-h-9 bg-[var(--tailwind-colors-rdns-600)] text-white hover:bg-[var(--tailwind-colors-rdns-800)] self-start sm:self-auto"
            disabled={restricted}
            aria-describedby={restricted ? describedBy : undefined}
            onClick={onClick}
        >
            {label}
        </Button>
    );
}

/** P16: the reason is visible text, not a `title`. */
export function LimitedAccessNote({ id }: { id: string }) {
    return (
        <p id={id} className={cn("text-[13px] leading-[18px]", mutedText)}>
            {LIMITED_ACCESS_TEXT}
        </p>
    );
}
