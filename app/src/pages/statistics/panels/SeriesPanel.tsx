import { useEffect, useMemo, useRef, useState } from "react";
import {
    Area,
    Bar,
    CartesianGrid,
    ComposedChart,
    ReferenceLine,
    Tooltip,
    XAxis,
    YAxis,
    useActiveTooltipLabel,
    useChartWidth,
    useIsTooltipActive,
    usePlotArea,
    useXAxisInverseDataSnapScale,
    useXAxisScale,
} from "recharts";
import { ChartContainer, type ChartConfig } from "@/components/ui/chart";
import { formatAxisCount, formatCount } from "@/lib/formatStats";
import { buildBuckets, countingSince, shorterViewForStart, type Bucket, type StatsData } from "../derive";
import { rangeDef, type RangeDef, type RangeKey } from "../ranges";
import { axisTicks, bucketUnitWord, formatBucketLabel, formatDateTime } from "../time";
import { LINE_BARS_TABLE, PanelShell, StatsTable, mutedText } from "../primitives";
import { cn } from "@/lib/utils";
import { ENTRANCE_MS, useChartEntrance } from "../useChartEntrance";
import { AXIS_BAND_HEIGHT, HOVER_LABEL_HEIGHT, HOVER_STRIP_HEIGHT, axisLabelBox } from "../axisHover";

const config: ChartConfig = {
    all: { label: "All queries", color: "var(--stats-all)" },
    blocked: { label: "Blocked", color: "var(--stats-blocked)" },
};

interface Row {
    ts: number;
    all: number | null;
    blocked: number | null;
    /** The in-progress tail, drawn dashed; it also holds the last closed point so the lines join. */
    allTail: number | null;
    blockedTail: number | null;
    /** Column heights for the Bars view: the bucket's own counts, in progress or not. */
    barAll: number | null;
    barBlocked: number | null;
    inProgress: boolean;
}

function toRows(buckets: Bucket[]): Row[] {
    return buckets.map((b, i) => {
        const next = buckets[i + 1];
        const inTail = b.inProgress || (next?.inProgress ?? false);
        return {
            ts: b.ts,
            all: b.inProgress ? null : b.total,
            blocked: b.inProgress ? null : b.blocked,
            allTail: inTail ? b.total : null,
            blockedTail: inTail ? b.blocked : null,
            barAll: b.total,
            barBlocked: b.blocked,
            inProgress: b.inProgress,
        };
    });
}

/** P9: with one or two buckets a line has nothing to draw, so each bucket gets a dot. */
export function fewBuckets(count: number): false | { r: number } {
    return count > 0 && count <= 2 ? { r: 3 } : false;
}

export function chartSummary(data: StatsData, buckets: Bucket[], range: RangeKey): string {
    const live = buckets.filter(b => b.total !== null);
    const peak = live.reduce<Bucket | null>((m, b) => (m === null || (b.total ?? 0) > (m.total ?? 0) ? b : m), null);
    const base = `Queries over the ${rangeDef(range).words}: ${formatCount(data.totals.total)} total, ${formatCount(data.totals.blocked)} blocked`;
    return peak && (peak.total ?? 0) > 0
        ? `${base}; busiest ${bucketUnitWord(data.bucketSeconds) === "daily" ? "day" : "bucket"} ${formatBucketLabel(peak.ts, data.bucketSeconds)} with ${formatCount(peak.total ?? 0)}`
        : base;
}

/** X3: one bucket as a column: all queries in slate with the blocked share overlaid in red. */
export function BucketColumn({ x = 0, y = 0, width = 0, height = 0, payload }: { x?: number; y?: number; width?: number; height?: number; payload?: Row }) {
    const all = payload?.barAll ?? 0;
    if (!payload || all <= 0) return null;
    const w = Math.max(width, 1);
    const h = Math.max(height, 1);
    const blocked = payload.barBlocked ?? 0;
    const bh = blocked > 0 ? Math.min(h, Math.max(1, (h * blocked) / all)) : 0;
    return (
        <g data-testid={payload.inProgress ? "bar-in-progress" : "bar-bucket"}>
            <rect
                x={x}
                y={y}
                width={w}
                height={h}
                fill="var(--stats-all)"
                fillOpacity={payload.inProgress ? 0.2 : 0.55}
                stroke={payload.inProgress ? "var(--stats-all)" : undefined}
                strokeDasharray={payload.inProgress ? "3 2" : undefined}
            />
            {bh > 0 && <rect x={x} y={y + h - bh} width={w} height={bh} fill="var(--stats-blocked)" fillOpacity={payload.inProgress ? 0.45 : 1} />}
        </g>
    );
}

