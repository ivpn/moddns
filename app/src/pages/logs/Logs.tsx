import { useEffect, useRef, useState, useCallback, useMemo, type JSX } from "react";
import type { AxiosError } from "axios";

interface NetworkError extends AxiosError { code?: string; }

import { toast } from "sonner";

import type { ModelAccount, ModelProfile, ModelQueryLog } from "@/api/client";
import Filters from "./Filters";
import NoLogs from "./NoLogs";
import LogsNotActive from "./LogsNotActive";
import QueryLogCard from "./QueryLogCard";
import QuickRuleSheet, { type QuickRuleAction } from "./QuickRuleSheet";
import { consolidateLogs, toSingletonGroup } from "@/lib/consolidateLogs";
import { computeNewQueryLogs } from "@/lib/queryLogsDiff";
import { refreshIntervalMsFor, type RefreshIntervalKey } from "@/lib/consts";
import api from "@/api/api";
import { useAppStore } from "@/store/general";
import { Skeleton } from "@/components/ui/skeleton";
import { ArrowUp, Info, X } from "lucide-react";
import { useScreenDetector } from "@/hooks/useScreenDetector";
import { useSubscriptionGuard } from "@/hooks/useSubscriptionGuard";
import LimitedAccessBanner from "@/components/LimitedAccessBanner";
import BetaEndingBanner from "@/components/BetaEndingBanner";

const QUERY_LIMIT = 25;

interface QueryLogsProps {
    account: ModelAccount;
    profiles: ModelProfile[];
}

