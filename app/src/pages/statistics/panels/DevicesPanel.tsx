import { formatCount, formatPercent } from "@/lib/formatStats";
import { deviceLabel, type StatsData, type StatsDevice } from "../derive";
import { rangeDef, type RangeKey } from "../ranges";
import { formatDateTime, formatRelative } from "../time";
import { BarList, PanelShell, StatsTable, mutedText } from "../primitives";
import { cn } from "@/lib/utils";

/** P20 */
export function DeviceName({ id }: { id: string }) {
    const l = deviceLabel(id);
    if (l.kind === "id") return <span className="font-mono truncate" title={id}>{id}</span>;
    return (
        <span title={l.hint} className="truncate">
            {l.text}
        </span>
    );
}

function seen(lastSeen: Map<string, number> | null, d: StatsDevice): { text: string; abs: string } | null {
    const ms = lastSeen?.get(d.id);
    return ms === undefined ? null : { text: formatRelative(ms), abs: formatDateTime(ms) };
}

/** `lastSeen` is null while logs are off: the column and the line are omitted. */
export function DevicesPanel({ data, range, lastSeen }: { data: StatsData; range: RangeKey; lastSeen: Map<string, number> | null }) {
    const devices = data.devices;
    return (
        <PanelShell title="Devices" toggle={devices.length > 0}>
            {view =>
                devices.length === 0 ? (
                    <p className={cn("text-sm", mutedText)}>No devices in this range.</p>
                ) : view === "table" ? (
                    <StatsTable
                        scroll
                        caption={`Queries per device, ${rangeDef(range).words}`}
                        columns={[
                            { label: "Device" },
                            { label: "Queries", align: "right" },
                            { label: "Blocked", align: "right" },
                            { label: "Blocked %", align: "right" },
                            ...(lastSeen ? [{ label: "Last seen in logs" }] : []),
                        ]}
                        rows={devices.map(d => {
                            const s = seen(lastSeen, d);
                            return [
                                <DeviceName key="n" id={d.id} />,
                                formatCount(d.total),
                                formatCount(d.blocked),
                                formatPercent(d.blocked, d.total),
                                ...(lastSeen ? [s ? <span key="s" title={s.abs}>{s.text}</span> : "—"] : []),
                            ];
                        })}
                    />
                ) : (
                    <>
                        <div aria-hidden className={cn("flex gap-4 text-[13px]", mutedText)}>
                            <span className="flex items-center gap-1.5">
                                <i className="inline-block w-2.5 h-2.5 rounded-sm" style={{ background: "var(--stats-all)" }} />
                                Resolved
                            </span>
                            <span className="flex items-center gap-1.5">
                                <i className="inline-block w-2.5 h-2.5 rounded-sm" style={{ background: "var(--stats-blocked)" }} />
                                Blocked
                            </span>
                        </div>
                        <BarList
                            noun="devices"
                            rows={devices.map(d => {
                                const s = seen(lastSeen, d);
                                return {
                                    id: d.id || "(none)",
                                    value: d.total,
                                    blocked: d.blocked,
                                    label: <DeviceName id={d.id} />,
                                    meta: (
                                        <>
                                            <b>{formatCount(d.total)}</b> · {formatCount(d.blocked)} blocked
                                            {s && (
                                                <span className={cn("block text-[12px]", mutedText)} title={s.abs}>
                                                    last seen in logs {s.text}
                                                </span>
                                            )}
                                        </>
                                    ),
                                };
                            })}
                        />
                    </>
                )
            }
        </PanelShell>
    );
}