function SeriesTooltip({ active, payload, bucketSeconds }: { active?: boolean; payload?: { payload: Row }[]; bucketSeconds: number }) {
    const row = active ? payload?.[0]?.payload : undefined;
    if (!row) return null;
    const total = row.all ?? row.allTail;
    const blocked = row.blocked ?? row.blockedTail;
    return (
        <div className="rounded-md border border-[var(--tailwind-colors-slate-600)] bg-[var(--shadcn-ui-app-background)] px-3 py-2 text-xs shadow-md tabular-nums">
            <div className="font-medium mb-1">{formatBucketLabel(row.ts, bucketSeconds)}</div>
            {total === null ? (
                <div className={mutedText}>Statistics were off</div>
            ) : (
                <>
                    <div>All queries: {formatCount(total ?? 0)}</div>
                    <div>Blocked: {formatCount(blocked ?? 0)}</div>
                </>
            )}
            {row.inProgress && <div className={mutedText}>In progress</div>}
        </div>
    );
}

/**
 * X9: a cursor and a label pinned on the time axis at the hovered bucket. Follows the value tooltip
 * (pointer over the plot, keyboard, tap) and, below the plot, the pointer's own position.
 */
function AxisHoverLabel({ pointerX, bucketSeconds, columns }: { pointerX: number | null; bucketSeconds: number; columns: number[] | null }) {
    const plot = usePlotArea();
    const scale = useXAxisScale();
    const snap = useXAxisInverseDataSnapScale();
    const tooltipActive = useIsTooltipActive();
    const activeLabel = useActiveTooltipLabel();
    const chartWidth = useChartWidth();
    if (!plot || !scale) return null;
    let ts: number | undefined;
    if (tooltipActive && typeof activeLabel === "number") ts = activeLabel;
    else if (pointerX !== null && pointerX >= plot.x && pointerX <= plot.x + plot.width) {
        const v = snap?.(pointerX);
        if (typeof v === "number") ts = v;
    }
    const left = ts === undefined ? undefined : scale(ts);
    // Columns sit in bands that start at scale(ts); points of the line chart sit on it.
    const second = columns && columns.length > 1 ? scale(columns[1]) : undefined;
    const first = columns ? scale(columns[0]) : undefined;
    const band = columns ? (second !== undefined && first !== undefined ? second - first : plot.width) : 0;
    const cx = left === undefined ? undefined : left + band / 2;
    if (ts === undefined || cx === undefined) return null;
    const text = formatBucketLabel(ts, bucketSeconds);
    const box = axisLabelBox(cx, text, 0, chartWidth ?? plot.x + plot.width);
    const bottom = plot.y + plot.height;
    return (
        <g aria-hidden="true" pointerEvents="none" data-testid="axis-hover-label">
            <line x1={cx} x2={cx} y1={plot.y} y2={bottom} stroke="var(--stats-axis)" strokeWidth={1} />
            <rect
                x={box.x}
                y={bottom + AXIS_BAND_HEIGHT + 2}
                width={box.width}
                height={HOVER_LABEL_HEIGHT}
                rx={4}
                fill="var(--shadcn-ui-app-background)"
                stroke="var(--tailwind-colors-slate-600)"
            />
            <text x={box.x + box.width / 2} y={bottom + AXIS_BAND_HEIGHT + 2 + HOVER_LABEL_HEIGHT / 2 + 4} textAnchor="middle" fontSize={11} fill="var(--tailwind-colors-slate-50)">
                {text}
            </text>
        </g>
    );
}

/** The element's width in px; undefined until measured or where ResizeObserver is missing. */
function useWidth(ref: React.RefObject<HTMLElement | null>): number | undefined {
    const [width, setWidth] = useState<number | undefined>(undefined);
    useEffect(() => {
        const el = ref.current;
        if (!el || typeof ResizeObserver === "undefined") return;
        const ro = new ResizeObserver(() => setWidth(el.clientWidth > 0 ? el.clientWidth : undefined));
        ro.observe(el);
        return () => ro.disconnect();
    });
    return width;
}

