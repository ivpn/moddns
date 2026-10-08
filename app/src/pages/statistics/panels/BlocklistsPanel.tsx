import { formatCount } from "@/lib/formatStats";
import { cn } from "@/lib/utils";
import { BarList, PanelShell, StatsTable, mutedText } from "../primitives";
import { rangeDef, type RangeKey } from "../ranges";

export interface BlocklistItem {
    id: string;
    /** Display name from the API; the id stands in when it is missing. */
    name?: string;
    count: number;
}

const MULTI_LIST_NOTE = "A query blocked by several lists counts once for each.";

export function BlocklistsPanel({
    items,
    range,
    windowWords,
}: {
    items: BlocklistItem[];
    range: RangeKey;
    windowWords: string;
}) {
    const nameOf = (b: BlocklistItem) => b.name || b.id;
    return (
        <PanelShell title="Top blocklists" toggle={items.length > 0}>
            {view =>
                items.length === 0 ? (
                    <p className={cn("text-sm", mutedText)}>No blocked queries in this range.</p>
                ) : (
                    <>
                        {view === "table" ? (
                            <StatsTable
                                caption={`Top blocklists, ${windowWords} of query logs (${rangeDef(range).words})`}
                                columns={[{ label: "#" }, { label: "Blocklist" }, { label: "Queries", align: "right" }]}
                                rows={items.map((b, i) => [String(i + 1), <span key="n" className="break-words">{nameOf(b)}</span>, formatCount(b.count)])}
                            />
                        ) : (
                            <BarList
                                noun="blocklists"
                                rows={items.map(b => ({
                                    id: b.id,
                                    value: b.count,
                                    color: "var(--stats-blocked)",
                                    label: (
                                        <span className="truncate" title={nameOf(b)}>
                                            {nameOf(b)}
                                        </span>
                                    ),
                                    meta: <b>{formatCount(b.count)}</b>,
                                }))}
                            />
                        )}
                        <p className={cn("text-[13px] leading-[18px]", mutedText)}>{MULTI_LIST_NOTE}</p>
                    </>
                )
            }
        </PanelShell>
    );
}
