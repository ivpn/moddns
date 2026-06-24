import { Button } from "@/components/ui/button";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Search } from "lucide-react";
import { useState, useEffect, useCallback, useMemo, type JSX } from "react";
import AlertCard from "@/components/general/AlertCard";
import { useNavigate } from "react-router-dom";
import CustomRulesSearch from "@/pages/custom_rules/Search";
import { useAppStore } from "@/store/general";
import { useSubscriptionGuard } from "@/hooks/useSubscriptionGuard";
import LimitedAccessBanner from "@/components/LimitedAccessBanner";
import BetaEndingBanner from "@/components/BetaEndingBanner";
import api from "@/api/api";
import { toast } from "sonner";
import type { ModelAccount, ModelCustomRule, ModelProfile, ResponsesCustomRuleBatchSkipped } from "@/api/client/api";
import { RuleComposer, type RuleOption } from "@/pages/custom_rules/RuleComposer";
import CustomRulesCard from "@/pages/custom_rules/CustomRulesCard";
import RuleEditDialog from "@/pages/custom_rules/RuleEditDialog";
import type { RequestsUpdateProfileCustomRuleBody, RequestsCustomRuleGroupUpdate } from "@/api/client/api";
import { RequestsCustomRuleGroupUpdateOperationEnum as GroupOp } from "@/api/client/api";
import CustomRulesExportLimitBanner from "@/pages/custom_rules/CustomRulesExportLimitBanner";
import { formatApiError } from "@/lib/apiError";

type RuleTab = "denylist" | "allowlist";

// toPointer encodes a group name as a single-segment RFC6901 JSON Pointer so it can
// travel in the group-ops request body (never the URL). Escape ~ before /.
const toPointer = (name: string): string => `/${name.replace(/~/g, "~0").replace(/\//g, "~1")}`;

const TAB_TO_ACTION: Record<RuleTab, "block" | "allow"> = {
    denylist: "block",
    allowlist: "allow",
};



interface MainContentSectionProps {
    account: ModelAccount;
    profiles?: ModelProfile[];
}

