import { formatCount } from "@/lib/formatStats";
import { BarList, PanelShell, StatsTable, mutedText } from "../primitives";
import { rangeDef, type RangeKey } from "../ranges";
import { stripTrailingDot } from "@/lib/utils";

export interface DomainItem {
    domain: string;
    count: number;
}

export function DomainsPanel({
    kind,
    items,
    range,
    windowWords,
}: {
    kind: "blocked" | "resolved";
    items: DomainItem[];
    range: RangeKey;
    windowWords: string;
}) {
    const title = kind === "blocked" ? "Top blocked domains" : "Top resolved domains";
    return (
        <PanelShell title={title} toggle={items.length > 0} defaultView="table">
            {view =>
                items.length === 0 ? (
                    <p className={`text-sm ${mutedText}`}>{`No ${kind} domains in this range.`}</p>
                ) : view === "table" ? (
                    <StatsTable
                        caption={`${title}, ${windowWords} of query logs (${rangeDef(range).words})`}
                        columns={[{ label: "#" }, { label: "Domain" }, { label: "Queries", align: "right" }]}
                        rows={items.map((d, i) => [String(i + 1), <span key="d" className="font-mono break-all">{stripTrailingDot(d.domain)}</span>, formatCount(d.count)])}
                    />
                ) : (
                    <BarList
                        scale="leader"
                        noun="domains"
                        rows={items.map(d => ({
                            id: d.domain,
                            value: d.count,
                            color: kind === "blocked" ? "var(--stats-blocked)" : "var(--stats-all)",
                            label: (
                                <span className="font-mono truncate" title={stripTrailingDot(d.domain)}>
                                    {stripTrailingDot(d.domain)}
                                </span>
                            ),
                            meta: <b>{formatCount(d.count)}</b>,
                        }))}
                    />
                )
            }
        </PanelShell>
    );
}
