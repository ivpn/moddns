import { formatCount, formatPercent } from "@/lib/formatStats";
import { REASON_CLASSES, type StatsData } from "../derive";
import { rangeDef, type RangeKey } from "../ranges";
import { BarList, PanelShell, StatsTable, mutedText } from "../primitives";

export function ReasonsPanel({ data, range }: { data: StatsData; range: RangeKey }) {
    const rows = REASON_CLASSES.map(c => ({ ...c, value: data.reasons[c.key] }))
        .filter(r => r.value > 0)
        .sort((a, b) => b.value - a.value);
    const total = rows.reduce((s, r) => s + r.value, 0);

    return (
        <PanelShell title="Blocked by reason" toggle={total > 0}>
            {view =>
                total === 0 ? (
                    <p className={`text-sm ${mutedText}`}>Nothing was blocked in this range.</p>
                ) : view === "table" ? (
                    <StatsTable
                        caption={`Blocked queries by reason, ${rangeDef(range).words}`}
                        columns={[{ label: "Reason" }, { label: "Blocked", align: "right" }, { label: "Share", align: "right" }]}
                        rows={rows.map(r => [r.label, formatCount(r.value), formatPercent(r.value, total)])}
                    />
                ) : (
                    <BarList
                        collapseAfter={REASON_CLASSES.length}
                        rows={rows.map(r => ({
                            id: r.key,
                            value: r.value,
                            color: `var(--stats-cat-${r.cat})`,
                            label: (
                                <>
                                    <i aria-hidden className="inline-block w-2.5 h-2.5 rounded-sm flex-none" style={{ background: `var(--stats-cat-${r.cat})` }} />
                                    <span className="truncate">{r.label}</span>
                                </>
                            ),
                            meta: (
                                <>
                                    <b>{formatCount(r.value)}</b> · {formatPercent(r.value, total)}
                                </>
                            ),
                        }))}
                    />
                )
            }
        </PanelShell>
    );
}