export default function MainContentSection({ profiles = [] }: Omit<MainContentSectionProps, "account">): JSX.Element {
    const { isRestricted } = useSubscriptionGuard();
    const customRulesAlertDismissed = useAppStore((state) => state.customRulesAlertDismissed);
    const setCustomRulesAlertDismissed = useAppStore((state) => state.setCustomRulesAlertDismissed);
    const [showSearch, setShowSearch] = useState(false);
    const [activeTab, setActiveTab] = useState<RuleTab>("denylist");
    const [loading, setLoading] = useState(false);
    const [searchValue, setSearchValue] = useState("");
    const [selectedIds, setSelectedIds] = useState<string[]>([]);
    const [editingRule, setEditingRule] = useState<ModelCustomRule | null>(null);
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

    const customRules: ModelCustomRule[] = useMemo(
        () => activeProfile?.settings?.custom_rules ?? [],
        [activeProfile?.settings?.custom_rules],
    );
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

    const handleComposerSubmit = useCallback(async (tab: RuleTab, tokensOverride?: RuleOption[]) => {
        if (!activeProfile?.profile_id) {
            toast.error("Select a profile before adding custom rules.");
            return;
        }

        // tokensOverride is supplied by the Add button so a value typed-but-not-yet-
        // chipped is included without waiting for a parent re-render.
        const originalTokens = tokensOverride ?? composerTokens[tab];
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

    // Edit a single rule in place via the PATCH endpoint, then refetch to sync.
    const handleSaveEdit = useCallback(async (ruleId: string, patch: RequestsUpdateProfileCustomRuleBody) => {
        if (!activeProfile?.profile_id) return;
        if (Object.keys(patch).length === 0) {
            setEditingRule(null);
            return;
        }
        setLoading(true);
        try {
            await api.Client.profilesApi.apiV1ProfilesProfileIdCustomRulesCustomRuleIdPatch(
                activeProfile.profile_id,
                ruleId,
                patch,
            );
            const updated = await api.Client.profilesApi.apiV1ProfilesIdGet(activeProfile.profile_id);
            setActiveProfile(updated.data);
            toast.success("Rule updated.");
            setEditingRule(null);
        } catch (error: unknown) {
            toast.error(formatApiError(error, "Failed to update rule"));
        } finally {
            setLoading(false);
        }
    }, [activeProfile?.profile_id, setActiveProfile]);

    // Persist a drag-reorder. The card sends the active tab's full ordered IDs; we
    // build the complete per-profile order (other tab keeps its order) so the
    // backend renumbers everything without cross-tab collisions.
    const handleReorder = useCallback(async (tab: RuleTab, orderedIdsForTab: string[]) => {
        if (!activeProfile?.profile_id) return;
        const byOrder = (a: ModelCustomRule, b: ModelCustomRule) => (a.order ?? 0) - (b.order ?? 0);
        const denyIds = tab === "denylist" ? orderedIdsForTab : denylist.slice().sort(byOrder).map(r => r.id);
        const allowIds = tab === "allowlist" ? orderedIdsForTab : allowlist.slice().sort(byOrder).map(r => r.id);
        const fullOrder = [...denyIds, ...allowIds];
        try {
            await api.Client.profilesApi.apiV1ProfilesIdCustomRulesOrderPatch(
                activeProfile.profile_id,
                { order: fullOrder },
            );
            const updated = await api.Client.profilesApi.apiV1ProfilesIdGet(activeProfile.profile_id);
            setActiveProfile(updated.data);
        } catch (error: unknown) {
            toast.error(formatApiError(error, "Failed to reorder rules"));
            // Revert optimistic ordering by refetching the persisted state.
            const updated = await api.Client.profilesApi.apiV1ProfilesIdGet(activeProfile.profile_id);
            setActiveProfile(updated.data);
        }
    }, [activeProfile?.profile_id, denylist, allowlist, setActiveProfile]);

    // All group-registry mutations go through one JSON-Patch endpoint. Group names
    // travel in the JSON-Pointer path/from (RFC6901), never the URL. Reverts via
    // refetch on error.
    const applyGroupOps = useCallback(async (
        ops: RequestsCustomRuleGroupUpdate[],
        failMsg: string,
        successMsg?: string,
    ) => {
        if (!activeProfile?.profile_id) return;
        try {
            await api.Client.profilesApi.apiV1ProfilesIdCustomRuleGroupsPatch(
                activeProfile.profile_id, { updates: ops },
            );
            const updated = await api.Client.profilesApi.apiV1ProfilesIdGet(activeProfile.profile_id);
            setActiveProfile(updated.data);
            if (successMsg) toast.success(successMsg);
        } catch (error: unknown) {
            toast.error(formatApiError(error, failMsg));
        }
    }, [activeProfile?.profile_id, setActiveProfile]);

    // Save (or clear) a group note. A cleared note keeps the group (replace "").
    const handleGroupNote = useCallback((group: string, note: string | null) => {
        void applyGroupOps(
            [{ operation: GroupOp.Replace, path: toPointer(group), value: note ?? "" }],
            "Failed to save group note",
        );
    }, [applyGroupOps]);

    // Move a rule to another group (drag): set its group, then renumber the tab's
    // full order, then a single refetch. Reverts via refetch on error.
    const handleMoveRule = useCallback(async (tab: RuleTab, orderedIdsForTab: string[], ruleId: string, newGroup: string) => {
        if (!activeProfile?.profile_id) return;
        const byOrder = (a: ModelCustomRule, b: ModelCustomRule) => (a.order ?? 0) - (b.order ?? 0);
        const denyIds = tab === "denylist" ? orderedIdsForTab : denylist.slice().sort(byOrder).map(r => r.id);
        const allowIds = tab === "allowlist" ? orderedIdsForTab : allowlist.slice().sort(byOrder).map(r => r.id);
        const fullOrder = [...denyIds, ...allowIds];
        try {
            await api.Client.profilesApi.apiV1ProfilesProfileIdCustomRulesCustomRuleIdPatch(
                activeProfile.profile_id, ruleId, { group: newGroup },
            );
            await api.Client.profilesApi.apiV1ProfilesIdCustomRulesOrderPatch(
                activeProfile.profile_id, { order: fullOrder },
            );
            const updated = await api.Client.profilesApi.apiV1ProfilesIdGet(activeProfile.profile_id);
            setActiveProfile(updated.data);
        } catch (error: unknown) {
            toast.error(formatApiError(error, "Failed to move rule"));
            const updated = await api.Client.profilesApi.apiV1ProfilesIdGet(activeProfile.profile_id);
            setActiveProfile(updated.data);
        }
    }, [activeProfile?.profile_id, denylist, allowlist, setActiveProfile]);

    // Create an empty group (registers it in the custom_rule_groups map).
    const handleCreateGroup = useCallback((name: string) => {
        void applyGroupOps(
            [{ operation: GroupOp.Add, path: toPointer(name), value: "" }],
            "Failed to create group",
        );
    }, [applyGroupOps]);

    const handleRenameGroup = useCallback((from: string, to: string) => {
        void applyGroupOps(
            [{ operation: GroupOp.Move, from: toPointer(from), path: toPointer(to) }],
            "Failed to rename group",
            `Group renamed to "${to}".`,
        );
    }, [applyGroupOps]);

    const handleDeleteGroup = useCallback((name: string) => {
        void applyGroupOps(
            [{ operation: GroupOp.Remove, path: toPointer(name) }],
            "Failed to delete group",
            `Group "${name}" deleted. Its rules moved to Ungrouped.`,
        );
    }, [applyGroupOps]);

    const groupNotes: Record<string, string> = activeProfile?.settings?.custom_rule_groups ?? {};
    const existingGroups = useMemo(
        () => Array.from(
            new Set([
                ...customRules.map(r => r.group).filter((g): g is string => !!g && g.trim() !== ""),
                ...Object.keys(activeProfile?.settings?.custom_rule_groups ?? {}).filter(g => g.trim() !== ""),
            ]),
        ).sort(),
        [customRules, activeProfile?.settings?.custom_rule_groups],
    );

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
            <BetaEndingBanner />
            <LimitedAccessBanner />
            <CustomRulesExportLimitBanner />
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
                        {/* TabsList stays interactive in LA so the user can switch tabs to view their existing rules in either list. */}
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

                        {/* Mutations (composer / delete / bulk delete) are blocked in LA — gate everything below the tab strip. */}
                        {/* Inner uses `flex flex-col gap-2` because shadcn Tabs applies the same on its children — wrapping the children
                            in plain divs would otherwise drop the inter-section spacing develop relies on. */}
                        <div title={isRestricted ? "Feature unavailable in limited access mode" : undefined} className={`w-full${isRestricted ? ' cursor-not-allowed' : ''}`}>
                        <div className={`flex flex-col gap-2 w-full${isRestricted ? ' opacity-50 pointer-events-none' : ''}`}>
                        {/* Page Description */}
                        <section className="w-full mt-4">
                            <p className="text-[var(--tailwind-colors-slate-200)] text-base leading-6">
                                Manually add domains and IP addresses to either block or allow when resolving.
                            </p>
                        </section>

                        {/* Shared AlertCard and input for both tabs */}
                        <section className="w-full pt-4 pb-0">
                            {!customRulesAlertDismissed && (
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
                                    onClose={() => setCustomRulesAlertDismissed(true)}
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
                                        onSubmit={(override) => handleComposerSubmit(activeTab, override)}
                                        loading={loading || !activeProfile?.profile_id || isRestricted}
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
                                groupNotes={groupNotes}
                                selectedIds={selectedIds}
                                onCheck={handleEntryCheck}
                                onDelete={(id: string) => { void handleDeleteRule(id); }}
                                onEdit={setEditingRule}
                                onReorder={(ids) => handleReorder("denylist", ids)}
                                onMoveRule={(ids, ruleId, g) => handleMoveRule("denylist", ids, ruleId, g)}
                                onSaveGroupNote={handleGroupNote}
                                onCreateGroup={handleCreateGroup}
                                onRenameGroup={handleRenameGroup}
                                onDeleteGroup={handleDeleteGroup}
                                allSelected={allSelected}
                                selectedCount={selectedCount}
                                handleBulkDelete={handleBulkDelete}
                                loading={loading || isRestricted}
                                type="denied"
                                searchQuery={searchValue}
                            />
                        </TabsContent>
                        <TabsContent value="allowlist" className="flex flex-col gap-4 mt-2 flex-1">
                            <CustomRulesCard
                                rules={filteredAllowlist}
                                groupNotes={groupNotes}
                                selectedIds={selectedIds}
                                onCheck={handleEntryCheck}
                                onDelete={(id: string) => { void handleDeleteRule(id); }}
                                onEdit={setEditingRule}
                                onReorder={(ids) => handleReorder("allowlist", ids)}
                                onMoveRule={(ids, ruleId, g) => handleMoveRule("allowlist", ids, ruleId, g)}
                                onSaveGroupNote={handleGroupNote}
                                onCreateGroup={handleCreateGroup}
                                onRenameGroup={handleRenameGroup}
                                onDeleteGroup={handleDeleteGroup}
                                allSelected={allSelected}
                                selectedCount={selectedCount}
                                handleBulkDelete={handleBulkDelete}
                                loading={loading || isRestricted}
                                type="allowed"
                                searchQuery={searchValue}
                            />
                        </TabsContent>
                        </div>
                        </div>
                    </Tabs>
                </div>
            </div>

            <RuleEditDialog
                rule={editingRule}
                open={editingRule !== null}
                onOpenChange={(open) => { if (!open) setEditingRule(null); }}
                existingGroups={existingGroups}
                loading={loading}
                onSave={handleSaveEdit}
            />
        </div>
    );
}
