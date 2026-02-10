import { Button } from "@/components/ui/button";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Search } from "lucide-react";
import { useState, useEffect, useCallback, useMemo, type JSX } from "react";
import AlertCard from "@/components/general/AlertCard";
import { useNavigate } from "react-router-dom";
import CustomRulesSearch from "@/pages/custom_rules/Search";
import { useAppStore } from "@/store/general";
import api from "@/api/api";
import { toast } from "sonner";
import type { ModelAccount, ModelCustomRule, ModelProfile, ResponsesCustomRuleBatchSkipped } from "@/api/client/api";
import { RuleComposer, type RuleOption } from "@/pages/custom_rules/RuleComposer";
import CustomRulesCard from "@/pages/custom_rules/CustomRulesCard";

type RuleTab = "denylist" | "allowlist";

const TAB_TO_ACTION: Record<RuleTab, "block" | "allow"> = {
    denylist: "block",
    allowlist: "allow",
};

interface ApiErrorLike {
    response?: {
        data?: {
            error?: string;
            message?: string;
            detail?: string;
        };
    };
    message?: string;
}

const formatApiError = (error: unknown, fallback: string): string => {
    if (typeof error === "string") {
        return error;
    }

    if (error && typeof error === "object") {
        const err = error as ApiErrorLike;
        const data = err.response?.data;
        return data?.error ?? data?.message ?? data?.detail ?? err.message ?? fallback;
    }

    return fallback;
};


interface MainContentSectionProps {
    account: ModelAccount;
    profiles?: ModelProfile[];
}

