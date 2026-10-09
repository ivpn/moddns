import { formatCompact, formatCount, formatPercent } from "@/lib/formatStats";
import { buildBuckets, type StatsData } from "../derive";
import { StatsCard, mutedText, titleText } from "../primitives";
import { cn } from "@/lib/utils";

const BINS = 28;

function binned(values: (number | null)[]): (number | null)[] {
    const size = Math.max(1, Math.round(values.length / BINS));
    const out: (number | null)[] = [];
    for (let i = 0; i < values.length; i += size) {
        const slice = values.slice(i, i + size).filter((v): v is number => v !== null);
        out.push(slice.length ? slice.reduce((a, b) => a + b, 0) : null);
    }
    return out;
}

/** X2: decorative; the card number is the content. */
function Sparkline({ values, color, fill }: { values: (number | null)[]; color: string; fill: string }) {
    const bins = binned(values);
    const max = Math.max(1, ...bins.map(v => v ?? 0));
    const n = bins.length;
    if (n < 2) return null;
    let line = "";
    let first = 0;
    let last = 0;
    bins.forEach((v, i) => {
        if (v === null) return;
        const x = (i / (n - 1)) * 100;
        const y = 30 - (v / max) * 26;
        line += `${line ? "L" : "M"}${x.toFixed(2)} ${y.toFixed(2)}`;
        if (!line.includes("L")) first = x;
        last = x;
    });
    if (!line) return null;
    const area = `${line}L${last.toFixed(2)} 32L${first.toFixed(2)} 32Z`;
    return (
        <svg className="hidden sm:block w-full h-8 mt-2" viewBox="0 0 100 32" preserveAspectRatio="none" aria-hidden focusable="false">
            <path d={area} fill={fill} stroke="none" />
            <path d={line} fill="none" stroke={color} strokeWidth="1.5" vectorEffect="non-scaling-stroke" />
        </svg>
    );
}

function Kpi({
    label,
    swatch,
    value,
    exact,
    sub,
    spark,
}: {
    label: string;
    swatch?: string;
    value: string;
    exact: string;
    sub?: string;
    spark?: React.ReactNode;
}) {
    return (
        <StatsCard className="p-4 md:p-4">
            <div className={cn("flex items-center gap-2 text-sm", mutedText)}>
                {swatch && <i aria-hidden className="inline-block w-2.5 h-2.5 rounded-sm" style={{ background: swatch }} />}
                {label}
            </div>
            <div
                className={cn("text-2xl font-bold leading-8 tabular-nums mt-1", titleText)}
                title={exact}
                aria-label={`${label}: ${exact}`}
            >
                {value}
            </div>
            {sub && <div className={cn("text-[13px] tabular-nums", mutedText)}>{sub}</div>}
            {spark}
        </StatsCard>
    );
}

/** X4: compact from 10,000 with the exact value in the accessible name. */
export function KpiCards({ data }: { data: StatsData }) {
    const { total, blocked, dnssec } = data.totals;
    const buckets = buildBuckets(data);
    const pctBlocked = formatPercent(blocked, total);
    return (
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-3 md:gap-4">
            <Kpi
                label="Total queries"
                swatch="var(--stats-all)"
                value={formatCompact(total)}
                exact={formatCount(total)}
                spark={<Sparkline values={buckets.map(b => b.total)} color="var(--stats-all)" fill="var(--stats-all-fill)" />}
            />
            <Kpi
                label="Blocked"
                swatch="var(--stats-blocked)"
                value={formatCompact(blocked)}
                exact={formatCount(blocked)}
                spark={<Sparkline values={buckets.map(b => b.blocked)} color="var(--stats-blocked)" fill="var(--stats-blocked-fill)" />}
            />
            <Kpi
                label="Blocked %"
                value={pctBlocked}
                exact={pctBlocked}
                sub={`${formatCount(blocked)} of ${formatCount(total)}`}
            />
            <Kpi
                label="DNSSEC-validated"
                swatch="var(--stats-dnssec)"
                value={formatCompact(dnssec)}
                exact={formatCount(dnssec)}
                sub={`${formatPercent(dnssec, total)} of total`}
                spark={<Sparkline values={buckets.map(b => b.dnssec)} color="var(--stats-dnssec)" fill="var(--stats-all-fill)" />}
            />
        </div>
    );
}
