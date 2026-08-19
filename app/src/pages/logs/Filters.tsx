import { useEffect, useState, type JSX } from "react";
import { Search, ListFilter, ArrowDownAZ, RefreshCw, Monitor, Clock, ChevronDown, X } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuRadioGroup,
    DropdownMenuRadioItem,
    DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
    QUERY_LOGS_REFRESH_INTERVALS,
    type RefreshIntervalKey,
} from "@/lib/consts";
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select";

interface FiltersProps {
    searchInputValue: string; // current text in the search input
    onSearchInputChange: (value: string) => void; // updates uncontrolled typing state
    onSearchCommit: () => void; // commit the current input value to trigger request
    onSearchClear: () => void; // empty the input AND the committed value
    /** Last committed search — drives the clear-filters chip, never the uncommitted typing. */
    committedSearchValue: string;
    onClearFilters: () => void;
    filterValue: string;
    onFilterChange: (value: string) => void;
    sortValue: string;
    onSortChange: (value: string) => void;
    onRefresh: () => void;
    timespanValue: string | undefined;
    onTimespanChange: (value: string | undefined) => void;
    /** Selected auto-refresh cadence ("off" = disabled). */
    refreshIntervalKey: RefreshIntervalKey;
    onRefreshIntervalChange: (key: RefreshIntervalKey) => void;
    /** True while a manual (one-shot) refresh is in flight — spins the refresh icon. */
    isRefreshing?: boolean;
    /** Last successful contact with the logs endpoint; null before the first load. */
    lastUpdatedAt?: number | null;
    deviceIdValue: string | undefined;
    onDeviceIdChange: (value: string | undefined) => void;
    availableDeviceIds: string[];
}

// Compact relative age for a bar that refreshes in seconds — date-fns'
// formatDistanceToNow bottoms out at "less than a minute", too vague here.
const formatAge = (ms: number): string => {
    const seconds = Math.max(0, Math.floor(ms / 1000));
    if (seconds < 10) return "just now";
    if (seconds < 60) return `${seconds}s ago`;
    const minutes = Math.floor(seconds / 60);
    if (minutes < 60) return `${minutes}m ago`;
    const hours = Math.floor(minutes / 60);
    return `${hours}h ago`;
};

// Freshness label beside the refresh controls. Lives in the sticky bar because that is
// the page's only persistent chrome — freshness matters most when the reader is deep in
// the list with auto-refresh off, exactly when the page description has scrolled away.
// A growing age is also the only user-visible signal when silent background ticks stop
// landing. Isolated so only this label re-renders on the 10s age tick. Deliberately NOT
// an aria-live region — announcing every tick would chatter at screen-reader users.
const FreshnessLabel = ({ lastUpdatedAt }: { lastUpdatedAt: number }): JSX.Element => {
    const [, forceTick] = useState(0);
    useEffect(() => {
        const interval = setInterval(() => forceTick(t => t + 1), 10000);
        return () => clearInterval(interval);
    }, []);
    return (
        <span
            className="whitespace-nowrap shrink-0 text-xs leading-4 text-[var(--tailwind-colors-slate-200)]"
            data-testid="logs-freshness"
        >
            Updated {formatAge(Date.now() - lastUpdatedAt)}
        </span>
    );
};

// Shared search input (rendered in the mobile row and the desktop row). Commits on
// Enter or after the owner's debounce — there is deliberately no commit-on-blur (the
// old behavior committed on blur only below 1024px, measured at event time, which made
// desktop and mobile behave differently for no discernible reason).
const LogsSearchInput = ({
    value,
    onChange,
    onCommit,
    onClear,
}: {
    value: string;
    onChange: (value: string) => void;
    onCommit: () => void;
    onClear: () => void;
}): JSX.Element => (
    <div className="relative flex-1 grow min-w-0">
        <div className="absolute left-3 top-1/2 -translate-y-1/2 text-[var(--tailwind-colors-slate-400)] pointer-events-none flex items-center">
            <Search className="h-4 w-4" />
        </div>
        <Input
            type="text"
            placeholder="Search domain or its part"
            aria-label="Search domain or its part"
            className="h-11 lg:h-9 min-h-0 pl-11 pr-11 py-2 !bg-[var(--shadcn-ui-app-background)] border-[var(--tailwind-colors-slate-600)] rounded-[var(--primitives-radius-radius-md)] text-sm text-[var(--tailwind-colors-slate-400)] font-text-sm-leading-5-normal placeholder:text-[var(--tailwind-colors-slate-400)]"
            value={value}
            onChange={e => onChange(e.target.value)}
            onKeyDown={e => { if (e.key === 'Enter') { onCommit(); e.currentTarget.blur(); } }}
        />
        {value.length > 0 && (
            <button
                type="button"
                aria-label="Clear search"
                data-testid="logs-search-clear"
                onClick={onClear}
                className="absolute right-0 top-1/2 -translate-y-1/2 flex items-center justify-center w-11 h-11 lg:w-9 lg:h-9 text-[var(--tailwind-colors-slate-400)] hover:text-[var(--tailwind-colors-slate-200)] cursor-pointer"
            >
                <X className="w-4 h-4" />
            </button>
        )}
    </div>
);