export default function MainContentSection({ profiles = [] }: Omit<MainContentSectionProps, "account">): JSX.Element {
    const [showAlert, setShowAlert] = useState(true);
    const [showSearch, setShowSearch] = useState(false);
    const [activeTab, setActiveTab] = useState<RuleTab>("denylist");
    const [loading, setLoading] = useState(false);
    const [searchValue, setSearchValue] = useState("");
    const [selectedIds, setSelectedIds] = useState<string[]>([]);
    const [composerTokens, setComposerTokens] = useState<Record<RuleTab, RuleOption[]>>({
        denylist: [],
        allowlist: [],
    });

    const updateComposerTokens = useCallback((tab: RuleTab, next: RuleOption[]) => {
        setComposerTokens(prev => ({
            ...prev,
            [tab]: next,
        }));
    }, [setComposerTokens]);

    const navigate = useNavigate();

    const activeProfile = useAppStore((state) => state.activeProfile);
    const setActiveProfile = useAppStore((state) => state.setActiveProfile);

    useEffect(() => {
        if (!activeProfile && profiles.length > 0) {
            setActiveProfile(profiles[0]);
        }
    }, [activeProfile, profiles, setActiveProfile]);

    const customRules: ModelCustomRule[] = activeProfile?.settings?.custom_rules ?? [];
    const denylist = customRules.filter(rule => rule.action === "block");
    const allowlist = customRules.filter(rule => rule.action === "allow");
    const denylistHasRules = denylist.length > 0;
    const allowlistHasRules = allowlist.length > 0;
    const activeTabHasRules = activeTab === "denylist" ? denylistHasRules : allowlistHasRules;

    useEffect(() => {
        setComposerTokens({ denylist: [], allowlist: [] });
        setSelectedIds([]);
    }, [activeProfile?.profile_id]);

    useEffect(() => {
        if (!activeTabHasRules) {
            setShowSearch(false);
        }
    }, [activeTabHasRules]);

    const handleComposerSubmit = useCallback(async (tab: RuleTab) => {
        if (!activeProfile?.profile_id) {
            toast.error("Select a profile before adding custom rules.");
            return;
        }

        const originalTokens = composerTokens[tab];
        const staticTokens = originalTokens.filter(token => token.meta?.error);
        const submissionTokens = originalTokens.filter(token => !token.meta?.error);

        if (submissionTokens.length === 0) {
            return;
        }

        setLoading(true);
        try {
            const response = await api.Client.profilesApi.apiV1ProfilesIdCustomRulesBatchPost(
                activeProfile.profile_id,
                {
                    action: TAB_TO_ACTION[tab],
                    values: submissionTokens.map(token => token.value),
                }
            );

            const created = response.data?.created ?? [];
            const skipped = response.data?.skipped ?? [];

            if (created.length > 0) {
                const updated = await api.Client.profilesApi.apiV1ProfilesIdGet(activeProfile.profile_id);
                setActiveProfile(updated.data);
                toast.success(`${created.length} entr${created.length === 1 ? "y" : "ies"} added to the ${tab}.`);
            }

            if (skipped.length > 0) {
                toast.warning(`${skipped.length} entr${skipped.length === 1 ? "y was" : "ies were"} skipped. Review the highlighted items.`);
            }

            if (created.length === 0 && skipped.length === 0) {
                toast.info("No changes were made.");
            }

            if (skipped.length > 0 || staticTokens.length > 0) {
                const skippedByValue = new Map<string, ResponsesCustomRuleBatchSkipped>();
                skipped.forEach(item => {
                    if (item?.value) {
                        skippedByValue.set(item.value, item);
                    }
                });

                const skippedTokens = submissionTokens
                    .filter(token => skippedByValue.has(token.value))
                    .map(token => {
                        const skippedItem = skippedByValue.get(token.value);
                        return {
                            ...token,
                            meta: {
                                error: skippedItem?.message ?? "Unable to add entry.",
                                reason: skippedItem?.reason,
                            },
                        } satisfies RuleOption;
                    });

                const deduped: RuleOption[] = [];
                const seen = new Set<string>();
                [...staticTokens, ...skippedTokens].forEach(token => {
                    if (!seen.has(token.value)) {
                        seen.add(token.value);
                        deduped.push(token);
                    }
                });

                updateComposerTokens(tab, deduped);
            } else {
                updateComposerTokens(tab, []);
            }
        } catch (error: unknown) {
            toast.error(formatApiError(error, "Failed to add custom rules"));
        } finally {
            setLoading(false);
        }
    }, [activeProfile?.profile_id, composerTokens, setActiveProfile, updateComposerTokens]);

    // Handler for deleting a custom rule
    const handleDeleteRule = useCallback(async (userRuleId: string) => {
        if (!activeProfile?.profile_id) return;
        setLoading(true);
        try {
            await api.Client.profilesApi.apiV1ProfilesIdCustomRulesCustomRuleIdDelete(
                activeProfile.profile_id,
                userRuleId
            );
            // Fetch updated profile and update store
            const updated = await api.Client.profilesApi.apiV1ProfilesIdGet(activeProfile.profile_id);
            setActiveProfile(updated.data);
            toast.success("Custom rule deleted successfully.");
        } catch (error: unknown) {
            toast.error(formatApiError(error, "Failed to delete rule"));
        } finally {
            setLoading(false);
        }
    }, [activeProfile?.profile_id, setActiveProfile]);

    // Memoize handlers to prevent unnecessary re-renders
    const handleEntryCheck = useCallback((id: string, checked: boolean) => {
        setSelectedIds(prev => {
            if (checked) {
                // Add only if not already present
                return prev.includes(id) ? prev : [...prev, id];
            }
            // Remove from selection
            return prev.filter(selId => selId !== id);
        });
    }, []);

    // Memoize bulk delete handler
    const handleBulkDelete = useCallback(async () => {
        if (!activeProfile?.profile_id || selectedIds.length === 0) return;
        setLoading(true);
        try {
            await Promise.all(
                selectedIds.map(userRuleId =>
                    api.Client.profilesApi.apiV1ProfilesIdCustomRulesCustomRuleIdDelete(
                        activeProfile.profile_id,
                        userRuleId
                    )
                )
            );
            // Fetch updated profile and update store
            const updated = await api.Client.profilesApi.apiV1ProfilesIdGet(activeProfile.profile_id);
            setActiveProfile(updated.data);
            toast.success("Selected custom rules deleted successfully.");
            setSelectedIds([]);
        } catch (error: unknown) {
            toast.error(formatApiError(error, "Failed to delete selected rules"));
        } finally {
            setLoading(false);
        }
    }, [activeProfile?.profile_id, selectedIds, setActiveProfile]);

    // Show header only if at least one is selected
    const allSelected = selectedIds.length > 0;
    const selectedCount = selectedIds.length;

    // Filtered lists based on search value (memoized to prevent unnecessary recalculations)
    const filteredDenylist = useMemo(() =>
        denylist.filter(rule =>
            rule.value?.toLowerCase().includes(searchValue.toLowerCase())
        ), [denylist, searchValue]);

    const filteredAllowlist = useMemo(() =>
        allowlist.filter(rule =>
            rule.value?.toLowerCase().includes(searchValue.toLowerCase())
        ), [allowlist, searchValue]);

    // CustomRulesCard component moved below for clarity

    return (
        <div className="flex flex-col flex-1 w-full h-full min-h-screen md:min-h-0 items-start gap-6 p-6 pt-8 md:pt-8 md:p-8 overflow-visible">

            <div className="flex w-full h-full flex-1 items-start relative min-h-0">
                <div className="flex flex-col flex-1 h-full w-full min-h-0">
                    <Tabs
                        defaultValue="denylist"
                        value={activeTab}
                        onValueChange={tab => {
                            setActiveTab(tab as "denylist" | "allowlist");
                            setSelectedIds([]); // Reset selection when switching tabs
                        }}
                        className="w-full"
                    >
                        <div className="w-full border-b border-[var(--tailwind-colors-slate-700)] overflow-x-auto no-scrollbar">
                            <TabsList className="flex h-auto w-fit bg-transparent rounded-none gap-0 justify-start p-0 border-b-0 min-w-max">
                                <TabsTrigger
                                    value="denylist"
                                    className="relative rounded-none border-t border-l border-r border-b-2 bg-transparent px-6 sm:px-10 md:px-16 lg:px-20 py-2 sm:py-2.5 md:py-3 text-[var(--tailwind-colors-slate-300)] border-transparent data-[state=active]:!bg-transparent dark:data-[state=active]:!bg-transparent data-[state=active]:shadow-none data-[state=active]:text-[var(--tailwind-colors-slate-50)] data-[state=active]:!border-t-[var(--tailwind-colors-slate-light-300)] data-[state=active]:!border-l-[var(--tailwind-colors-slate-light-300)] data-[state=active]:!border-r-[var(--tailwind-colors-slate-light-300)] dark:data-[state=active]:!border-t-[var(--tailwind-colors-slate-700)] dark:data-[state=active]:!border-l-[var(--tailwind-colors-slate-700)] dark:data-[state=active]:!border-r-[var(--tailwind-colors-slate-700)] data-[state=active]:!border-b-[var(--tailwind-colors-rdns-600)] hover:text-[var(--tailwind-colors-slate-50)] transition-colors duration-200 ease-out after:absolute after:left-0 after:right-0 after:-bottom-[2px] after:h-[2px] after:rounded-full after:bg-[var(--tailwind-colors-rdns-600)] after:opacity-0 after:transition-opacity after:duration-200 after:ease-out hover:after:opacity-40 data-[state=active]:after:opacity-0"
                                >
                                    Denylist
                                </TabsTrigger>
                                <TabsTrigger
                                    value="allowlist"
                                    className="relative rounded-none border-t border-l border-r border-b-2 bg-transparent px-6 sm:px-10 md:px-16 lg:px-20 py-2 sm:py-2.5 md:py-3 text-[var(--tailwind-colors-slate-300)] border-transparent data-[state=active]:!bg-transparent dark:data-[state=active]:!bg-transparent data-[state=active]:shadow-none data-[state=active]:text-[var(--tailwind-colors-slate-50)] data-[state=active]:!border-t-[var(--tailwind-colors-slate-light-300)] data-[state=active]:!border-l-[var(--tailwind-colors-slate-light-300)] data-[state=active]:!border-r-[var(--tailwind-colors-slate-light-300)] dark:data-[state=active]:!border-t-[var(--tailwind-colors-slate-700)] dark:data-[state=active]:!border-l-[var(--tailwind-colors-slate-700)] dark:data-[state=active]:!border-r-[var(--tailwind-colors-slate-700)] data-[state=active]:!border-b-[var(--tailwind-colors-rdns-600)] hover:text-[var(--tailwind-colors-slate-50)] transition-colors duration-200 ease-out after:absolute after:left-0 after:right-0 after:-bottom-[2px] after:h-[2px] after:rounded-full after:bg-[var(--tailwind-colors-rdns-600)] after:opacity-0 after:transition-opacity after:duration-200 after:ease-out hover:after:opacity-40 data-[state=active]:after:opacity-0"
                                >
                                    Allowlist
                                </TabsTrigger>
                            </TabsList>
                        </div>

                        {/* Page Description */}
                        <section className="w-full mt-4">
                            <p className="text-[var(--tailwind-colors-slate-200)] text-base leading-6">
                                Manually add domains and IP addresses to either block or allow when resolving.
                            </p>
                        </section>

                        {/* Shared AlertCard and input for both tabs */}
                        <section className="w-full pt-4 pb-0">
                            {showAlert && (
                                <AlertCard
                                    description={
                                        <>
                                            <div>
                                                Custom rules take precedence over blocklists and other settings. You can add domains, IP addresses, or ASNs (e.g. AS15169). Subdomains of custom rules entries are included by default (*.domain) - you can change this in <span
                                                    className="underline cursor-pointer"
                                                    onClick={() => navigate("/settings")}
                                                >
                                                    Settings
                                                </span>. Wildcard options are available, see <span
                                                    className="underline cursor-pointer"
                                                    onClick={() => navigate("/faq")}
                                                >
                                                    FAQ
                                                </span>.
                                            </div>
                                        </>
                                    }
                                    onClose={() => setShowAlert(false)}
                                />
                            )}
                        </section>

                        <div className="flex flex-col gap-3 w-full">
                            <div className="flex flex-row flex-wrap md:flex-row items-stretch md:items-start gap-3 w-full min-w-0">
                                <div className="flex flex-row flex-1 items-stretch md:items-start gap-3 min-w-0">
                                    <RuleComposer
                                        action={activeTab}
                                        tokens={composerTokens[activeTab]}
                                        onTokensChange={(next) => updateComposerTokens(activeTab, next)}
                                        onSubmit={() => handleComposerSubmit(activeTab)}
                                        loading={loading || !activeProfile?.profile_id}
                                        className="flex-1 min-w-0"
                                    />
                                    <Button
                                        className={`w-11 h-11 md:w-auto md:h-9 rounded-md flex items-center justify-center md:px-4 md:gap-2 ${showSearch
                                            ? "bg-[var(--tailwind-colors-slate-800)] text-[var(--tailwind-colors-slate-400)]"
                                            : "bg-[var(--tailwind-colors-rdns-600)] text-background"}`}
                                        onClick={() => setShowSearch((prev) => !prev)}
                                        aria-label={showSearch ? 'Close search' : 'Open search'}
                                        disabled={!activeTabHasRules}
                                    >
                                        <Search className="w-4 h-4" />
                                        <span className="hidden md:inline text-sm font-medium">
                                            {showSearch ? 'Close search' : 'Search'}
                                        </span>
                                    </Button>
                                </div>
                            </div>

                            {/* Show Search.tsx here if toggled */}
                            {showSearch && (
                                <div className="w-full bg-background">
                                    <CustomRulesSearch
                                        value={searchValue}
                                        onChange={setSearchValue}
                                        allSelected={
                                            (activeTab === "denylist"
                                                ? filteredDenylist
                                                : filteredAllowlist
                                            ).length > 0 &&
                                            (activeTab === "denylist"
                                                ? filteredDenylist
                                                : filteredAllowlist
                                            ).every(r => selectedIds.includes(r.id))
                                        }
                                        onSelectAll={() => {
                                            const visibleIds = (activeTab === "denylist" ? filteredDenylist : filteredAllowlist).map(r => r.id);
                                            setSelectedIds(visibleIds);
                                        }}
                                        onDeselectAll={() => {
                                            setSelectedIds([]);
                                        }}
                                    />
                                </div>
                            )}
                        </div>

                        <TabsContent value="denylist" className="flex flex-col gap-4 mt-2 flex-1">
                            <CustomRulesCard
                                rules={filteredDenylist}
                                selectedIds={selectedIds}
                                onCheck={handleEntryCheck}
                                onDelete={(id: string) => { void handleDeleteRule(id); }}
                                allSelected={allSelected}
                                selectedCount={selectedCount}
                                handleBulkDelete={handleBulkDelete}
                                loading={loading}
                                type="denied"
                                searchQuery={searchValue}
                            />
                        </TabsContent>
                        <TabsContent value="allowlist" className="flex flex-col gap-4 mt-2 flex-1">
                            <CustomRulesCard
                                rules={filteredAllowlist}
                                selectedIds={selectedIds}
                                onCheck={handleEntryCheck}
                                onDelete={(id: string) => { void handleDeleteRule(id); }}
                                allSelected={allSelected}
                                selectedCount={selectedCount}
                                handleBulkDelete={handleBulkDelete}
                                loading={loading}
                                type="allowed"
                                searchQuery={searchValue}
                            />
                        </TabsContent>
                    </Tabs>
                </div>
            </div>
        </div>
    );
}
