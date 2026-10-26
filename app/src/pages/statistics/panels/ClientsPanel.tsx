import { formatCount } from "@/lib/formatStats";
import { PanelShell, ScrollRegion, StatsTable, TopListNote, mutedText } from "../primitives";
import { cn } from "@/lib/utils";

export interface ClientItem {
    ip: string;
    count: number;
    asOrg: string | null;
    asn: number | null;
    country: string | null;
}

const isp = (c: ClientItem) => (c.asOrg ? (c.asn ? `${c.asOrg} (AS${c.asn})` : c.asOrg) : "-");

export function ClientsPanel({ items, scopeIn }: { items: ClientItem[]; scopeIn: string }) {
    const noGeo = items.length > 0 && items.every(c => !c.asOrg && !c.asn && !c.country);
    return (
        <PanelShell title="Top clients" toggle={false}>
            {() =>
                items.length === 0 ? (
                    <p className={cn("text-sm", mutedText)}>{`No client addresses ${scopeIn}.`}</p>
                ) : (
                    <>
                        <div className="hidden sm:block">
                            <StatsTable
                                scroll
                                caption={`Top clients, from query logs ${scopeIn}`}
                                columns={[{ label: "IP address" }, { label: "Queries", align: "right" }, { label: "ISP (ASN)" }, { label: "Country" }]}
                                rows={items.map(c => [<span key="ip" className="font-mono">{c.ip}</span>, formatCount(c.count), isp(c), c.country ?? "-"])}
                            />
                        </div>
                        <ScrollRegion label="Top clients" className="sm:hidden">
                            <ul className="flex flex-col gap-3 pr-1">
                                {items.map(c => (
                                    <li key={c.ip} className="flex flex-col gap-0.5">
                                        <div className="flex justify-between gap-3 text-sm">
                                            <span className="font-mono min-w-0 break-all">{c.ip}</span>
                                            <span className="tabular-nums">{formatCount(c.count)}</span>
                                        </div>
                                        <div className={cn("text-[13px]", mutedText)}>
                                            {isp(c)} · {c.country ?? "-"}
                                        </div>
                                    </li>
                                ))}
                            </ul>
                        </ScrollRegion>
                        <TopListNote count={items.length} />
                        {noGeo && <p className={cn("text-[13px]", mutedText)}>ISP and country unavailable</p>}
                    </>
                )
            }
        </PanelShell>
    );
}
