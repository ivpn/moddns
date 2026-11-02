import { formatCompact, formatCount, formatPercent } from "@/lib/formatStats";
import { PROTOCOLS, type StatsData } from "../derive";
import { rangeDef, type RangeKey } from "../ranges";
import { PanelShell, StatsTable, mutedText } from "../primitives";

function Donut({ slices, total }: { slices: { label: string; value: number; cat: number }[]; total: number }) {
    const cx = 70;
    const cy = 70;
    const outer = 64;
    const inner = 40;
    const pt = (r: number, a: number) => `${(cx + r * Math.cos(a)).toFixed(2)} ${(cy + r * Math.sin(a)).toFixed(2)}`;
    let angle = -Math.PI / 2;
    const label = `Protocols: ${slices.map(s => `${s.label} ${formatPercent(s.value, total)}`).join(", ")}`;
    return (
        <svg className="w-28 h-28 flex-none" viewBox="0 0 140 140" role="img" aria-label={label}>
            {slices.map(s => {
                const sweep = (s.value / total) * Math.PI * 2;
                const end = angle + sweep;
                const large = sweep > Math.PI ? 1 : 0;
                // A full circle needs two arcs; a single arc with identical endpoints draws nothing.
                const d =
                    slices.length === 1
                        ? `M${pt(outer, -Math.PI / 2)}A${outer} ${outer} 0 1 1 ${pt(outer, Math.PI / 2)}A${outer} ${outer} 0 1 1 ${pt(outer, -Math.PI / 2)}ZM${pt(inner, -Math.PI / 2)}A${inner} ${inner} 0 1 0 ${pt(inner, Math.PI / 2)}A${inner} ${inner} 0 1 0 ${pt(inner, -Math.PI / 2)}Z`
                        : `M${pt(outer, angle)}A${outer} ${outer} 0 ${large} 1 ${pt(outer, end)}L${pt(inner, end)}A${inner} ${inner} 0 ${large} 0 ${pt(inner, angle)}Z`;
                angle = end;
                return (
                    <path
                        key={s.label}
                        d={d}
                        fillRule="evenodd"
                        fill={`var(--stats-cat-${s.cat})`}
                        stroke="var(--variable-collection-surface, var(--shadcn-ui-app-background))"
                        strokeWidth="2"
                        strokeLinejoin="round"
                    />
                );
            })}
            <text x="70" y="68" textAnchor="middle" fontSize="16" fontWeight="700" className="tabular-nums" fill="currentColor">
                {formatCompact(total)}
            </text>
            <text x="70" y="86" textAnchor="middle" fontSize="11" fill="var(--stats-axis)">
                queries
            </text>
        </svg>
    );
}

export function ProtocolsPanel({ data, range }: { data: StatsData; range: RangeKey }) {
    const slices = PROTOCOLS.map(p => ({ label: p.label, cat: p.cat, value: data.protocols[p.key] })).filter(s => s.value > 0);
    const total = slices.reduce((s, p) => s + p.value, 0);

    return (
        <PanelShell title="Protocols" toggle={total > 0}>
            {view =>
                total === 0 ? (
                    <p className={`text-sm ${mutedText}`}>No encrypted queries in this range.</p>
                ) : view === "table" ? (
                    <StatsTable
                        caption={`Queries by protocol, ${rangeDef(range).words}`}
                        columns={[{ label: "Protocol" }, { label: "Queries", align: "right" }, { label: "Share", align: "right" }]}
                        rows={slices.map(s => [s.label, formatCount(s.value), formatPercent(s.value, total)])}
                    />
                ) : (
                    <div className="flex items-center gap-6">
                        <Donut slices={slices} total={total} />
                        <ul className="flex flex-col gap-2 flex-1 min-w-0 text-sm">
                            {slices.map(s => (
                                <li key={s.label} className="flex items-center gap-2">
                                    <i aria-hidden className="inline-block w-2.5 h-2.5 rounded-sm flex-none" style={{ background: `var(--stats-cat-${s.cat})` }} />
                                    <span>{s.label}</span>
                                    <span className="ml-auto tabular-nums">
                                        {formatPercent(s.value, total)} · {formatCount(s.value)}
                                    </span>
                                </li>
                            ))}
                        </ul>
                    </div>
                )
            }
        </PanelShell>
    );
}
