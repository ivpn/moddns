import { ChartColumn, List, SearchX } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import { LOGS_RETENTION_WORDS } from "@/components/data-collection/model";
import { countsState, expectedFirstCounts, type StatsData } from "./derive";
import { logsWindowCaption, nextLongerRange, rangeDef, type RangeDef, type RangeKey } from "./ranges";
import { formatDateTime, formatExpected } from "./time";
import { GateAction, GroupHeading, LimitedAccessNote, MessageCard, StatsCard, mutedText } from "./primitives";
import { KpiCards } from "./panels/KpiCards";
import { SeriesPanel } from "./panels/SeriesPanel";
import { ReasonsPanel } from "./panels/ReasonsPanel";
import { ProtocolsPanel } from "./panels/ProtocolsPanel";
import { DevicesPanel } from "./panels/DevicesPanel";
import { DomainsPanel, type DomainItem } from "./panels/DomainsPanel";
import { BlocklistsPanel, type BlocklistItem } from "./panels/BlocklistsPanel";
import { ClientsPanel, type ClientItem } from "./panels/ClientsPanel";
import type { ResourceState } from "./useApiResource";
import QuickRuleSheet, { type QuickRuleAction } from "@/components/custom-rules/QuickRuleSheet";
import { LIMITED_ACCESS_TOOLTIP, type QuickRuleApi } from "./QuickRuleButton";
import { customRulesPath } from "@/pages/custom_rules/utils";

export type GateId = "stats" | "logs" | "domains" | "ips";

export interface GateProps {
    restricted: boolean;
    laNoteId: string;
    onGate: (g: GateId) => void;
}

function Gate({
    id,
    title,
    text,
    action,
    icon,
    restricted,
    laNoteId,
    onGate,
}: GateProps & { id: GateId; title: string; text: string; action: string; icon: React.ReactNode }) {
    return (
        <MessageCard
            icon={icon}
            title={title}
            action={<GateAction label={action} restricted={restricted} describedBy={`${laNoteId}-${id}`} onClick={() => onGate(id)} />}
        >
            <p className={cn("text-sm leading-5", mutedText)}>{text}</p>
            {restricted && <LimitedAccessNote id={`${laNoteId}-${id}`} />}
        </MessageCard>
    );
}

function PanelSkeleton({ className }: { className?: string }) {
    return (
        <StatsCard aria-hidden>
            <Skeleton className={cn("h-40 w-full", className)} />
        </StatsCard>
    );
}

export function CountsSkeleton() {
    return (
        <div className="flex flex-col gap-4" data-testid="stats-counts-skeleton" aria-busy="true">
            <div className="grid grid-cols-2 lg:grid-cols-4 gap-3 md:gap-4">
                {[0, 1, 2, 3].map(i => (
                    <StatsCard key={i} aria-hidden className="p-4 md:p-4">
                        <Skeleton className="h-4 w-24" />
                        <Skeleton className="h-8 w-20 mt-2" />
                    </StatsCard>
                ))}
            </div>
            <PanelSkeleton className="h-[200px] md:h-[280px]" />
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
                <PanelSkeleton />
                <PanelSkeleton />
            </div>
            <PanelSkeleton />
        </div>
    );
}

export function LoadError({ status, onRetry, what }: { status: number | null; onRetry: () => void; what: string }) {
    return (
        <MessageCard
            icon={<SearchX className="w-6 h-6" />}
            title={status === 429 ? "Too many requests" : `Couldn't load ${what}`}
            action={
                <Button type="button" variant="outline" className="min-h-11 lg:min-h-9 self-start sm:self-auto" onClick={onRetry}>
                    Try again
                </Button>
            }
        >
            <p className={cn("text-sm", mutedText)} role="alert">
                {status === 429 ? "Too many requests. Try again in a minute." : "Something went wrong. Try again."}
            </p>
        </MessageCard>
    );
}

export interface CountsGroupProps extends GateProps {
    statsOn: boolean;
    stats: ResourceState<StatsData> & { reload: () => void };
    range: RangeKey;
    onRange: (k: RangeKey) => void;
    offered: RangeDef[];
    lastSeen: Map<string, number> | null;
    emptyHeadingRef: React.Ref<HTMLHeadingElement>;
    /** Clock for the collecting / no-queries-yet switch (P5, P23). */
    now: number;
}

