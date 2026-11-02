import { formatCount } from "@/lib/formatStats";
import { cn } from "@/lib/utils";
import { BarList, PanelShell, StatsTable, TopListNote, mutedText } from "../primitives";

export interface BlocklistItem {
    id: string;
    /** Display name from the API; the id stands in when it is missing. */
    name?: string;
    count: number;
}

// The proxy stops at the first subscribed list that blocks a query and logs only that list's id.
const CREDIT_NOTE =
    "Each blocked query counts for one list only: the earliest-enabled list that blocks it. Lists that overlap with earlier ones can show few or no hits.";

export function BlocklistsPanel({
    items,
    scopeIn,
}: {
    items: BlocklistItem[];
    /** P27: "in the last hour" / "since …". */
    scopeIn: string;
}) {
    const nameOf = (b: BlocklistItem) => b.name || b.id;
    const caption = `Top blocklists, from query logs ${scopeIn}`;
    const hits = items.filter(b => b.count > 0).length;
    return (
        <PanelShell title="Top blocklists" toggle={hits > 0} defaultView="table">
            {view =>
                hits === 0 ? (
                    <p className={cn("text-sm", mutedText)}>{`No queries blocked by a blocklist ${scopeIn}.`}</p>
                ) : (
                    <>
                        {view === "table" ? (
                            <StatsTable
                                scroll
                                caption={caption}
                                columns={[{ label: "#" }, { label: "Blocklist" }, { label: "Queries", align: "right" }]}
                                rows={items.map((b, i) => [String(i + 1), <span key="n" className="break-words">{nameOf(b)}</span>, formatCount(b.count)])}
                            />
                        ) : (
                            <BarList
                                scale="leader"
                                scroll={caption}
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
                        <TopListNote count={hits} />
                        <p className={cn("text-[13px] leading-[18px]", mutedText)}>{CREDIT_NOTE}</p>
                    </>
                )
            }
        </PanelShell>
    );
}
