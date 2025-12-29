import { type JSX, useState, useEffect } from "react";
import AlertCard from "@/components/general/AlertCard";
import BlocklistCard from "./BlocklistCard";
import EmptyState from "@/pages/blocklists/NoBlocklistsFound";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { ScrollArea } from "@/components/ui/scroll-area";
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select";
import {
    ListFilterIcon,
    SearchIcon,
    ToggleLeftIcon,
    Clock,
} from "lucide-react";
import type { ApiBlocklistsUpdates } from "@/api/client/api";
import api from "@/api/api";
import { useAppStore } from "@/store/general";
import { formatDistanceToNow, parseISO } from "date-fns";
import { toast } from "sonner";
import axios from "axios";

const INDIVIDUAL_LISTS = [
    { label: "Hagezi", tag: "hagezi" },
    { label: "Adguard", tag: "adguard" },
    { label: "OISD", tag: "oisd" },
    { label: "Steven Black", tag: "steven_black" },
];

const PREDEFINED_LISTS = [
    { label: "Basic", tag: "basic" },
    { label: "Comprehensive", tag: "comprehensive" },
    { label: "Restrictive", tag: "restrictive" },
];

const STATUS_FILTERS = [
    { label: "Enabled", value: "enabled" },
    { label: "Disabled", value: "disabled" },
];

interface MainContentSectionProps {
}

