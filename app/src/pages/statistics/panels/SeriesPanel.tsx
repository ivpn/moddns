import { useMemo } from "react";
import { Area, CartesianGrid, ComposedChart, ReferenceLine, Tooltip, XAxis, YAxis } from "recharts";
import { ChartContainer, type ChartConfig } from "@/components/ui/chart";
import { formatAxisCount, formatCount } from "@/lib/formatStats";
import { buildBuckets, countingSince, type Bucket, type StatsData } from "../derive";
import { rangeDef, type RangeKey } from "../ranges";
import { bucketUnitWord, formatBucketLabel, formatBucketTick, formatDateTime } from "../time";
import { PanelShell, StatsTable, mutedText } from "../primitives";
import { cn } from "@/lib/utils";
import { ENTRANCE_MS, useChartEntrance } from "../useChartEntrance";

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
            inProgress: b.inProgress,
        };
    });
}

export function chartSummary(data: StatsData, buckets: Bucket[], range: RangeKey): string {
    const live = buckets.filter(b => b.total !== null);
    const peak = live.reduce<Bucket | null>((m, b) => (m === null || (b.total ?? 0) > (m.total ?? 0) ? b : m), null);
    const base = `Queries over the ${rangeDef(range).words}: ${formatCount(data.totals.total)} total, ${formatCount(data.totals.blocked)} blocked`;
    return peak && (peak.total ?? 0) > 0
        ? `${base}; busiest ${bucketUnitWord(data.bucketSeconds) === "daily" ? "day" : "bucket"} ${formatBucketLabel(peak.ts, data.bucketSeconds)} with ${formatCount(peak.total ?? 0)}`
        : base;
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

export function SeriesPanel({ data, range, busy }: { data: StatsData; range: RangeKey; busy?: boolean }) {
    const buckets = useMemo(() => buildBuckets(data), [data]);
    const rows = useMemo(() => toRows(buckets), [buckets]);
    const since = countingSince(data);
    const unit = bucketUnitWord(data.bucketSeconds);
    const unitPlural = unit === "daily" ? "days" : unit === "hourly" ? "hours" : "15-minute buckets";
    const summary = chartSummary(data, buckets, range);
    const animate = useChartEntrance(`${data.bucketSeconds}:${Math.round((data.toMs - data.fromMs) / 3_600_000)}`);
    const markerTs = since !== null ? rows.find(r => r.all !== null || r.allTail !== null)?.ts : undefined;

    return (
        <PanelShell title="Queries over time" className={cn(busy && "opacity-60")}>
            {view => (
                <>
                    {since !== null && (
                        <p className={cn("text-[13px]", mutedText)}>
                            Counting since {formatDateTime(since)}. Earlier {unitPlural} are blank, not zero.
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
                            </div>
                            <div role="group" aria-roledescription="chart" aria-label={summary}>
                                <ChartContainer config={config} className="aspect-auto h-[200px] md:h-[280px] w-full">
                                    <ComposedChart accessibilityLayer data={rows} margin={{ top: 8, right: 8, bottom: 0, left: 0 }}>
                                        <CartesianGrid vertical={false} stroke="var(--stats-grid)" />
                                        <XAxis
                                            dataKey="ts"
                                            tickLine={false}
                                            axisLine={{ stroke: "var(--stats-axis)" }}
                                            minTickGap={28}
                                            interval="preserveStartEnd"
                                            tick={{ fill: "var(--stats-axis)" }}
                                            tickFormatter={(ts: number) => formatBucketTick(ts, data.bucketSeconds, range === "12m")}
                                        />
                                        <YAxis
                                            width={40}
                                            tickLine={false}
                                            axisLine={false}
                                            allowDecimals={false}
                                            tick={{ fill: "var(--stats-axis)" }}
                                            tickFormatter={(v: number) => formatAxisCount(v)}
                                        />
                                        <Tooltip content={<SeriesTooltip bucketSeconds={data.bucketSeconds} />} />
                                        {markerTs !== undefined && (
                                            <ReferenceLine
                                                x={markerTs}
                                                stroke="var(--stats-axis)"
                                                strokeDasharray="2 3"
                                                label={{ value: "Statistics on", position: "insideTopLeft", fill: "var(--stats-axis)", fontSize: 11 }}
                                            />
                                        )}
                                        <Area dataKey="all" type="monotone" stroke="var(--stats-all)" fill="var(--stats-all-fill)" strokeWidth={1.5} dot={false} isAnimationActive={animate} animationDuration={ENTRANCE_MS} animationEasing="ease-out" />
                                        <Area dataKey="blocked" type="monotone" stroke="var(--stats-blocked)" fill="var(--stats-blocked-fill)" strokeWidth={1.5} dot={false} isAnimationActive={animate} animationDuration={ENTRANCE_MS} animationEasing="ease-out" />
                                        <Area dataKey="allTail" type="monotone" stroke="var(--stats-all)" fill="var(--stats-all-fill)" fillOpacity={0.5} strokeWidth={1.5} strokeDasharray="4 3" dot={false} isAnimationActive={animate} animationDuration={ENTRANCE_MS} animationEasing="ease-out" />
                                        <Area dataKey="blockedTail" type="monotone" stroke="var(--stats-blocked)" fill="var(--stats-blocked-fill)" fillOpacity={0.5} strokeWidth={1.5} strokeDasharray="4 3" dot={false} isAnimationActive={animate} animationDuration={ENTRANCE_MS} animationEasing="ease-out" />
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