export function CountsGroup(p: CountsGroupProps) {
    const { stats, range } = p;
    let body: React.ReactNode;

    if (!p.statsOn) {
        body = (
            <Gate
                {...p}
                id="stats"
                icon={<ChartColumn className="w-6 h-6" />}
                title="Counts are off"
                text="Query logs are on without statistics, so nothing is counted."
                action="Also keep statistics"
            />
        );
    } else if (stats.status === "error") {
        body = stats.errorStatus === 403 ? null : <LoadError status={stats.errorStatus} onRetry={stats.reload} what="statistics" />;
    } else if (stats.status !== "ready" || !stats.data) {
        body = <CountsSkeleton />;
    } else {
        const d = stats.data;
        const state = countsState(d);
        if (state === "no-queries-yet") {
            const expected = expectedFirstCounts(range, d.turnedOnAt, d.historyDeletedAt);
            const collecting = expected !== null && p.now < expected.collectingUntilMs;
            const start = d.enabledAt !== null ? formatDateTime(d.enabledAt) : null;
            body = (
                <MessageCard
                    icon={<ChartColumn className="w-6 h-6" />}
                    title={collecting ? "Collecting statistics" : "No queries counted yet"}
                    headingRef={p.emptyHeadingRef}
                >
                    <p className={cn("text-sm", mutedText)}>
                        {collecting
                            ? `${start ? `Statistics are on since ${start}. ` : ""}To protect your privacy, modDNS counts queries in 15-minute blocks, never one by one, so the first counts appear at about ${formatExpected(expected.displayMs, p.now)}. This page updates by itself.`
                            : `No queries have reached this profile${start ? ` since ${start}` : ""}. If your devices should be using it, check the device setup.`}
                    </p>
                    <Link
                        to="/setup"
                        className="text-sm underline text-[var(--tailwind-colors-rdns-600)] min-h-11 inline-flex items-center self-start rounded focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--tailwind-colors-rdns-600)]"
                    >
                        Check your device setup
                    </Link>
                </MessageCard>
            );
        } else if (state === "empty-range") {
            const next = nextLongerRange(range);
            body = (
                <MessageCard
                    icon={<ChartColumn className="w-6 h-6" />}
                    title={`No queries in the ${rangeDef(range).words}.`}
                    action={
                        next ? (
                            <Button type="button" variant="outline" className="min-h-11 lg:min-h-9 self-start sm:self-auto" onClick={() => p.onRange(next)}>
                                Show {rangeDef(next).words}
                            </Button>
                        ) : undefined
                    }
                />
            );
        } else {
            body = (
                <div
                    className={cn("flex flex-col gap-4 transition-opacity", stats.refreshing && "opacity-60")}
                    aria-busy={stats.refreshing || undefined}
                    data-testid="stats-counts-data"
                >
                    <KpiCards data={d} />
                    <SeriesPanel data={d} range={range} offered={p.offered} onRange={p.onRange} />
                    <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
                        <ReasonsPanel data={d} range={range} />
                        <ProtocolsPanel data={d} range={range} />
                    </div>
                    <DevicesPanel data={d} range={range} lastSeen={p.lastSeen} />
                </div>
            );
        }
    }

    return (
        <div className="flex flex-col gap-4" data-testid="stats-counts-group">
            <GroupHeading>Counts</GroupHeading>
            {body}
        </div>
    );
}

export type ListState<T> = ResourceState<T[]> & { reload: () => void };

export interface LogsGroupProps extends GateProps {
    logsOn: boolean;
    domainsOn: boolean;
    ipsOn: boolean;
    logsRetention: string | undefined;
    range: RangeKey;
    blocked: ListState<DomainItem>;
    resolved: ListState<DomainItem>;
    clients: ListState<ClientItem>;
    blocklists: ListState<BlocklistItem>;
    /** The active profile: a switch closes the quick rule sheet. */
    profileId: string;
}

function ListSlot<T>({ state, what, children }: { state: ListState<T>; what: string; children: (items: T[]) => React.ReactNode }) {
    if (state.status === "error") {
        return state.errorStatus === 403 ? null : <LoadError status={state.errorStatus} onRetry={state.reload} what={what} />;
    }
    if (state.status !== "ready" || !state.data) return <PanelSkeleton />;
    return (
        <div className={cn("transition-opacity", state.refreshing && "opacity-60")} aria-busy={state.refreshing || undefined}>
            {children(state.data)}
        </div>
    );
}

