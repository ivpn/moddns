import { type JSX } from "react";
import { Search, ListFilter, ArrowDownAZ, RefreshCw, Monitor, Clock } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
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
    isAutoRefreshing?: boolean;
    onToggleAutoRefresh?: () => void;
    deviceIdValue: string | undefined;
    onDeviceIdChange: (value: string | undefined) => void;
    availableDeviceIds: string[];
}

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
    isAutoRefreshing = false,
    onToggleAutoRefresh,
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
                <Button
                    variant="outline"
                    size="icon"
                    className={`w-11 h-11 lg:h-9 min-h-0 !bg-[var(--shadcn-ui-app-background)] border-[var(--tailwind-colors-slate-600)] ${isAutoRefreshing ? 'bg-[var(--tailwind-colors-rdns-600)]' : ''}`}
                    onClick={onToggleAutoRefresh || onRefresh}
                    title={isAutoRefreshing ? "Stop auto-refresh" : "Start auto-refresh"}
                >
                    <RefreshCw className={`w-4 h-4 ${isAutoRefreshing ? 'text-[var(--tailwind-colors-rdns-600)] animate-spin' : 'text-[var(--tailwind-colors-rdns-600)]'}`} />
                </Button>
            </div>

            {/* Row 2 (mobile: single horizontal scroll line) / Full single row (desktop) */}
            <div className="flex lg:flex-nowrap items-start w-full min-w-0 lg:flex-1 lg:grow overflow-x-auto no-scrollbar flex-nowrap gap-1.5 lg:gap-2.5">
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

                {/* Desktop refresh button (hidden on mobile second row) */}
                <div className="hidden lg:block">
                    <Button
                        variant="outline"
                        size="icon"
                        className={`w-11 h-11 lg:h-9 min-h-0 !bg-[var(--shadcn-ui-app-background)] border-[var(--tailwind-colors-slate-600)] ${isAutoRefreshing ? 'bg-[var(--tailwind-colors-rdns-600)]' : ''}`}
                        onClick={onToggleAutoRefresh || onRefresh}
                        title={isAutoRefreshing ? "Stop auto-refresh" : "Start auto-refresh"}
                    >
                        <RefreshCw className={`w-4 h-4 ${isAutoRefreshing ? 'text-[var(--tailwind-colors-rdns-600)] animate-spin' : 'text-[var(--tailwind-colors-rdns-600)]'}`} />
                    </Button>
                </div>
            </div>
        </div>
    </>
);

export default Filters;