export default function MainContentSection({ }: MainContentSectionProps): JSX.Element {
    const [showAlert, setShowAlert] = useState(true);
    const [blocklists, setBlocklists] = useState<any[]>([]);
    const [loading, setLoading] = useState(true);
    const [updating, setUpdating] = useState<string | null>(null);
    const [searchValue, setSearchValue] = useState("");
    const [filterValue, setFilterValue] = useState("all");
    const [sortValue, setSortValue] = useState("updated");

    // Get activeProfile from the store
    const activeProfile = useAppStore((state) => state.activeProfile);
    const setActiveProfile = useAppStore((state) => state.setActiveProfile);

    // Get enabled blocklists from activeProfile
    const enabledBlocklists: string[] =
        activeProfile?.settings?.privacy?.blocklists ?? [];

    useEffect(() => {
        const fetchBlocklists = async () => {
            setLoading(true);
            try {
                const resp = await api.Client.blocklistsApi.apiV1BlocklistsGet();
                setBlocklists(resp.data || []);
            } catch (error: any) {
                if (axios.isAxiosError(error)) {
                    // Handle rate limiting (429)
                    if (error.response?.status === 429) {
                        toast.error("Too many requests", {
                            description: "Blocklists are temporarily unavailable. Please try again in a moment.",
                        });
                        setBlocklists([]);
                        return;
                    }
                }
                // For other errors, just set empty array (silent failure)
                setBlocklists([]);
            } finally {
                setLoading(false);
            }
        };
        fetchBlocklists();
    }, []);

    // Handler to enable/disable a blocklist for the user
    const handleBlocklistSwitch = async (blocklistId: string, checked: boolean) => {
        if (!activeProfile?.profile_id) return;
        setUpdating(blocklistId);
        try {
            let resp;
            if (checked) {
                resp = await api.Client.profilesApi.apiV1ProfilesIdBlocklistsPost(
                    activeProfile.profile_id,
                    { blocklist_ids: [blocklistId] } as ApiBlocklistsUpdates
                );
            } else {
                resp = await api.Client.profilesApi.apiV1ProfilesIdBlocklistsDelete(
                    activeProfile.profile_id,
                    { blocklist_ids: [blocklistId] } as ApiBlocklistsUpdates
                );
            }
            if (resp && resp.status === 200) {
                const updatedProfile = await api.Client.profilesApi.apiV1ProfilesIdGet(activeProfile.profile_id);
                setActiveProfile(updatedProfile.data);
                toast.success(
                    checked ? "Blocklist enabled" : "Blocklist disabled",
                    {
                        description: checked
                            ? "Blocklist has been enabled successfully."
                            : "Blocklist has been disabled successfully.",
                    }
                );
            }
        } catch (e) {
            toast.error("Error", {
                description: "Failed to update blocklist. Please try again.",
            });
        } finally {
            setUpdating(null);
        }
    };

    // Filter blocklists by search and filter value (basic, comprehensive, restrictive, all)
    let filteredBlocklists = blocklists.filter((blocklist) => {
        const matchesSearch =
            !searchValue.trim() ||
            blocklist.name?.toLowerCase().includes(searchValue.toLowerCase()) ||
            blocklist.description?.toLowerCase().includes(searchValue.toLowerCase());

        let matchesFilter = true;
        if (
            filterValue !== "all" &&
            filterValue !== "enabled" &&
            filterValue !== "disabled"
        ) {
            // For predefined lists, use accumulative logic
            if (filterValue === "basic") {
                matchesFilter = Array.isArray(blocklist.tags) && blocklist.tags.includes("basic");
            } else if (filterValue === "comprehensive") {
                matchesFilter = Array.isArray(blocklist.tags) &&
                    (blocklist.tags.includes("basic") || blocklist.tags.includes("comprehensive"));
            } else if (filterValue === "restrictive") {
                matchesFilter = Array.isArray(blocklist.tags) &&
                    (blocklist.tags.includes("basic") || blocklist.tags.includes("comprehensive") || blocklist.tags.includes("restrictive"));
            } else {
                // For individual lists (hagezi, adguard, oisd), use exact match
                matchesFilter = Array.isArray(blocklist.tags) && blocklist.tags.includes(filterValue);
            }
        } else if (filterValue === "enabled") {
            matchesFilter = enabledBlocklists.includes(blocklist.blocklist_id);
        } else if (filterValue === "disabled") {
            matchesFilter = !enabledBlocklists.includes(blocklist.blocklist_id);
        }

        return matchesSearch && matchesFilter;
    });

    // Sort blocklists by last_modified (newest first) if "updated" is selected
    if (sortValue === "updated") {
        filteredBlocklists = filteredBlocklists.slice().sort((a, b) => {
            const aTime = a.last_modified ? new Date(a.last_modified).getTime() : 0;
            const bTime = b.last_modified ? new Date(b.last_modified).getTime() : 0;
            return bTime - aTime;
        });
    }

    // Enable Listed Button: active if any filter is set (not "all" or "enabled") and there are filtered blocklists
    const enableListedActive =
        filterValue !== "all" &&
        filterValue !== "enabled" &&
        filteredBlocklists.length > 0;

    // Handler to enable all filtered blocklists
    const handleEnableListed = async () => {
        if (!activeProfile?.profile_id || !enableListedActive) return;
        setUpdating("all");
        // Get all filtered blocklist IDs not already enabled
        const toEnable = filteredBlocklists
            .map(b => b.blocklist_id)
            .filter(id => !enabledBlocklists.includes(id));
        if (toEnable.length === 0) {
            setUpdating(null);
            return;
        }
        try {
            // Enable all at once using ApiBlocklistsUpdates
            await api.Client.profilesApi.apiV1ProfilesIdBlocklistsPost(
                activeProfile.profile_id,
                { blocklist_ids: toEnable } as ApiBlocklistsUpdates
            );
            // Refetch profile after enabling
            const updatedProfile = await api.Client.profilesApi.apiV1ProfilesIdGet(activeProfile.profile_id);
            setActiveProfile(updatedProfile.data);
            toast.success("Blocklists enabled", {
                description: "All filtered blocklists have been enabled successfully.",
            });
        } catch (e) {
            toast.error("Error", {
                description: "Failed to enable blocklists. Please try again.",
            });
        } finally {
            setUpdating(null);
        }
    };

    return (
        <div className="flex flex-col w-full items-start gap-6 p-6 md:p-8">
            {/* Page Description */}
            <section className="w-full">
                <p className="text-[var(--tailwind-colors-slate-200)] text-base leading-6">
                    Blocklists are collections of domains and IP addresses that help block trackers, ads, and malicious content. Choose from curated lists or individual providers to customize your DNS filtering experience.
                </p>
            </section>

            {/* Alert Card */}
            <section className="w-full">
                {showAlert && (
                    <AlertCard
                        description={
                            <>
                                <div>
                                    Blocklists are collections of domains and IP addresses you can enable to block trackers, advertisers, and malicious actors.
                                    Enabling several large blocklists may degrade your browsing experience. Start with one of our predefined lists that fits your protection needs:
                                    <span className="inline-flex gap-2 ml-1 align-baseline">
                                        <span
                                            className="underline cursor-pointer"
                                            onClick={() => setFilterValue("basic")}
                                        >
                                            Basic
                                        </span>
                                        <span
                                            className="underline cursor-pointer"
                                            onClick={() => setFilterValue("comprehensive")}
                                        >
                                            Comprehensive
                                        </span>
                                        <span
                                            className="underline cursor-pointer"
                                            onClick={() => setFilterValue("restrictive")}
                                        >
                                            Restrictive
                                        </span>
                                    </span>
                                </div>
                            </>
                        }
                        onClose={() => setShowAlert(false)}
                        className="w-full"
                    />
                )}
            </section>

            {/* Filters and Search (mobile-first layout similar to logs page) */}
            <section className="w-full flex flex-col gap-2.5">
                {/* Row 1: search only (mobile). Desktop search handled in row 2 */}
                <div className="flex items-start w-full md:hidden">
                    <div className="relative flex-1 min-w-0 w-full">
                        <SearchIcon className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-[var(--tailwind-colors-slate-400)]" />
                        <Input
                            className="h-11 min-h-11 pl-10 pr-3 py-2 !bg-[var(--shadcn-ui-app-background)] border-[var(--tailwind-colors-slate-700)] text-[var(--tailwind-colors-slate-200)] rounded-lg placeholder:text-[var(--tailwind-colors-slate-500)]"
                            placeholder="Search blocklists"
                            aria-label="Search blocklists"
                            value={searchValue}
                            onChange={e => setSearchValue(e.target.value)}
                        />
                    </div>
                </div>
                {/* Row 2: horizontal scroll filters line (mobile) / single row on desktop */}
                <div className="flex items-start gap-2 md:gap-3 w-full flex-wrap md:flex-nowrap overflow-visible md:overflow-x-auto no-scrollbar md:flex-row">
                    {/* Desktop search (hidden on mobile second row) */}
                    <div className="relative flex-1 min-w-0 hidden md:block">
                        <SearchIcon className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-[var(--tailwind-colors-slate-400)]" />
                        <Input
                            className="h-9 pl-10 pr-3 py-2 !bg-[var(--shadcn-ui-app-background)] border-[var(--tailwind-colors-slate-700)] text-[var(--tailwind-colors-slate-200)] rounded-lg placeholder:text-[var(--tailwind-colors-slate-400)]"
                            placeholder="Search blocklists"
                            aria-label="Search blocklists"
                            value={searchValue}
                            onChange={e => setSearchValue(e.target.value)}
                        />
                    </div>
                    {/* List Filter */}
                    <Select value={filterValue} onValueChange={setFilterValue}>
                        <SelectTrigger aria-label="Filter lists" className="h-11 md:h-9 min-h-11 md:min-h-0 flex-1 md:flex-none w-full md:w-auto md:min-w-[170px] md:max-w-xs px-2 md:px-3 !bg-[var(--shadcn-ui-app-background)] border-[var(--tailwind-colors-slate-700)] text-[var(--tailwind-colors-slate-50)] rounded-lg flex">
                            <div className="flex items-center gap-1 w-full min-w-0">
                                <ListFilterIcon className="h-4 w-4 shrink-0" />
                                <span className="text-sm truncate"><SelectValue placeholder="All lists" /></span>
                            </div>
                        </SelectTrigger>
                        <SelectContent>
                            <SelectItem value="all">All lists</SelectItem>
                            <div className="px-2 py-1 text-xs text-[var(--tailwind-colors-rdns-600)] font-semibold">Pre-defined lists</div>
                            {PREDEFINED_LISTS.map(({ label, tag }) => (
                                <SelectItem key={tag} value={tag}>{label}</SelectItem>
                            ))}
                            <div className="px-2 py-1 text-xs text-[var(--tailwind-colors-rdns-600)] font-semibold">Individual lists</div>
                            {INDIVIDUAL_LISTS.map(({ label, tag }) => (
                                <SelectItem key={tag} value={tag}>{label}</SelectItem>
                            ))}
                            <div className="px-2 py-1 text-xs text-[var(--tailwind-colors-rdns-600)] font-semibold">Status</div>
                            {STATUS_FILTERS.map(({ label, value }) => (
                                <SelectItem key={value} value={value}>{label}</SelectItem>
                            ))}
                        </SelectContent>
                    </Select>
                    {/* Sort By */}
                    <Select value={sortValue} onValueChange={setSortValue}>
                        <SelectTrigger aria-label="Sort blocklists" className="h-11 md:h-9 min-h-11 md:min-h-0 flex-1 md:flex-none w-full md:w-auto md:min-w-[140px] md:max-w-[180px] px-2 md:px-3 !bg-[var(--shadcn-ui-app-background)] border-[var(--tailwind-colors-slate-700)] text-[var(--tailwind-colors-slate-50)] rounded-lg flex">
                            <div className="flex items-center gap-1 w-full min-w-0">
                                <Clock className="h-4 w-4 shrink-0" />
                                <span className="text-sm truncate"><SelectValue placeholder="Updated" /></span>
                            </div>
                        </SelectTrigger>
                        <SelectContent>
                            <SelectItem value="updated">Updated</SelectItem>
                        </SelectContent>
                    </Select>
                    {/* Enable Listed Button (mobile & desktop at end of row) */}
                    <div className="flex-shrink-0 ml-auto">
                        <Button
                            aria-label="Enable listed blocklists"
                            variant="outline"
                            size="icon"
                            className={`w-11 h-11 md:h-11 min-h-11 !bg-[var(--shadcn-ui-app-background)] border-[var(--tailwind-colors-slate-700)] ${enableListedActive ? "opacity-100" : "opacity-50"}`}
                            disabled={!enableListedActive || updating === "all"}
                            onClick={handleEnableListed}
                            title="Enable currently listed blocklists"
                        >
                            <ToggleLeftIcon className={`w-4 h-4 ${enableListedActive ? 'text-[var(--tailwind-colors-rdns-600)]' : 'text-[var(--tailwind-colors-slate-500)]'}`} />
                        </Button>
                    </div>
                </div>
            </section>

            {/* Blocklist Cards */}
            <section className="w-full">
                {/*
                 * On tablets the previous combination of parent h-full, flex-1 and nested ScrollArea with h-full
                 * resulted in the ScrollArea viewport height being computed smaller than the content area created
                 * by stacked fixed headers, preventing the overall document from scrolling to the very bottom.
                 * We remove forced h-full and instead cap the ScrollArea only when there is sufficient vertical space.
                 */}
                <ScrollArea className="w-full max-h-[calc(100vh-var(--app-header-stack,120px)-200px)] md:max-h-[unset]">
                    <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 xl:grid-cols-4 gap-6 pb-8">
                        {loading ? (
                            <div className="col-span-full text-center text-[var(--tailwind-colors-slate-400)] py-8">
                                Loading blocklists...
                            </div>
                        ) : filteredBlocklists.length === 0 ? (
                            <div className="col-span-full flex justify-center py-8">
                                <EmptyState searchTerm={searchValue.trim() || undefined} />
                            </div>
                        ) : (
                            filteredBlocklists.map((blocklist) => {
                                const blocklistId = blocklist.blocklist_id;
                                const isEnabled = enabledBlocklists.includes(blocklistId);
                                return (
                                    <BlocklistCard
                                        key={blocklistId}
                                        title={blocklist.name}
                                        description={blocklist.description}
                                        entries={blocklist.entries}
                                        updated={
                                            blocklist.last_modified
                                                ? formatDistanceToNow(parseISO(blocklist.last_modified), { addSuffix: true })
                                                : ""
                                        }
                                        onSwitchChange={(checked) => handleBlocklistSwitch(blocklistId, checked)}
                                        switchChecked={isEnabled}
                                        switchDisabled={updating === blocklistId}
                                        homepage={blocklist.homepage}
                                    />
                                );
                            })
                        )}
                    </div>
                </ScrollArea>
            </section>
        </div>
    );
}