export function SeriesPanel({
    data,
    range,
    busy,
    offered,
    onRange,
}: {
    data: StatsData;
    range: RangeKey;
    busy?: boolean;
    offered?: RangeDef[];
    onRange?: (k: RangeKey) => void;
}) {
    const buckets = useMemo(() => buildBuckets(data), [data]);
    const rows = useMemo(() => toRows(buckets), [buckets]);
    const since = countingSince(data);
    const unit = bucketUnitWord(data.bucketSeconds);
    const summary = chartSummary(data, buckets, range);
    const animate = useChartEntrance(`${data.bucketSeconds}:${Math.round((data.toMs - data.fromMs) / 3_600_000)}`);
    const chartBox = useRef<HTMLDivElement>(null);
    const width = useWidth(chartBox);
    const [pointerX, setPointerX] = useState<number | null>(null);
    const trackPointer = (e: React.PointerEvent<HTMLDivElement>) => {
        const svg = chartBox.current?.querySelector("svg.recharts-surface");
        if (svg) setPointerX(e.clientX - svg.getBoundingClientRect().left);
    };
    const ticks = useMemo(() => axisTicks(range, rows.map(r => r.ts), width), [range, rows, width]);
    const tickLabels = useMemo(() => new Map(ticks.map(t => [t.ts, t.label])), [ticks]);
    const markerTs = since !== null ? rows.find(r => r.all !== null || r.allTail !== null)?.ts : undefined;
    const dot = fewBuckets(rows.filter(r => r.all !== null || r.allTail !== null).length);
    const shorter = shorterViewForStart(data, range, offered ?? []);

    return (
        <PanelShell title="Queries over time" options={LINE_BARS_TABLE} rememberAs="queries-over-time" className={cn(busy && "opacity-60")}>
            {view => (
                <>
                    {since !== null && (
                        <p className={cn("text-[13px]", mutedText)}>
                            Counting since {formatDateTime(since)}.
                            {shorter && onRange && (
                                <>
                                    {" "}
                                    <button
                                        type="button"
                                        className="underline text-[var(--tailwind-colors-rdns-600)] rounded focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--tailwind-colors-rdns-600)]"
                                        onClick={() => onRange(shorter)}
                                    >
                                        Show {rangeDef(shorter).words}
                                    </button>
                                </>
                            )}
                        </p>
                    )}
                    {view === "table" ? (
                        <StatsTable
                            scroll
                            caption={`${unit === "daily" ? "Daily buckets (UTC days)" : unit === "hourly" ? "Hourly buckets" : "15-minute buckets"}, newest first`}
                            columns={[
                                { label: "Time" },
                                { label: "Total", align: "right" },
                                { label: "Blocked", align: "right" },
                                { label: "DNSSEC", align: "right" },
                            ]}
                            rows={[...buckets]
                                .reverse()
                                .filter(b => b.total !== null)
                                .map(b => [
                                    <>
                                        {formatBucketLabel(b.ts, data.bucketSeconds)}
                                        {b.inProgress && <span className={cn("ml-1", mutedText)}>(in progress)</span>}
                                    </>,
                                    formatCount(b.total ?? 0),
                                    formatCount(b.blocked ?? 0),
                                    formatCount(b.dnssec ?? 0),
                                ])}
                        />
                    ) : (
                        <>
                            <div aria-hidden className={cn("flex flex-wrap gap-x-4 gap-y-1 text-[13px]", mutedText)}>
                                {view === "bars" ? (
                                    <>
                                        <span className="flex items-center gap-1.5">
                                            <i className="inline-block w-3 h-3 rounded-sm" style={{ background: "var(--stats-all)", opacity: 0.55 }} />
                                            All queries
                                        </span>
                                        <span className="flex items-center gap-1.5">
                                            <i className="inline-block w-3 h-3 rounded-sm" style={{ background: "var(--stats-blocked)" }} />
                                            Blocked
                                        </span>
                                        <span className="flex items-center gap-1.5">
                                            <i className="inline-block w-3 h-3 rounded-sm border border-dashed" style={{ borderColor: "var(--stats-axis)" }} />
                                            In progress
                                        </span>
                                    </>
                                ) : (
                                    <>
                                        <span className="flex items-center gap-1.5">
                                            <i className="inline-block w-4 border-t-2" style={{ borderColor: "var(--stats-all)" }} />
                                            All queries
                                        </span>
                                        <span className="flex items-center gap-1.5">
                                            <i className="inline-block w-4 border-t-2" style={{ borderColor: "var(--stats-blocked)" }} />
                                            Blocked
                                        </span>
                                        <span className="flex items-center gap-1.5">
                                            <i className="inline-block w-4 border-t-2 border-dashed" style={{ borderColor: "var(--stats-axis)" }} />
                                            In progress
                                        </span>
                                    </>
                                )}
                            </div>
                            <div
                                ref={chartBox}
                                role="group"
                                aria-roledescription="chart"
                                aria-label={summary}
                                onPointerMove={trackPointer}
                                onPointerDown={trackPointer}
                                onPointerLeave={e => e.pointerType === "mouse" && setPointerX(null)}
                            >
                                <ChartContainer config={config} className="aspect-auto h-[224px] md:h-[304px] w-full">
                                    <ComposedChart accessibilityLayer data={rows} barCategoryGap="10%" margin={{ top: 8, right: 8, bottom: HOVER_STRIP_HEIGHT, left: 0 }}>
                                        <CartesianGrid vertical={false} stroke="var(--stats-grid)" />
                                        <XAxis
                                            dataKey="ts"
                                            height={AXIS_BAND_HEIGHT}
                                            tickLine={false}
                                            axisLine={{ stroke: "var(--stats-axis)" }}
                                            padding={{ left: 0, right: 24 }}
                                            ticks={ticks.map(t => t.ts)}
                                            interval={0}
                                            tick={{ fill: "var(--stats-axis)" }}
                                            tickFormatter={(ts: number) => tickLabels.get(ts) ?? ""}
                                        />
                                        <YAxis
                                            width={40}
                                            tickLine={false}
                                            axisLine={false}
                                            allowDecimals={false}
                                            tick={{ fill: "var(--stats-axis)" }}
                                            tickFormatter={(v: number) => formatAxisCount(v)}
                                        />
                                        <Tooltip cursor={false} content={<SeriesTooltip bucketSeconds={data.bucketSeconds} />} />
                                        {markerTs !== undefined && (
                                            <ReferenceLine
                                                x={markerTs}
                                                stroke="var(--stats-axis)"
                                                strokeDasharray="2 3"
                                                label={{ value: "Statistics on", position: "insideTopLeft", fill: "var(--stats-axis)", fontSize: 11 }}
                                            />
                                        )}
                                        {view === "bars" ? (
                                            <Bar dataKey="barAll" shape={<BucketColumn />} maxBarSize={28} isAnimationActive={animate} animationDuration={ENTRANCE_MS} animationEasing="ease-out" />
                                        ) : (
                                            <>
                                            <Area dataKey="all" type="monotone" stroke="var(--stats-all)" fill="var(--stats-all-fill)" strokeWidth={1.5} dot={dot} isAnimationActive={animate} animationDuration={ENTRANCE_MS} animationEasing="ease-out" />
                                            <Area dataKey="blocked" type="monotone" stroke="var(--stats-blocked)" fill="var(--stats-blocked-fill)" strokeWidth={1.5} dot={dot} isAnimationActive={animate} animationDuration={ENTRANCE_MS} animationEasing="ease-out" />
                                            <Area dataKey="allTail" type="monotone" stroke="var(--stats-all)" fill="var(--stats-all-fill)" fillOpacity={0.5} strokeWidth={1.5} strokeDasharray="4 3" dot={dot} isAnimationActive={animate} animationDuration={ENTRANCE_MS} animationEasing="ease-out" />
                                            <Area dataKey="blockedTail" type="monotone" stroke="var(--stats-blocked)" fill="var(--stats-blocked-fill)" fillOpacity={0.5} strokeWidth={1.5} strokeDasharray="4 3" dot={dot} isAnimationActive={animate} animationDuration={ENTRANCE_MS} animationEasing="ease-out" />
                                            </>
                                        )}
                                        <AxisHoverLabel pointerX={pointerX} bucketSeconds={data.bucketSeconds} columns={view === "bars" ? rows.map(r => r.ts) : null} />
                                    </ComposedChart>
                                </ChartContainer>
                            </div>
                        </>
                    )}
                </>
            )}
        </PanelShell>
    );
}