// Grafana-style split refresh control, rendered in the mobile search row and at the end
// of the desktop filter row. Left half: one-shot refresh (never touches the loop). Right
// half: auto-refresh interval menu; while an interval is active its compact label shows
// on the button and the refresh icon spins continuously (the "live" cue).
const RefreshControls = ({
    onRefresh,
    isRefreshing,
    refreshIntervalKey,
    onRefreshIntervalChange,
}: {
    onRefresh: () => void;
    isRefreshing: boolean;
    refreshIntervalKey: RefreshIntervalKey;
    onRefreshIntervalChange: (key: RefreshIntervalKey) => void;
}): JSX.Element => {
    const activeOption = QUERY_LOGS_REFRESH_INTERVALS.find(option => option.key === refreshIntervalKey);
    const isAutoRefreshing = (activeOption?.ms ?? null) !== null;
    return (
        <div className="flex items-stretch shrink-0">
            <Button
                variant="outline"
                size="icon"
                className="w-11 h-11 lg:h-9 lg:w-9 min-h-0 shrink-0 rounded-r-none border-r-0 !bg-[var(--shadcn-ui-app-background)] border-[var(--tailwind-colors-slate-600)]"
                onClick={onRefresh}
                aria-label="Refresh query logs"
                title="Refresh"
                data-testid="logs-refresh-button"
            >
                {/* Fast spin only while a fetch is actually in flight; a calm 3s rotation
                    is the persistent live-mode cue. Reduced motion falls back to the
                    interval label as the only cue. */}
                <RefreshCw
                    className={`w-4 h-4 text-[var(--tailwind-colors-rdns-600)] motion-reduce:animate-none ${
                        isRefreshing ? "animate-spin" : isAutoRefreshing ? "animate-[spin_3s_linear_infinite]" : ""
                    }`}
                />
            </Button>
            <DropdownMenu>
                <DropdownMenuTrigger asChild>
                    <Button
                        variant="outline"
                        className="h-11 sm:h-11 lg:h-9 min-h-0 shrink-0 rounded-l-none px-1.5 lg:px-1.5 gap-0.5 !bg-[var(--shadcn-ui-app-background)] border-[var(--tailwind-colors-slate-600)]"
                        aria-label="Auto-refresh interval"
                        title="Auto-refresh interval"
                        data-testid="logs-refresh-interval-trigger"
                    >
                        {isAutoRefreshing && (
                            <span className="text-xs text-[var(--tailwind-colors-rdns-600)]" data-testid="logs-refresh-interval-label">
                                {activeOption?.buttonLabel}
                            </span>
                        )}
                        <ChevronDown className={`w-3.5 h-3.5 ${isAutoRefreshing ? "text-[var(--tailwind-colors-rdns-600)]" : "text-[var(--tailwind-colors-slate-400)]"}`} />
                    </Button>
                </DropdownMenuTrigger>
                {/* align="start": open down-right so the menu doesn't drop over the
                    quick-rule column at the right edge of the cards below. Radix
                    collision handling still flips it where the viewport is too narrow. */}
                <DropdownMenuContent align="start">
                    <DropdownMenuRadioGroup
                        value={refreshIntervalKey}
                        onValueChange={value => onRefreshIntervalChange(value as RefreshIntervalKey)}
                    >
                        {QUERY_LOGS_REFRESH_INTERVALS.map(option => (
                            <DropdownMenuRadioItem
                                key={option.key}
                                value={option.key}
                                data-testid={`logs-refresh-interval-${option.key}`}
                            >
                                {option.label}
                            </DropdownMenuRadioItem>
                        ))}
                    </DropdownMenuRadioGroup>
                </DropdownMenuContent>
            </DropdownMenu>
        </div>
    );
};