/** Domains a rule was added for in this browser session, keyed by profile and domain (P26). */
const addedRules = new Set<string>();

/** Forgets the session tags (tests). */
export const resetQuickRuleSession = () => addedRules.clear();

function useQuickRules(profileId: string, restricted: boolean, laNoteId: string) {
    const navigate = useNavigate();
    const [sheet, setSheet] = useState<{ domain: string; action: QuickRuleAction } | null>(null);
    const [, bump] = useState(0);
    const trigger = useRef<HTMLElement | null>(null);

    useEffect(() => setSheet(null), [profileId]);

    const api: QuickRuleApi = {
        restricted,
        describedBy: `${laNoteId}-quick-rule`,
        open: (domain, action, el) => {
            if (restricted) return;
            trigger.current = el;
            setSheet({ domain, action });
        },
        isAdded: domain => addedRules.has(`${profileId}|${domain}`),
    };
    const element = (
        <QuickRuleSheet
            open={sheet !== null}
            onOpenChange={open => !open && setSheet(null)}
            domain={sheet?.domain}
            defaultAction={sheet?.action}
            returnFocusTo={trigger}
            onCreated={() => {
                if (sheet) addedRules.add(`${profileId}|${sheet.domain}`);
                bump(n => n + 1);
            }}
            successToast={rule => ({
                description: "Past queries still count here - new queries follow the rule.",
                action: { label: "View rules", onClick: () => navigate(customRulesPath(rule.action)) },
            })}
            duplicateNotice={(value, action) => (
                <>
                    A custom rule for {value} already exists. Edit it in{" "}
                    <Link to={customRulesPath(action)} className="underline">
                        Custom rules
                    </Link>
                    .
                </>
            )}
        />
    );
    return { api, element };
}

export function LogsGroup(p: LogsGroupProps) {
    const quick = useQuickRules(p.profileId, p.restricted, p.laNoteId);
    const caption = p.logsOn ? logsWindowCaption(p.range, p.logsRetention) : null;
    const retentionWords = LOGS_RETENTION_WORDS[(p.logsRetention as keyof typeof LOGS_RETENTION_WORDS) ?? "1h"] ?? "1 hour";
    let body: React.ReactNode;

    if (!p.logsOn) {
        body = (
            <Gate
                {...p}
                id="logs"
                icon={<List className="w-6 h-6" />}
                title="Domains and clients come from query logs"
                text="Query logs are off for this profile."
                action="Turn on query logs"
            />
        );
    } else {
        body = (
            <>
                {p.domainsOn ? (
                    <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
                        <ListSlot state={p.blocked} what="top blocked domains">
                            {items => <DomainsPanel kind="blocked" items={items} range={p.range} windowWords={retentionWords} quickRule={quick.api} />}
                        </ListSlot>
                        <ListSlot state={p.resolved} what="top resolved domains">
                            {items => <DomainsPanel kind="resolved" items={items} range={p.range} windowWords={retentionWords} quickRule={quick.api} />}
                        </ListSlot>
                    </div>
                ) : (
                    <Gate
                        {...p}
                        id="domains"
                        icon={<List className="w-6 h-6" />}
                        title="Domains aren't logged"
                        text="Query logs are on, but domain names are not kept."
                        action="Log domains"
                    />
                )}
                <ListSlot state={p.blocklists} what="top blocklists">
                    {items => <BlocklistsPanel items={items} range={p.range} windowWords={retentionWords} />}
                </ListSlot>
                {p.ipsOn ? (
                    <ListSlot state={p.clients} what="top clients">
                        {items => <ClientsPanel items={items} windowWords={retentionWords} />}
                    </ListSlot>
                ) : (
                    <Gate
                        {...p}
                        id="ips"
                        icon={<List className="w-6 h-6" />}
                        title="Client IPs aren't logged"
                        text="Turn on client IP logging to see addresses and ISPs."
                        action="Log client IPs"
                    />
                )}
            </>
        );
    }

    return (
        <div className="flex flex-col gap-4" data-testid="stats-logs-group">
            <GroupHeading caption={caption}>From query logs</GroupHeading>
            {body}
            {p.logsOn && p.domainsOn && (
                <>
                    {p.restricted && (
                        <span id={quick.api.describedBy} className="sr-only">
                            {LIMITED_ACCESS_TOOLTIP}
                        </span>
                    )}
                    {quick.element}
                </>
            )}
        </div>
    );
}

