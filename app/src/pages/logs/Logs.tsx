import { useEffect, useRef, useState, useCallback, type JSX } from "react";
import type { AxiosError } from "axios";

interface NetworkError extends AxiosError { code?: string; }

import { toast } from "sonner";

import type { ModelAccount, ModelProfile, ModelQueryLog } from "@/api/client";
import Filters from "./Filters";
import NoLogs from "./NoLogs";
import LogsNotActive from "./LogsNotActive";
import QueryLogCard from "./QueryLogCard";
import QuickRuleSheet, { type QuickRuleAction } from "./QuickRuleSheet";
import api from "@/api/api";
import { useAppStore } from "@/store/general";
import { Skeleton } from "@/components/ui/skeleton";
import { useScreenDetector } from "@/hooks/useScreenDetector";

const QUERY_LIMIT = 25;

interface QueryLogsProps {
    account: ModelAccount;
    profiles: ModelProfile[];
}

const QueryLogs = ({ profiles }: QueryLogsProps): JSX.Element => {
    const [logs, setLogs] = useState<ModelQueryLog[]>([]);
    const [page, setPage] = useState(1);
    const [hasMore, setHasMore] = useState(true);
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [isAutoRefreshing, setIsAutoRefreshing] = useState(false);
    const [refreshTrigger, setRefreshTrigger] = useState(0); // Add trigger for forced refresh
    const [fadeClass, setFadeClass] = useState('opacity-100 transition-opacity duration-300 ease-in-out'); // Track fade animation state
    const [isQuickRuleSheetOpen, setIsQuickRuleSheetOpen] = useState(false);
    const [quickRuleDomain, setQuickRuleDomain] = useState<string | undefined>(undefined);
    const [quickRuleDefaultAction, setQuickRuleDefaultAction] = useState<QuickRuleAction>("denylist");

    // Search input (uncommitted while typing) and committed value that triggers requests
    const [searchInputValue, setSearchInputValue] = useState("");
    const [committedSearchValue, setCommittedSearchValue] = useState("");
    const [filterValue, setFilterValue] = useState("all");
    const [sortValue, setSortValue] = useState("created");
    const [timespanValue, setTimespanValue] = useState<string | undefined>(undefined);
    const [deviceIdValue, setDeviceIdValue] = useState<string | undefined>(undefined);

    // Maintain a separate list of all available device IDs (not filtered by current selection)
    const [allAvailableDeviceIds, setAllAvailableDeviceIds] = useState<string[]>([]);

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

    const handleOpenQuickRule = useCallback((domain?: string, defaultAction: QuickRuleAction = "denylist") => {
        if (!domain) return;
        setQuickRuleDomain(domain);
        setQuickRuleDefaultAction(defaultAction);
        setIsQuickRuleSheetOpen(true);
    }, []);

    const handleQuickRuleSheetChange = useCallback((nextOpen: boolean) => {
        setIsQuickRuleSheetOpen(nextOpen);
        if (!nextOpen) {
            setQuickRuleDomain(undefined);
        }
    }, []);

    // Reset logs, device IDs and page when committed filters change
    useEffect(() => {
        setLogs([]);
        setPage(1);
        setHasMore(true);
        setAllAvailableDeviceIds([]);
    }, [committedSearchValue, filterValue, sortValue, timespanValue, deviceIdValue]);

    useEffect(() => {
        const currentId = activeProfile?.profile_id;
        if (previousProfileIdRef.current && previousProfileIdRef.current !== currentId) {
            setIsQuickRuleSheetOpen(false);
            setQuickRuleDomain(undefined);
        }
        previousProfileIdRef.current = currentId;
    }, [activeProfile?.profile_id]);

    const commitSearch = useCallback(() => {
        setCommittedSearchValue(prev => prev === searchInputValue ? prev : searchInputValue);
    }, [searchInputValue]);

    // Fetch logs and then fetch logos for the batch
    useEffect(() => {
        let cancelled = false;
        let fadeInTimeout: ReturnType<typeof setTimeout> | undefined;
        const fetchLogs = async () => {
            // Don't fetch if no active profile
            if (!activeProfile?.profile_id) {
                setLoading(false);
                return;
            }

            setLoading(true);
            setError(null);

            // Start fade-out animation only for page 1 (refresh)
            if (page === 1) {
                setFadeClass('opacity-0 transition-opacity duration-200 ease-out');
            }

            try {
                // Status is already handled in filters.Status
                // Use expanded limit on first page to gather more device IDs; subsequent pages respect configured limit
                const effectiveLimit = (page === 1 && !isAutoRefreshing) ? 100 : filters.Limit;
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


                    // Trigger fade-in animation with a delay to ensure content is rendered
                    if (page === 1) {
                        fadeInTimeout = setTimeout(() => {
                            setFadeClass('opacity-100 transition-opacity duration-200 ease-in');
                        }, 100);
                    }
                } else {
                    setHasMore(false);
                    if (page === 1) {
                        setFadeClass('opacity-100 transition-opacity duration-400 ease-in-out');
                    }
                }
            } catch (err: unknown) {
                // Handle different HTTP error codes with specific messages
                let errorMessage = "Failed to load logs";
                const httpErr = err as AxiosError & { code?: string };
                const status = httpErr.response?.status;
                if (status === 429) {
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
                if (page === 1) {
                    setFadeClass('opacity-100 transition-opacity duration-300 ease-in-out');
                }
            } finally {
                if (!cancelled) setLoading(false);
            }
        };
        fetchLogs();
        return () => {
            cancelled = true;
            if (fadeInTimeout) clearTimeout(fadeInTimeout);
        };
    // eslint-disable-next-line react-hooks/exhaustive-deps -- committedSearchValue, isAutoRefreshing, and sortValue are consumed via the `filters` object and `refreshTrigger`; adding them directly would cause redundant re-fetches since the filters object already captures their derived values
    }, [page, filters.Limit, filters.Status, filters.Timespan.Value, filters.Search, filters.Sort, activeProfile, refreshTrigger, deviceIdValue]);

    // Auto-refresh effect
    useEffect(() => {
        let interval: NodeJS.Timeout | null = null;

        if (isAutoRefreshing && activeProfile?.profile_id) {
            interval = setInterval(() => {
                // Force refresh by incrementing trigger and resetting to first page
                setPage(1);
                setLogs([]);
                setHasMore(true);
                setRefreshTrigger(prev => prev + 1);
            }, 10000); // 10 seconds
        }

        return () => {
            if (interval) {
                clearInterval(interval);
            }
        };
    }, [isAutoRefreshing, activeProfile?.profile_id]);

    // Handle auto-refresh toggle
    const handleToggleAutoRefresh = () => {
        setIsAutoRefreshing(prev => !prev);
        if (!isAutoRefreshing) {
            // When starting auto-refresh, immediately refresh once
            setLogs([]);
            setPage(1);
            setHasMore(true);
            setRefreshTrigger(prev => prev + 1);
        }
    };

    // Handle manual refresh
    const handleRefresh = () => {
        setLogs([]);
        setPage(1);
        setHasMore(true);
        setRefreshTrigger(prev => prev + 1);
    };

    // --- Pull-to-refresh (mobile only) ---
    const { isMobile } = useScreenDetector();
    const [pullDistance, setPullDistance] = useState(0);
    const [isRefreshing, setIsRefreshing] = useState(false);
    const pullStartY = useRef(0);
    const isPulling = useRef(false);
    const scrollContainerRef = useRef<HTMLDivElement>(null);

    const PULL_THRESHOLD = 60;

    const handleTouchStart = useCallback((e: React.TouchEvent) => {
        if (!isMobile || isRefreshing) return;
        const container = scrollContainerRef.current;
        if (container && container.scrollTop <= 0) {
            pullStartY.current = e.touches[0].clientY;
            isPulling.current = true;
        }
    }, [isMobile, isRefreshing]);

    const handleTouchMove = useCallback((e: React.TouchEvent) => {
        if (!isPulling.current || !isMobile || isRefreshing) return;
        const container = scrollContainerRef.current;
        if (!container || container.scrollTop > 0) {
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
            // Trigger the existing refresh mechanism
            setLogs([]);
            setPage(1);
            setHasMore(true);
            setRefreshTrigger(prev => prev + 1);
            // Reset refreshing indicator after a short delay
            setTimeout(() => setIsRefreshing(false), 1200);
        } else {
            setPullDistance(0);
        }
    }, [pullDistance, isRefreshing, loading, isMobile]);

    const logsEnabled =
        activeProfile?.settings?.logs.enabled !== false; // default to true if undefined

    return (
        <div className="flex flex-col flex-1 w-full h-full min-h-screen md:min-h-0 items-start gap-6 p-6 pt-8 md:pt-8 md:p-8 overflow-visible bg-[var(--shadcn-ui-app-background)]">
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
                    isAutoRefreshing={isAutoRefreshing}
                    onToggleAutoRefresh={handleToggleAutoRefresh}
                    deviceIdValue={deviceIdValue}
                    onDeviceIdChange={setDeviceIdValue}
                    availableDeviceIds={allAvailableDeviceIds}
                />

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
                                ref={scrollContainerRef}
                                className="relative flex-1 w-full h-full overflow-y-auto overscroll-contain px-0"
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
                                <div className={`flex flex-col gap-1.5 md:gap-2 py-1.5 md:py-2 min-h-full bg-[var(--shadcn-ui-app-background)] overflow-x-hidden ${fadeClass || 'opacity-100'}`}>
                                    {logs.map((log, index) => {
                                        const isLast = index === logs.length - 1;
                                        return (
                                            <QueryLogCard
                                                key={`${log.profile_id}-${log.timestamp}-${index}`}
                                                log={log}
                                                isLast={isLast}
                                                lastLogRef={isLast ? lastLogRef : undefined}
                                                onQuickRule={handleOpenQuickRule}
                                            />
                                        );
                                    })}
                                    {loading && (
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