const Filters = ({
    searchInputValue,
    onSearchInputChange,
    onSearchCommit,
    onSearchClear,
    committedSearchValue,
    onClearFilters,
    filterValue,
    onFilterChange,
    sortValue,
    onSortChange,
    onRefresh,
    timespanValue,
    onTimespanChange,
    refreshIntervalKey,
    onRefreshIntervalChange,
    isRefreshing = false,
    lastUpdatedAt = null,
    deviceIdValue,
    onDeviceIdChange,
    availableDeviceIds,
}: FiltersProps): JSX.Element => {
    // Below `md` the Select values are hidden, so the accent border/icon is the ONLY
    // signal that a filter narrows the list.
    const statusActive = filterValue !== "all";
    const deviceActive = deviceIdValue !== undefined;
    const sortActive = sortValue !== "created";
    const timespanActive = timespanValue !== undefined && timespanValue !== "all";
    const searchCommitted = committedSearchValue.trim().length > 0;
    const anyActive = statusActive || deviceActive || sortActive || timespanActive || searchCommitted;
    // The base SelectTrigger's focus-visible ring + border-ring fires on Radix's
    // programmatic refocus after picking an option, painting a gray outline over the
    // accent border — suppress it and keep the border tracking the active state.
    // Active accent matches the query-log cards' hover/open outline (full rdns-600 on
    // light, /40 on dark) so the two surfaces share one visual language.
    const triggerBorder = (active: boolean) =>
        active
            ? "border-[var(--tailwind-colors-rdns-600)] dark:border-[var(--tailwind-colors-rdns-600)]/40 focus-visible:border-[var(--tailwind-colors-rdns-600)] dark:focus-visible:border-[var(--tailwind-colors-rdns-600)]/40 focus-visible:ring-0"
            : "border-[var(--tailwind-colors-slate-600)] focus-visible:border-[var(--tailwind-colors-slate-600)] focus-visible:ring-0";
    const iconTint = (active: boolean) => (active ? "text-[var(--tailwind-colors-rdns-600)]" : "");
    return (
    <>
        {/* Freshness line above the controls row, right-aligned over the refresh button.
            Deliberately NOT inline with the row: the split button widens when an interval
            is active and the search input flexes, so an inline label would jitter. The
            height is reserved from the start (desktop) so the label's appearance after
            the first load shifts nothing. */}
        <div className="hidden lg:flex justify-end h-4 mb-1 self-stretch w-full">
            {lastUpdatedAt !== null && <FreshnessLabel lastUpdatedAt={lastUpdatedAt} />}
        </div>
        {/* Tablet layout adjustment: two-row layout persists through md (tablets). Desktop (>=lg) collapses to one row. */}
        <div className="flex flex-col lg:flex-row lg:items-start gap-2.5 relative self-stretch w-full min-w-0">
            {/* Row 1: search + refresh (mobile). Desktop: all inline revert -> wrap both rows into one flex row via md:hidden/md:flex patterns */}
            <div className="flex items-start gap-2.5 w-full min-w-0 lg:flex-1 lg:grow lg:hidden">
                <LogsSearchInput
                    value={searchInputValue}
                    onChange={onSearchInputChange}
                    onCommit={onSearchCommit}
                    onClear={onSearchClear}
                />
                <RefreshControls
                    onRefresh={onRefresh}
                    isRefreshing={isRefreshing}
                    refreshIntervalKey={refreshIntervalKey}
                    onRefreshIntervalChange={onRefreshIntervalChange}
                />
            </div>

            {/* Row 2 (mobile: single horizontal scroll line) / Full single row (desktop).
                p-1/-m-1 keeps the 3px focus rings of the search input and filter
                controls inside the overflow-x-auto clip box without shifting layout. */}
            <div className="flex lg:flex-nowrap items-start w-full min-w-0 lg:flex-1 lg:grow overflow-x-auto no-scrollbar flex-nowrap gap-1.5 lg:gap-2.5 p-1 -m-1">
                {/* Desktop search (hidden on mobile) */}
                <div className="hidden lg:flex flex-1 grow min-w-0">
                    <LogsSearchInput
                        value={searchInputValue}
                        onChange={onSearchInputChange}
                        onCommit={onSearchCommit}
                        onClear={onSearchClear}
                    />
                </div>

                {/* Query filter */}
                <Select value={filterValue} onValueChange={onFilterChange}>
                    <SelectTrigger aria-label="Filter by status" className={`w-28 md:w-32 px-1.5 md:px-2 py-1.5 !bg-[var(--shadcn-ui-app-background)] ${triggerBorder(statusActive)}`}>
                        <div className="flex items-center gap-0.5 md:gap-1">
                            <ListFilter className={`w-4 h-4 ${iconTint(statusActive)}`} />
                            <span className="hidden md:inline"><SelectValue placeholder="All queries" /></span>
                        </div>
                    </SelectTrigger>
                    <SelectContent>
                        <SelectItem value="all">All queries</SelectItem>
                        <SelectItem value="blocked">Blocked</SelectItem>
                        <SelectItem value="processed">Processed</SelectItem>
                    </SelectContent>
                </Select>

                {/* Device filter */}
                <Select
                    value={deviceIdValue ?? "all"}
                    onValueChange={val => onDeviceIdChange(val === "all" ? undefined : val)}
                >
                    <SelectTrigger aria-label="Filter by device" className={`px-1.5 md:px-2 py-1.5 !bg-[var(--shadcn-ui-app-background)] ${triggerBorder(deviceActive)} w-36 md:w-40`}>
                        <div className="flex items-center gap-0.5 md:gap-1">
                            <Monitor className={`w-4 h-4 ${iconTint(deviceActive)}`} />
                            <span className="hidden md:inline"><SelectValue placeholder="All devices" /></span>
                        </div>
                    </SelectTrigger>
                    <SelectContent>
                        <SelectItem value="all">All devices</SelectItem>
                        {availableDeviceIds.map((deviceId) => (
                            <SelectItem key={deviceId} value={deviceId}>
                                {deviceId}
                            </SelectItem>
                        ))}
                    </SelectContent>
                </Select>

                {/* Sort filter */}
                <Select value={sortValue} onValueChange={onSortChange}>
                    <SelectTrigger aria-label="Sort logs" className={`px-1.5 md:px-2 py-1.5 !bg-[var(--shadcn-ui-app-background)] ${triggerBorder(sortActive)}`}>
                        <div className="flex items-center gap-0.5 md:gap-1">
                            <ArrowDownAZ className={`w-4 h-4 ${iconTint(sortActive)}`} />
                            <span className="hidden md:inline"><SelectValue placeholder="Created" /></span>
                        </div>
                    </SelectTrigger>
                    <SelectContent>
                        <SelectItem value="created">Created</SelectItem>
                        <SelectItem value="domain">Domain</SelectItem>
                        <SelectItem value="client_ip">Client IP</SelectItem>
                    </SelectContent>
                </Select>

                {/* Timespan filter */}
                <Select
                    value={timespanValue ?? "all"}
                    onValueChange={val => onTimespanChange(val === "all" ? undefined : val)}
                >
                    <SelectTrigger aria-label="Filter by timespan" className={`px-1.5 md:px-2 py-1.5 !bg-[var(--shadcn-ui-app-background)] ${triggerBorder(timespanActive)} w-28 md:w-32`}>
                        <div className="flex items-center gap-0.5 md:gap-1">
                            <Clock className={`w-4 h-4 ${iconTint(timespanActive)}`} />
                            <span className="hidden md:inline"><SelectValue placeholder="Timespan" /></span>
                        </div>
                    </SelectTrigger>
                    <SelectContent>
                        <SelectItem value="all">All time</SelectItem>
                        <SelectItem value="LAST_1_HOUR">Last 1 hour</SelectItem>
                        <SelectItem value="LAST_12_HOURS">Last 12 hours</SelectItem>
                        <SelectItem value="LAST_1_DAY">Last 1 day</SelectItem>
                        <SelectItem value="LAST_7_DAYS">Last 7 days</SelectItem>
                        <SelectItem value="LAST_MONTH">Last 30 days</SelectItem>
                    </SelectContent>
                </Select>

                {/* Clear-all chip: appears once anything narrows the list (a non-default
                    select OR a committed search — never uncommitted typing). */}
                {anyActive && (
                    <Button
                        variant="ghost"
                        onClick={onClearFilters}
                        aria-label="Clear all filters"
                        title="Clear all filters"
                        data-testid="logs-clear-filters"
                        className="h-11 lg:h-9 min-h-0 shrink-0 px-2 gap-1 text-sm text-[var(--tailwind-colors-slate-400)] hover:text-[var(--tailwind-colors-slate-200)]"
                    >
                        <X className="w-4 h-4" />
                        <span className="hidden md:inline">Clear</span>
                    </Button>
                )}

                {/* Desktop refresh controls (hidden on mobile second row) */}
                <div className="hidden lg:flex items-start">
                    <RefreshControls
                        onRefresh={onRefresh}
                        isRefreshing={isRefreshing}
                        refreshIntervalKey={refreshIntervalKey}
                        onRefreshIntervalChange={onRefreshIntervalChange}
                    />
                </div>
            </div>
        </div>
    </>
    );
};

export default Filters;