const QueryLogs = ({ profiles }: QueryLogsProps): JSX.Element => {
    const { isRestricted } = useSubscriptionGuard();
    const [logs, setLogs] = useState<ModelQueryLog[]>([]);
    const [page, setPage] = useState(1);
    const [hasMore, setHasMore] = useState(true);
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);
    // Auto-refresh cadence, selected via the split refresh button's interval menu
    // ("off" = disabled). Session-only by design — never persisted.
    const [refreshIntervalKey, setRefreshIntervalKey] = useState<RefreshIntervalKey>("off");
    const refreshIntervalMs = refreshIntervalMsFor(refreshIntervalKey);
    const isAutoRefreshing = refreshIntervalMs !== null;
    const [refreshTrigger, setRefreshTrigger] = useState(0); // Add trigger for forced refresh
    // Fade choreography for page-1 loads: true = list held at opacity-0. Starts true so the
    // initial load fades in. Set true by every refresh/filter trigger; cleared ONLY by the
    // dedicated effect below once loading settles — never by the fetch effect's cleanup, so
    // a mid-refresh page bump cannot cancel the fade-in and strand the list invisible.
    const [isListFading, setIsListFading] = useState(true);
    const [isQuickRuleSheetOpen, setIsQuickRuleSheetOpen] = useState(false);
    const [quickRuleDomain, setQuickRuleDomain] = useState<string | undefined>(undefined);
    const [quickRuleDefaultAction, setQuickRuleDefaultAction] = useState<QuickRuleAction>("denylist");

    // New entries found by the auto-refresh background tick, staged behind the
    // "N new queries" pill instead of disturbing the list. Recomputed wholesale
    // against the displayed list on every tick. pendingOverflow: the tick's full
    // page shared nothing with the list — prepending would leave a gap.
    const [pendingLogs, setPendingLogs] = useState<ModelQueryLog[]>([]);
    const [pendingOverflow, setPendingOverflow] = useState(false);

    // Expansion state of cards, lifted here (keyed by group identity, not React key)
    // so open cards survive the remounts caused by refreshes and pill merges.
    const [expandedKeys, setExpandedKeys] = useState<Set<string>>(new Set());

    // Group identities revealed by the last pill click. A prepend remounts EVERY card
    // (React keys embed the list index), so the entry animation must be scoped to the
    // groups that are actually new — not everything that remounted.
    const [freshIdentities, setFreshIdentities] = useState<Set<string>>(new Set());

    // Search input (uncommitted while typing) and committed value that triggers requests
    const [searchInputValue, setSearchInputValue] = useState("");
    const [committedSearchValue, setCommittedSearchValue] = useState("");
    const [filterValue, setFilterValue] = useState("all");
    const [sortValue, setSortValue] = useState("created");
    const [timespanValue, setTimespanValue] = useState<string | undefined>(undefined);
    const [deviceIdValue, setDeviceIdValue] = useState<string | undefined>(undefined);

    // Maintain a separate list of all available device IDs (not filtered by current selection)
    const [allAvailableDeviceIds, setAllAvailableDeviceIds] = useState<string[]>([]);

    // id→name catalogs for enriching query-log reasons (blocklist/service ids). Loaded once on
    // mount; failures degrade gracefully to raw ids and must never block logs from rendering.
    const [blocklistNames, setBlocklistNames] = useState<Record<string, string>>({});
    const [serviceNames, setServiceNames] = useState<Record<string, string>>({});

    // One-time mobile hint teaching that a row is tappable (there is no visible chevron).
    // Dismissed on the ✕ or after the first row expand. Persisted in the shared "moddns-storage"
    // zustand store (alongside the other one-time dismissals) so it never reappears.
    const expandHintDismissed = useAppStore((state) => state.logsExpandHintDismissed);
    const setLogsExpandHintDismissed = useAppStore((state) => state.setLogsExpandHintDismissed);
    const dismissExpandHint = useCallback(() => {
        setLogsExpandHintDismissed(true);
    }, [setLogsExpandHintDismissed]);

    // Compose filters object for API
    const filters = {
        Limit: QUERY_LIMIT,
        Status: filterValue === "all" ? undefined : filterValue,
        Timespan: { Value: timespanValue === "all" ? undefined : timespanValue },
        Search: committedSearchValue,
        Sort: sortValue,
    };

    const observer = useRef<IntersectionObserver | null>(null);
    const previousProfileIdRef = useRef<string | undefined>(undefined);
    // Mirror of `logs` for reads inside the background tick, which runs outside the
    // render cycle (setInterval) and must diff against the list as displayed NOW.
    const logsRef = useRef<ModelQueryLog[]>([]);
    const bgFetchInFlight = useRef(false);
    const lastLogRef = useCallback(
        (node: HTMLDivElement | null) => {
            if (loading) return;
            if (observer.current) observer.current.disconnect();
            observer.current = new window.IntersectionObserver(entries => {
                if (entries[0].isIntersecting && hasMore) {
                    setPage(prev => prev + 1);
                }
            });
            if (node) observer.current.observe(node);
        },
        [loading, hasMore]
    );

    // Consolidate sequential duplicate rows (issue #161). Runs over the FULL accumulated
    // logs array, so groups that straddle a pagination boundary heal automatically once the
    // next page appends. Sequential adjacency is only meaningful under the default time sort;
    // under domain/client_ip sort every row stays un-merged (identical to before this feature).
    const displayGroups = useMemo(
        () => (sortValue === "created" ? consolidateLogs(logs) : logs.map(toSingletonGroup)),
        [logs, sortValue]
    );

    const activeProfile = useAppStore((state) => state.activeProfile);
    const { setActiveProfile } = useAppStore();

    // Set active profile from profiles prop when component loads
    useEffect(() => {
        if (profiles.length > 0) {
            if (activeProfile?.profile_id) {
                // Find the profile with matching ID from profiles prop and overwrite activeProfile
                const matchingProfile = profiles.find(profile => profile.profile_id === activeProfile.profile_id);
                if (matchingProfile && JSON.stringify(matchingProfile) !== JSON.stringify(activeProfile)) {
                    // Only update if the profile data has actually changed
                    setActiveProfile(matchingProfile);
                }
            } else {
                // If no active profile, set the first one
                setActiveProfile(profiles[0]);
            }
        }
    // eslint-disable-next-line react-hooks/exhaustive-deps -- activeProfile is intentionally excluded to avoid re-running this effect when the profile object changes (which this effect itself triggers via setActiveProfile)
    }, [profiles, setActiveProfile]);

    // Load blocklist + service catalogs once to resolve reason ids to human names in the
    // expandable log card. Best-effort: on failure the maps stay empty and reasons fall back
    // to raw ids — never block logs on catalog load.
    useEffect(() => {
        let cancelled = false;
        const loadCatalogs = async () => {
            try {
                const [blocklistsResp, servicesResp] = await Promise.all([
                    api.Client.blocklistsApi.apiV1BlocklistsGet(),
                    api.Client.servicesApi.apiV1ServicesGet(),
                ]);
                if (cancelled) return;
                const blMap: Record<string, string> = {};
                (blocklistsResp.data || []).forEach(bl => {
                    if (bl.blocklist_id) blMap[bl.blocklist_id] = bl.name;
                });
                setBlocklistNames(blMap);

                const svcMap: Record<string, string> = {};
                (servicesResp.data?.services || []).forEach(svc => {
                    if (svc.id && svc.name) svcMap[svc.id] = svc.name;
                });
                setServiceNames(svcMap);
            } catch {
                // Leave maps empty; reasons degrade to raw ids.
            }
        };
        loadCatalogs();
        return () => { cancelled = true; };
    }, []);

    const handleOpenQuickRule = useCallback((domain?: string, defaultAction: QuickRuleAction = "denylist") => {
        if (!domain) return;
        if (isRestricted) return; // POST custom_rules is blocked in Limited Access / Pending Delete
        setQuickRuleDomain(domain);
        setQuickRuleDefaultAction(defaultAction);
        setIsQuickRuleSheetOpen(true);
    }, [isRestricted]);

    const handleQuickRuleSheetChange = useCallback((nextOpen: boolean) => {
        setIsQuickRuleSheetOpen(nextOpen);
        if (!nextOpen) {
            setQuickRuleDomain(undefined);
        }
    }, []);

    useEffect(() => {
        logsRef.current = logs;
    }, [logs]);

    // Reset logs, device IDs, staged entries and page when committed filters change
    useEffect(() => {
        setLogs([]);
        setPage(1);
        setHasMore(true);
        setAllAvailableDeviceIds([]);
        setIsListFading(true);
        setPendingLogs([]);
        setPendingOverflow(false);
        setExpandedKeys(new Set());
        setFreshIdentities(new Set());
    }, [committedSearchValue, filterValue, sortValue, timespanValue, deviceIdValue]);

    // Fade-in: once no fetch is in flight, release the fade after a short delay so the
    // swapped-in content is committed before the opacity transition starts. Lives in its own
    // effect keyed only on [isListFading, loading]: a new refresh (loading goes true) merely
    // postpones it, and it always converges to visible once loading settles.
    useEffect(() => {
        if (!isListFading || loading) return;
        const fadeIn = setTimeout(() => setIsListFading(false), 100);
        return () => clearTimeout(fadeIn);
    }, [isListFading, loading]);

    useEffect(() => {
        const currentId = activeProfile?.profile_id;
        if (previousProfileIdRef.current && previousProfileIdRef.current !== currentId) {
            setIsQuickRuleSheetOpen(false);
            setQuickRuleDomain(undefined);
            setPendingLogs([]);
            setPendingOverflow(false);
            setExpandedKeys(new Set());
            setFreshIdentities(new Set());
        }
        previousProfileIdRef.current = currentId;
    }, [activeProfile?.profile_id]);

    const commitSearch = useCallback(() => {
        setCommittedSearchValue(prev => prev === searchInputValue ? prev : searchInputValue);
    }, [searchInputValue]);

    const toggleCardExpanded = useCallback((identity: string) => {
        setExpandedKeys(prev => {
            const next = new Set(prev);
            if (next.has(identity)) next.delete(identity);
            else next.add(identity);
            return next;
        });
    }, []);

    // Fetch logs and then fetch logos for the batch
    useEffect(() => {
        let cancelled = false;
        const fetchLogs = async () => {
            // Don't fetch if no active profile
            if (!activeProfile?.profile_id) {
                setLoading(false);
                return;
            }

            setLoading(true);
            setError(null);

            try {
                // Status is already handled in filters.Status
                // Use expanded limit on first page to gather more device IDs; subsequent pages respect configured limit
                const effectiveLimit = page === 1 ? 100 : filters.Limit;
                const searchParam = committedSearchValue || undefined;
                const response = await api.Client.queryLogsApi.apiV1ProfilesIdLogsGet(
                    activeProfile.profile_id,
                    page,
                    effectiveLimit,
                    filters.Status,
                    filters.Timespan.Value,
                    deviceIdValue || undefined,
                    searchParam,
                    sortValue
                );
                // A newer fetch (or unmount) superseded this one — drop the stale response
                // instead of letting it overwrite fresher state.
                if (cancelled) return;
                if (response.status === 200) {
                    const newLogs = response.data || [];

                    // Set logs and update state
                    setLogs(prev => (page === 1 ? newLogs : [...prev, ...newLogs]));
                    setHasMore(newLogs.length === effectiveLimit);

                    // Accumulate unique device IDs progressively
                    setAllAvailableDeviceIds(prev => {
                        const merged = new Set(prev);
                        response.data.forEach(log => {
                            if (log.device_id) merged.add(log.device_id);
                        });
                        return Array.from(merged).sort();
                    });
                } else {
                    setHasMore(false);
                }
            } catch (err: unknown) {
                if (cancelled) return;
                // Handle different HTTP error codes with specific messages
                let errorMessage = "Failed to load logs";
                const httpErr = err as AxiosError & { code?: string };
                const status = httpErr.response?.status;
                if (status === 403) {
                    // Account is cut off (inactive / pending_delete): logs are not
                    // entitled in these states. AccountCutoffGuard redirects to
                    // /account-preferences, so surface no toast here — matching how
                    // the other restricted pages behave during cut-off.
                    setHasMore(false);
                    return;
                } else if (status === 429) {
                    errorMessage = "Too many requests. Please wait a moment before trying again.";
                } else if (status === 500) {
                    errorMessage = "Server error occurred while loading logs.";
                } else if (status === 404) {
                    errorMessage = "Profile not found.";
                } else if ((httpErr as NetworkError)?.code === 'NETWORK_ERROR' || !httpErr.response) {
                    errorMessage = "Network error. Please check your connection.";
                }

                toast.error(errorMessage);
                setHasMore(false);
            } finally {
                if (!cancelled) setLoading(false);
            }
        };
        fetchLogs();
        return () => {
            cancelled = true;
        };
    // eslint-disable-next-line react-hooks/exhaustive-deps -- committedSearchValue and sortValue are consumed via the `filters` object and `refreshTrigger`; adding them directly would cause redundant re-fetches since the filters object already captures their derived values
    }, [page, filters.Limit, filters.Status, filters.Timespan.Value, filters.Search, filters.Sort, activeProfile, refreshTrigger, deviceIdValue]);

    // Spin the refresh icon for at least a half rotation (500ms) per manual refresh —
    // tied to `loading` alone, a fast response ends the spin after a couple of frames
    // and the click appears to do nothing.
    const [manualSpinActive, setManualSpinActive] = useState(false);
    const manualSpinTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
    useEffect(() => () => {
        if (manualSpinTimer.current) clearTimeout(manualSpinTimer.current);
    }, []);

    // Handle manual (one-shot) refresh: page-1 replace, discarding staged entries.
    const handleRefresh = () => {
        setPage(1);
        setIsListFading(true);
        setPendingLogs([]);
        setPendingOverflow(false);
        setFreshIdentities(new Set());
        setRefreshTrigger(prev => prev + 1);
        setManualSpinActive(true);
        if (manualSpinTimer.current) clearTimeout(manualSpinTimer.current);
        manualSpinTimer.current = setTimeout(() => setManualSpinActive(false), 500);
    };

    const logsEnabled =
        activeProfile?.settings?.logs.enabled !== false; // default to true if undefined

    // Auto-refresh background tick: fetch page 1 and diff it against the displayed
    // list; genuinely new entries wait behind the "N new queries" pill instead of
    // replacing the list (which reset scroll position and collapsed open cards).
    const runBackgroundTick = async () => {
        const profileId = activeProfile?.profile_id;
        if (document.hidden || !profileId || !logsEnabled) return;
        if (loading || bgFetchInFlight.current) return;
        if (sortValue !== "created") {
            // Non-temporal sorts have no meaningful prepend point — fall back to the
            // wholesale page-1 replace.
            handleRefresh();
            return;
        }
        bgFetchInFlight.current = true;
        try {
            const response = await api.Client.queryLogsApi.apiV1ProfilesIdLogsGet(
                profileId,
                1,
                100,
                filters.Status,
                filters.Timespan.Value,
                deviceIdValue || undefined,
                committedSearchValue || undefined,
                sortValue
            );
            if (response.status !== 200) return;
            const fetched = response.data || [];
            if (logsRef.current.length === 0) {
                // Nothing on screen to preserve — apply directly, a pill over an
                // empty state helps no one.
                setLogs(fetched);
                setHasMore(fetched.length === 100);
                setPendingLogs([]);
                setPendingOverflow(false);
                return;
            }
            const { newLogs, overlapFound } = computeNewQueryLogs(fetched, logsRef.current);
            setPendingLogs(newLogs);
            setPendingOverflow(!overlapFound && fetched.length === 100);
        } catch {
            // Background ticks fail silently — the next tick retries; foreground
            // fetches own user-visible error reporting.
        } finally {
            bgFetchInFlight.current = false;
        }
    };
    // Latest-closure ref so the interval (bound once per auto-refresh session) always
    // calls a tick that sees current filters/logs without restarting the timer.
    const tickRef = useRef(runBackgroundTick);
    useEffect(() => {
        tickRef.current = runBackgroundTick;
    });

    // Auto-refresh loop at the selected cadence: paused while the tab is hidden (the
    // tick self-skips), with an immediate catch-up tick on return to a visible tab.
    useEffect(() => {
        if (refreshIntervalMs === null || !activeProfile?.profile_id) return;
        const interval = setInterval(() => {
            void tickRef.current();
        }, refreshIntervalMs);
        const onVisibilityChange = () => {
            if (!document.hidden) void tickRef.current();
        };
        document.addEventListener("visibilitychange", onVisibilityChange);
        return () => {
            clearInterval(interval);
            document.removeEventListener("visibilitychange", onVisibilityChange);
        };
    }, [refreshIntervalMs, activeProfile?.profile_id]);

    // Interval menu selection
    const handleRefreshIntervalChange = (key: RefreshIntervalKey) => {
        const wasOn = isAutoRefreshing;
        setRefreshIntervalKey(key);
        if (refreshIntervalMsFor(key) === null) {
            setPendingLogs([]);
            setPendingOverflow(false);
        } else if (!wasOn) {
            // Immediate feedback without disturbing the current list; interval-to-
            // interval changes just retime the loop.
            void tickRef.current();
        }
    };

    // Reveal staged entries: prepend them above the current list. When the tick found
    // a full page with no overlap, prepending would leave a gap — reload instead.
    const handleShowPending = () => {
        if (pendingOverflow) {
            handleRefresh();
            return;
        }
        const previous = logsRef.current;
        const merged = [...pendingLogs, ...previous];
        const previousIdentities = new Set(consolidateLogs(previous).map(group => group.identity));
        setFreshIdentities(
            new Set(
                consolidateLogs(merged)
                    .map(group => group.identity)
                    .filter(identity => !previousIdentities.has(identity))
            )
        );
        setLogs(merged);
        setPendingLogs([]);
    };

    // --- Pull-to-refresh (mobile only) ---
    const { isMobile } = useScreenDetector();
    const [pullDistance, setPullDistance] = useState(0);
    const [isRefreshing, setIsRefreshing] = useState(false);
    const pullStartY = useRef(0);
    const isPulling = useRef(false);
    const PULL_THRESHOLD = 60;

    const handleTouchStart = useCallback((e: React.TouchEvent) => {
        if (!isMobile || isRefreshing) return;
        if (window.scrollY <= 0) {
            pullStartY.current = e.touches[0].clientY;
            isPulling.current = true;
        }
    }, [isMobile, isRefreshing]);

    const handleTouchMove = useCallback((e: React.TouchEvent) => {
        if (!isPulling.current || !isMobile || isRefreshing) return;
        if (window.scrollY > 0) {
            isPulling.current = false;
            setPullDistance(0);
            return;
        }
        const deltaY = e.touches[0].clientY - pullStartY.current;
        if (deltaY > 0) {
            // Apply diminishing resistance: actual distance = delta * 0.4
            setPullDistance(Math.min(deltaY * 0.4, 100));
        } else {
            setPullDistance(0);
        }
    }, [isMobile, isRefreshing]);

    const handleTouchEnd = useCallback(() => {
        if (!isPulling.current || !isMobile) return;
        isPulling.current = false;
        if (pullDistance > PULL_THRESHOLD && !isRefreshing && !loading) {
            setIsRefreshing(true);
            setPullDistance(0);
            // Same one-shot path as the refresh button (also clears staged pill entries)
            handleRefresh();
            // Reset refreshing indicator after a short delay
            setTimeout(() => setIsRefreshing(false), 1200);
        } else {
            setPullDistance(0);
        }
    }, [pullDistance, isRefreshing, loading, isMobile]);

    return (
        <div className="flex flex-col flex-1 w-full h-full min-h-screen md:min-h-0 items-start gap-6 p-6 pt-8 md:pt-8 md:p-8 overflow-visible bg-[var(--shadcn-ui-app-background)]">
            <BetaEndingBanner />
            <LimitedAccessBanner />
            {/* GET /profiles/{id}/logs and DELETE /profiles/{id}/logs are LA-allowed; only the per-row Quick rule action (POST custom_rules) is gated below. */}
            <div className="flex flex-col items-start gap-6 relative flex-1 self-stretch grow w-full">
                {/* Page Description */}
                <section className="w-full">
                    <div className="flex flex-col gap-1">
                        <p className="text-[var(--tailwind-colors-slate-200)] text-sm md:text-base leading-5 md:leading-10">
                            Monitor and analyze DNS queries in real-time. View blocked and processed requests for your active profile.
                        </p>
                    </div>
                </section>

                <Filters
                    searchInputValue={searchInputValue}
                    onSearchInputChange={setSearchInputValue}
                    onSearchCommit={commitSearch}
                    filterValue={filterValue}
                    onFilterChange={setFilterValue}
                    sortValue={sortValue}
                    onSortChange={setSortValue}
                    onRefresh={handleRefresh}
                    timespanValue={timespanValue}
                    onTimespanChange={setTimespanValue}
                    refreshIntervalKey={refreshIntervalKey}
                    onRefreshIntervalChange={handleRefreshIntervalChange}
                    isRefreshing={manualSpinActive || (loading && page === 1)}
                    deviceIdValue={deviceIdValue}
                    onDeviceIdChange={setDeviceIdValue}
                    availableDeviceIds={allAvailableDeviceIds}
                />

                {/* Sibling of Filters and the list section so the parent's gap-6 spaces it
                    evenly between the two. empty:hidden collapses the slot (and its gaps)
                    entirely while nothing is staged; the wrapper stays mounted so the live
                    region exists before the pill text arrives. */}
                <div aria-live="polite" className="w-full flex justify-center empty:hidden -mb-3">
                    {pendingLogs.length > 0 && (
                        <button
                            type="button"
                            onClick={handleShowPending}
                            data-testid="logs-new-queries-pill"
                            className="flex items-center gap-1.5 min-h-11 lg:min-h-9 px-4 py-1.5 rounded-full border border-[var(--tailwind-colors-rdns-600)] bg-[var(--shadcn-ui-app-background)] text-sm text-[var(--tailwind-colors-rdns-600)] cursor-pointer transition-colors hover:bg-[var(--tailwind-colors-rdns-600)]/10 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--tailwind-colors-rdns-600)] animate-in fade-in slide-in-from-top-1 duration-300 ease-out motion-reduce:animate-none"
                        >
                            <ArrowUp className="w-4 h-4" aria-hidden />
                            {pendingOverflow
                                ? "100+ new queries"
                                : `${pendingLogs.length} new ${pendingLogs.length === 1 ? "query" : "queries"}`}
                        </button>
                    )}
                </div>

                <div className="flex flex-col items-start gap-3 md:gap-4 relative flex-1 self-stretch w-full grow min-w-0 overflow-x-hidden">
                    <div className="flex flex-col items-start gap-2 relative flex-1 self-stretch w-full grow rounded-md min-w-0 overflow-x-hidden">
                        {!logsEnabled && (
                            <div className="flex flex-col w-full grow bg-transparent dark:bg-[var(--variable-collection-surface)] rounded-lg overflow-hidden border border-[var(--tailwind-colors-slate-light-300)] dark:border-transparent">
                                <div className="flex flex-col h-auto md:h-[652px] items-start gap-3 md:gap-8 p-4 pt-3 md:pt-4 relative self-stretch w-full">
                                    <div className="flex flex-col items-center justify-start md:justify-center gap-2.5 relative self-stretch w-full md:flex-1 md:grow">
                                        <LogsNotActive profile={activeProfile ?? profiles[0]} />
                                    </div>
                                </div>
                            </div>
                        )}
                        {logsEnabled && logs.length === 0 && !loading && (
                            <div className="flex flex-col w-full grow bg-transparent dark:bg-[var(--variable-collection-surface)] rounded-lg overflow-hidden border border-[var(--tailwind-colors-slate-light-300)] dark:border-transparent" data-testid="logs-empty-state">
                                <div className="flex flex-col h-auto md:h-[652px] items-start gap-3 md:gap-8 p-4 pt-3 md:pt-4 relative self-stretch w-full">
                                    <div className="flex flex-col items-center justify-start md:justify-center gap-2.5 relative self-stretch w-full md:flex-1 md:grow">
                                        <NoLogs isSearchActive={committedSearchValue.trim().length > 0} />
                                    </div>
                                </div>
                            </div>
                        )}

                        {logsEnabled && (
                            <div
                                className="relative flex-1 w-full h-full px-0"
                                data-testid="logs-scroll-container"
                                onTouchStart={isMobile ? handleTouchStart : undefined}
                                onTouchMove={isMobile ? handleTouchMove : undefined}
                                onTouchEnd={isMobile ? handleTouchEnd : undefined}
                            >
                                {/* Pull-to-refresh indicator (mobile only) */}
                                {isMobile && (pullDistance > 0 || isRefreshing) && (
                                    <div className="flex justify-center py-2 text-[var(--tailwind-colors-slate-200)] text-sm select-none"
                                         style={{ opacity: isRefreshing ? 1 : Math.min(pullDistance / PULL_THRESHOLD, 1) }}
                                    >
                                        {isRefreshing
                                            ? "Refreshing..."
                                            : pullDistance > PULL_THRESHOLD
                                                ? "Release to refresh"
                                                : "Pull to refresh"}
                                    </div>
                                )}
                                <div className={`flex flex-col gap-1.5 md:gap-2 px-1.5 md:px-2 py-1.5 md:py-2 min-h-full bg-[var(--shadcn-ui-app-background)] overflow-x-hidden transition-opacity duration-200 ease-in-out ${isListFading ? 'opacity-0' : 'opacity-100'}`}>
                                    {!expandHintDismissed && logs.length > 0 && (
                                        <div
                                            className="md:hidden flex items-start gap-2 rounded-[var(--primitives-radius-radius-md)] border border-[var(--tailwind-colors-slate-light-300)] dark:border-transparent bg-transparent dark:bg-[var(--variable-collection-surface)] px-3 py-2 text-xs text-[var(--tailwind-colors-slate-100)]"
                                            data-testid="logs-expand-hint"
                                        >
                                            <Info className="w-4 h-4 shrink-0 mt-0.5 text-[var(--tailwind-colors-rdns-600)]" aria-hidden />
                                            <span className="flex-1">Tap any entry to see full request details.</span>
                                            <button
                                                type="button"
                                                aria-label="Dismiss hint"
                                                onClick={dismissExpandHint}
                                                data-testid="logs-expand-hint-dismiss"
                                                className="shrink-0 p-0.5 -m-0.5 text-[var(--tailwind-colors-slate-200)] hover:text-[var(--tailwind-colors-slate-50)]"
                                            >
                                                <X className="w-3.5 h-3.5" />
                                            </button>
                                        </div>
                                    )}
                                    {displayGroups.map((group, index) => {
                                        const isLast = index === displayGroups.length - 1;
                                        return (
                                            <QueryLogCard
                                                key={group.key}
                                                log={group.representative}
                                                group={group}
                                                isLast={isLast}
                                                lastLogRef={isLast ? lastLogRef : undefined}
                                                onQuickRule={handleOpenQuickRule}
                                                quickRuleRestricted={isRestricted}
                                                blocklistNames={blocklistNames}
                                                serviceNames={serviceNames}
                                                onExpand={dismissExpandHint}
                                                expanded={expandedKeys.has(group.identity)}
                                                onToggleExpanded={() => toggleCardExpanded(group.identity)}
                                                animateEntry={freshIdentities.has(group.identity)}
                                            />
                                        );
                                    })}
                                    {/* Skeletons: initial load / post-filter-clear (empty list) and pagination.
                                        An in-place refresh (page 1 with rows on screen) shows nothing extra —
                                        the current cards stay until the response replaces them. */}
                                    {loading && (page > 1 || logs.length === 0) && (
                                        <div className="space-y-2">
                                            {Array.from({ length: 8 }).map((_, i) => (
                                                <div key={i} className="flex items-center gap-3 px-3 py-3 bg-transparent dark:bg-[var(--variable-collection-surface)] rounded-[var(--primitives-radius-radius-md)] border border-[var(--tailwind-colors-slate-light-300)] dark:border-transparent">
                                                    <Skeleton className="h-4 w-4 rounded-full" />
                                                    <Skeleton className="h-4 flex-1 max-w-[200px]" />
                                                    <Skeleton className="h-4 w-16" />
                                                    <Skeleton className="h-4 w-10 ml-auto" />
                                                </div>
                                            ))}
                                        </div>
                                    )}
                                    {error && (
                                        <div className="w-full text-center py-4 text-[var(--tailwind-colors-red-500)]">
                                            {error}
                                        </div>
                                    )}
                                </div>
                            </div>
                        )}
                    </div>
                </div>
            </div>

            <QuickRuleSheet
                open={isQuickRuleSheetOpen}
                onOpenChange={handleQuickRuleSheetChange}
                domain={quickRuleDomain}
                defaultAction={quickRuleDefaultAction}
            />
        </div>
    );
};

export default QueryLogs;
