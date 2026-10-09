// The Statistics page.
//
// Source of truth: docs/specs/statistics-behaviour.md Sections P, K, D, X.

import { useCallback, useEffect, useId, useMemo, useRef, useState, type JSX } from "react";
import { useSearchParams } from "react-router-dom";
import type { ModelAccount, ModelProfile, ModelQueryLogTopBlocklists } from "@/api/client";
import api from "@/api/api";
import { useAppStore } from "@/store/general";
import { useSubscriptionGuard } from "@/hooks/useSubscriptionGuard";
import BetaEndingBanner from "@/components/BetaEndingBanner";
import LimitedAccessBanner from "@/components/LimitedAccessBanner";
import { DataCollectionDialog } from "@/components/data-collection/DataCollectionDialog";
import { fromProfile, type DataCollectionState } from "@/components/data-collection/model";
import type { InitialFocus } from "@/components/data-collection/DataCollectionControl";
import { countsState, expectedFirstCounts, normalizeStats, type StatsData } from "./derive";
import { profileStatsRetention, statsRetentionWords } from "@/components/data-collection/model";
import { LOGS_TIMESPAN, STATS_TIMESPAN, offeredRanges, parseRange, resolveRange, DEFAULT_RANGE, type RangeKey } from "./ranges";
import { formatDateTime, formatRangeCaption } from "./time";
import { useApiResource } from "./useApiResource";
import { CountsGroup, LogsGroup, type GateId } from "./groups";
import { StatsToolbar } from "./StatsToolbar";
import { RetentionControl } from "./RetentionControl";
import { StatsHero } from "./StatsHero";
import { mutedText } from "./primitives";
import { cn } from "@/lib/utils";
import { Info } from "lucide-react";
import type { DomainItem } from "./panels/DomainsPanel";
import type { ClientItem } from "./panels/ClientsPanel";
import type { BlocklistItem } from "./panels/BlocklistsPanel";

interface StatisticsProps {
    account?: ModelAccount;
    profiles: ModelProfile[];
}

const LIST_LIMIT = 10;

const GATE_DIALOG: Record<GateId, { focus: InitialFocus; pending: Partial<DataCollectionState> }> = {
    stats: { focus: "keep", pending: { level: "logs", keep: true } },
    logs: { focus: "level-logs", pending: { level: "logs", keep: true } },
    domains: { focus: "domains", pending: { level: "logs", domains: true } },
    ips: { focus: "ips", pending: { level: "logs", ips: true } },
};

function asDomains(raw: unknown): DomainItem[] {
    const items = (raw as { items?: unknown })?.items;
    return Array.isArray(items)
        ? items.flatMap(i => (typeof i?.domain === "string" ? [{ domain: i.domain as string, count: Number(i.count) || 0 }] : []))
        : [];
}

function asBlocklists(raw: ModelQueryLogTopBlocklists | undefined): BlocklistItem[] {
    return (raw?.items ?? []).flatMap(i =>
        i.blocklist_id ? [{ id: i.blocklist_id, name: i.name, count: Number(i.count) || 0 }] : [],
    );
}

function asClients(raw: unknown): ClientItem[] {
    const items = (raw as { items?: unknown })?.items;
    return Array.isArray(items)
        ? items.flatMap(i =>
              typeof i?.ip === "string"
                  ? [{ ip: i.ip as string, count: Number(i.count) || 0, asOrg: i.as_org ?? null, asn: i.asn ?? null, country: i.country ?? null }]
                  : [],
          )
        : [];
}

function asLastSeen(raw: unknown): Map<string, number> {
    const m = new Map<string, number>();
    if (Array.isArray(raw)) {
        for (const d of raw) {
            const t = Date.parse(d?.last_seen);
            if (typeof d?.device_id === "string" && !Number.isNaN(t)) m.set(d.device_id, t);
        }
    }
    return m;
}

