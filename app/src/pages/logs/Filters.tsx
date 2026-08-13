import { type JSX } from "react";
import { Search, ListFilter, ArrowDownAZ, RefreshCw, Monitor, Clock, ChevronDown } from "lucide-react";
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
    deviceIdValue: string | undefined;
    onDeviceIdChange: (value: string | undefined) => void;
    availableDeviceIds: string[];
}

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
        // `group` scopes the hover cue: pointing at either half tints the whole split
        // button's border, so it reads as one control. Border-only — the `!bg` override
        // (needed to sit flush on the page background) suppresses the outline variant's
        // hover background, and a louder cue would compete with the filter accents.
        <div className="flex items-stretch shrink-0 group">
            <Button
                variant="outline"
                size="icon"
                className="w-11 h-11 lg:h-9 lg:w-9 min-h-0 shrink-0 rounded-r-none border-r-0 !bg-[var(--shadcn-ui-app-background)] border-[var(--tailwind-colors-slate-600)] transition-colors group-hover:border-[var(--tailwind-colors-rdns-600)]/50"
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
                        className="h-11 sm:h-11 lg:h-9 min-h-0 shrink-0 rounded-l-none px-1.5 lg:px-1.5 gap-0.5 !bg-[var(--shadcn-ui-app-background)] border-[var(--tailwind-colors-slate-600)] transition-colors group-hover:border-[var(--tailwind-colors-rdns-600)]/50"
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
    deviceIdValue,
    onDeviceIdChange,
    availableDeviceIds,
}: FiltersProps): JSX.Element => (
    <>
        {/* Tablet layout adjustment: two-row layout persists through md (tablets). Desktop (>=lg) collapses to one row. */}
        <div className="flex flex-col lg:flex-row lg:items-start gap-2.5 relative self-stretch w-full min-w-0">
            {/* Row 1: search + refresh (mobile). Desktop: all inline revert -> wrap both rows into one flex row via md:hidden/md:flex patterns */}
            <div className="flex items-start gap-2.5 w-full min-w-0 lg:flex-1 lg:grow lg:hidden">
                <div className="relative flex-1 grow min-w-0">
                    <div className="absolute left-3 top-1/2 -translate-y-1/2 text-[var(--tailwind-colors-slate-400)] pointer-events-none flex items-center">
                        <Search className="h-4 w-4" />
                    </div>
                    <Input
                        type="text"
                        placeholder="Search domain or its part"
                        aria-label="Search domain or its part"
                        className="h-11 lg:h-9 min-h-0 pl-11 pr-3 py-2 !bg-[var(--shadcn-ui-app-background)] border-[var(--tailwind-colors-slate-600)] rounded-[var(--primitives-radius-radius-md)] text-sm text-[var(--tailwind-colors-slate-400)] font-text-sm-leading-5-normal placeholder:text-[var(--tailwind-colors-slate-500)]"
                        value={searchInputValue}
                        onChange={e => onSearchInputChange(e.target.value)}
                        onKeyDown={e => { if (e.key === 'Enter') { onSearchCommit(); e.currentTarget.blur(); } }}
                        onBlur={() => { if (window.innerWidth < 1024) onSearchCommit(); }}
                    />
                </div>
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
                <div className="relative flex-1 grow min-w-0 hidden lg:block">
                    <div className="absolute left-3 top-1/2 -translate-y-1/2 text-[var(--tailwind-colors-slate-400)] pointer-events-none flex items-center">
                        <Search className="h-4 w-4" />
                    </div>
                    <Input
                        type="text"
                        placeholder="Search domain or its part"
                        aria-label="Search domain or its part"
                        className="h-11 lg:h-9 min-h-0 pl-11 pr-3 py-2 !bg-[var(--shadcn-ui-app-background)] border-[var(--tailwind-colors-slate-600)] rounded-[var(--primitives-radius-radius-md)] text-sm text-[var(--tailwind-colors-slate-400)] font-text-sm-leading-5-normal placeholder:text-[var(--tailwind-colors-slate-400)]"
                        value={searchInputValue}
                        onChange={e => onSearchInputChange(e.target.value)}
                        onKeyDown={e => { if (e.key === 'Enter') { onSearchCommit(); e.currentTarget.blur(); } }}
                        onBlur={() => { if (window.innerWidth < 1024) onSearchCommit(); }}
                    />
                </div>

                {/* Query filter */}
                <Select value={filterValue} onValueChange={onFilterChange}>
                    <SelectTrigger className="w-28 md:w-32 px-1.5 md:px-2 py-1.5 !bg-[var(--shadcn-ui-app-background)] border-[var(--tailwind-colors-slate-600)]">
                        <div className="flex items-center gap-0.5 md:gap-1">
                            <ListFilter className="w-4 h-4" />
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
                    <SelectTrigger className="px-1.5 md:px-2 py-1.5 !bg-[var(--shadcn-ui-app-background)] border-[var(--tailwind-colors-slate-600)] w-36 md:w-40">
                        <div className="flex items-center gap-0.5 md:gap-1">
                            <Monitor className="w-4 h-4" />
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
                    <SelectTrigger className="px-1.5 md:px-2 py-1.5 !bg-[var(--shadcn-ui-app-background)] border-[var(--tailwind-colors-slate-600)]">
                        <div className="flex items-center gap-0.5 md:gap-1">
                            <ArrowDownAZ className="w-4 h-4" />
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
                    <SelectTrigger className="px-1.5 md:px-2 py-1.5 !bg-[var(--shadcn-ui-app-background)] border-[var(--tailwind-colors-slate-600)] w-28 md:w-32">
                        <div className="flex items-center gap-0.5 md:gap-1">
                            <Clock className="w-4 h-4" />
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

export default Filters;