export default function Statistics({ profiles }: StatisticsProps): JSX.Element {
    const storeProfile = useAppStore(s => s.activeProfile);
    const setActiveProfile = useAppStore(s => s.setActiveProfile);
    const profile = storeProfile ?? profiles[0] ?? null;
    const { isRestricted } = useSubscriptionGuard();
    const laNoteId = useId();
    const [params, setParams] = useSearchParams();
    const retention = profileStatsRetention(profile);
    const range = resolveRange(params.get("range"), retention);
    const offered = useMemo(() => offeredRanges(retention), [retention]);

    const setRange = useCallback(
        (k: RangeKey) =>
            setParams(
                p => {
                    const next = new URLSearchParams(p);
                    if (k === DEFAULT_RANGE) next.delete("range");
                    else next.set("range", k);
                    return next;
                },
                { replace: true },
            ),
        [setParams],
    );
    // K2: a view the retention no longer offers is corrected in the URL without a message.
    const askedRange = params.get("range");
    useEffect(() => {
        if (askedRange && parseRange(askedRange) !== range) setRange(range);
    }, [askedRange, range, setRange]);

    const pid = profile?.profile_id ?? null;
    const cfg = fromProfile(profile);
    const logsOn = !!profile?.settings?.logs?.enabled;
    const statsOn = !!profile?.settings?.statistics?.enabled;
    const domainsOn = logsOn && (profile?.settings?.logs?.log_domains ?? true);
    const ipsOn = logsOn && !!profile?.settings?.logs?.log_clients_ips;

    const [reload, setReload] = useState(0);
    // A retention change or a history deletion invalidates the server cache (J46), so refetch.
    const stats = useApiResource<StatsData>(pid, pid && statsOn ? `${pid}|${range}|${retention}|${reload}` : null, async () => {
        const res = await api.Client.statisticsApi.apiV1ProfilesIdStatisticsGet(pid as string, STATS_TIMESPAN[range]);
        return normalizeStats(res.data);
    });

    const logsTimespan = LOGS_TIMESPAN[range];
    const blocked = useApiResource<DomainItem[]>(pid, pid && domainsOn ? `${pid}|${range}|b` : null, async () =>
        asDomains((await api.Client.queryLogsApi.apiV1ProfilesIdLogsTopGet(pid as string, "blocked", logsTimespan, LIST_LIMIT)).data),
    );
    const resolved = useApiResource<DomainItem[]>(pid, pid && domainsOn ? `${pid}|${range}|r` : null, async () =>
        asDomains((await api.Client.queryLogsApi.apiV1ProfilesIdLogsTopGet(pid as string, "resolved", logsTimespan, LIST_LIMIT)).data),
    );
    const clients = useApiResource<ClientItem[]>(pid, pid && ipsOn ? `${pid}|${range}|c` : null, async () =>
        asClients((await api.Client.queryLogsApi.apiV1ProfilesIdLogsClientsGet(pid as string, logsTimespan, LIST_LIMIT)).data),
    );
    const blocklists = useApiResource<BlocklistItem[]>(pid, pid && logsOn ? `${pid}|${range}|bl` : null, async () =>
        asBlocklists((await api.Client.queryLogsApi.apiV1ProfilesIdLogsBlocklistsGet(pid as string, logsTimespan, LIST_LIMIT)).data),
    );
    // Once per profile: `last_seen` does not depend on the range and the endpoint is slow and rate-limited.
    const devices = useApiResource<Map<string, number>>(pid, pid && logsOn ? `${pid}|devices` : null, async () =>
        asLastSeen((await api.Client.queryLogsApi.apiV1ProfilesIdLogsDevicesGet(pid as string)).data),
    );

    // P17: the store says on but the API says off. Show OFF and revalidate once per profile.
    const revalidated = useRef<string | null>(null);
    const serverOff = statsOn && stats.status === "ready" && stats.data !== null && !stats.data.enabled;
    useEffect(() => {
        if (!serverOff || !pid || revalidated.current === pid) return;
        revalidated.current = pid;
        api.Client.profilesApi
            .apiV1ProfilesIdGet(pid)
            .then(res => res.data && setActiveProfile(res.data))
            .catch(() => undefined);
    }, [serverOff, pid, setActiveProfile]);

    const countsOn = statsOn && !serverOff;
    const anyCollection = countsOn || logsOn;

    const [gate, setGate] = useState<GateId | null>(null);
    const emptyHeading = useRef<HTMLHeadingElement>(null);
    const [focusEmpty, setFocusEmpty] = useState(false);
    useEffect(() => {
        if (focusEmpty && emptyHeading.current) {
            emptyHeading.current.focus();
            setFocusEmpty(false);
        }
    }, [focusEmpty, stats.status, stats.data]);

    // K10: while P5 shows, one silent refetch just after the first bucket flushes; no other polling.
    const [now, setNow] = useState(() => Date.now());
    const expected =
        stats.status === "ready" && stats.data && countsState(stats.data) === "no-queries-yet"
            ? expectedFirstCounts(range, stats.data.turnedOnAt, stats.data.historyDeletedAt)
            : null;
    const collectingUntil = expected?.collectingUntilMs ?? null;
    const refetchAt = expected ? expected.boundaryMs + 90_000 : null;
    const refetchStats = stats.refetch;
    useEffect(() => {
        if (collectingUntil === null || refetchAt === null) return;
        const t0 = Date.now();
        setNow(t0);
        if (t0 >= collectingUntil) return;
        const timers = [setTimeout(() => setNow(Date.now()), collectingUntil - t0)];
        if (refetchAt > t0) timers.push(setTimeout(refetchStats, refetchAt - t0));
        return () => timers.forEach(clearTimeout);
    }, [collectingUntil, refetchAt, refetchStats, pid, range]);

    const caption = useMemo(() => {
        const d = countsOn && stats.status === "ready" ? stats.data : null;
        if (!d || d.toMs <= d.fromMs) return null;
        return formatRangeCaption(d.fromMs, d.toMs, d.bucketSeconds);
    }, [countsOn, stats.status, stats.data, range]);

    if (!profile || !pid) return <div />;

    const lastSeen = logsOn && devices.status === "ready" ? devices.data : null;
    const statsData = stats.status === "ready" ? stats.data : null;

    return (
        <div className="flex flex-col w-full items-start gap-6 py-6 pt-8 md:p-8 min-w-0 bg-[var(--shadcn-ui-app-background)] [&_button:not(:disabled)]:cursor-pointer [&_select:not(:disabled)]:cursor-pointer">
            <BetaEndingBanner />
            <LimitedAccessBanner />
            <p className={cn("text-sm md:text-base leading-5 md:leading-6", mutedText)}>
                An overview of this profile's DNS queries over time.
            </p>

            {!anyCollection ? (
                <div className="w-full">
                    <StatsHero profile={profile} onSaved={() => setFocusEmpty(true)} />
                </div>
            ) : (
                <div className="flex flex-col gap-6 w-full min-w-0">
                    <StatsToolbar
                        ranges={offered}
                        range={range}
                        onRange={setRange}
                        caption={caption}
                        trailing={
                            countsOn && (
                                <RetentionControl
                                    profile={profile}
                                    onHistoryDeleted={() => {
                                        setReload(n => n + 1);
                                        setFocusEmpty(true);
                                    }}
                                />
                            )
                        }
                    />
                    {countsOn && statsData && (
                        <p className={cn("flex gap-1.5 text-sm", mutedText)} data-testid="stats-availability">
                            <Info className="w-4 h-4 mt-0.5 flex-none" aria-hidden />
                            <span>
                                {statsData.enabledAt !== null ? `Statistics on since ${formatDateTime(statsData.enabledAt)} · ` : "Statistics on · "}
                                kept for {statsRetentionWords(statsData.retention)}
                            </span>
                        </p>
                    )}
                    <CountsGroup
                        statsOn={countsOn}
                        stats={stats}
                        range={range}
                        onRange={setRange}
                        offered={offered}
                        lastSeen={lastSeen}
                        emptyHeadingRef={emptyHeading}
                        now={now}
                        restricted={isRestricted}
                        laNoteId={laNoteId}
                        onGate={setGate}
                    />
                    <LogsGroup
                        logsOn={logsOn}
                        domainsOn={domainsOn}
                        ipsOn={ipsOn}
                        logsRetention={cfg.retention}
                        range={range}
                        blocked={blocked}
                        resolved={resolved}
                        clients={clients}
                        blocklists={blocklists}
                        restricted={isRestricted}
                        laNoteId={laNoteId}
                        onGate={setGate}
                    />
                </div>
            )}

            {gate && (
                <DataCollectionDialog
                    open
                    onOpenChange={open => !open && setGate(null)}
                    profile={profile}
                    initialPending={GATE_DIALOG[gate].pending}
                    initialFocus={GATE_DIALOG[gate].focus}
                />
            )}
        </div>
    );
}
