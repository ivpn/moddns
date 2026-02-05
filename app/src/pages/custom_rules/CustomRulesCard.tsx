import { useCallback, useState, type JSX } from "react";
import { Card } from "@/components/ui/card";
import { MinusIcon, Trash2 } from "lucide-react";
import type { ModelCustomRule } from "@/api/client/api";
import NoRulesExist from "@/pages/custom_rules/NoRulesExist";
import CustomRuleEntry from "@/pages/custom_rules/Entry";

export interface CustomRulesCardProps {
    rules: ModelCustomRule[];
    selectedIds: string[];
    onCheck: (id: string, checked: boolean) => void;
    onDelete: (id: string) => void | Promise<void>;
    allSelected: boolean;
    selectedCount: number;
    handleBulkDelete: () => void | Promise<void>;
    loading: boolean;
    type: "denied" | "allowed";
    searchQuery: string;
}

export default function CustomRulesCard({
    rules,
    selectedIds,
    onCheck,
    onDelete,
    allSelected,
    selectedCount,
    handleBulkDelete,
    loading,
    type,
    searchQuery,
}: CustomRulesCardProps): JSX.Element {
    const [removingIds, setRemovingIds] = useState<string[]>([]);

    const handleEntryDelete = useCallback((id: string) => {
        setRemovingIds(prev => [...prev, id]);
        setTimeout(() => {
            onDelete(id);
            setRemovingIds(prev => prev.filter(rid => rid !== id));
        }, 300);
    }, [onDelete]);

    if (rules.length === 0) {
        if (searchQuery.trim().length > 0) {
            return (
                <Card className="flex flex-1 h-full items-center justify-center border-[var(--tailwind-colors-slate-600)] rounded-md bg-background">
                    <NoRulesExist
                        type={type}
                        title="No results found"
                        message={`Try a different search term or clear your search to see all ${type === "denied" ? "denylist" : "allowlist"} domains.`}
                    />
                </Card>
            );
        }

        return (
            <Card className="flex flex-col flex-1 self-stretch w-full grow bg-transparent dark:bg-[var(--variable-collection-surface)] rounded-lg border border-[var(--tailwind-colors-slate-light-300)] dark:border-transparent">
                <div className="flex flex-col items-center gap-6 p-6 w-full text-center">
                    <NoRulesExist
                        type={type}
                        title={type === "allowed" ? "There are no allowed domains yet." : undefined}
                    />
                </div>
            </Card>
        );
    }

    return (
        <div className="flex flex-col items-start gap-2 relative flex-1 self-stretch w-full grow rounded-md">
            {allSelected && selectedCount > 0 && (
                <div className="flex justify-between py-2 px-4 w-full bg-[var(--tailwind-colors-slate-900)] border border-solid border-[var(--tailwind-colors-slate-600)] rounded-md items-center">
                    <div className="inline-flex items-center gap-4">
                        <div className="relative w-4 h-4 bg-[var(--tailwind-colors-rdns-600)] rounded-[var(--shadcn-ui-radius-radius-sm)] border-[var(--tailwind-colors-rdns-600)]">
                            <MinusIcon className="absolute w-3.5 h-3.5 top-px left-px" />
                        </div>
                        <div className="font-text-sm-leading-5-normal text-[var(--tailwind-colors-slate-50)] text-[length:var(--text-sm-leading-5-normal-font-size)] tracking-[var(--text-sm-leading-5-normal-letter-spacing)] leading-[var(--text-sm-leading-5-normal-line-height)]">
                            {selectedCount} selected
                        </div>
                        <button
                            className="flex w-10 h-10 items-center justify-center rounded-[var(--primitives-radius-radius-md)] hover:bg-[var(--tailwind-colors-rdns-600)] group"
                            onClick={handleBulkDelete}
                            disabled={loading}
                            title="Delete selected entries"
                            aria-label="Delete selected entries"
                        >
                            <Trash2 className="w-4 h-4 text-[var(--tailwind-colors-rdns-600)] group-hover:text-[var(--tailwind-colors-slate-900)] transition-colors" />
                        </button>
                    </div>
                </div>
            )}
            {rules.map((rule) => {
                const checked = selectedIds.includes(rule.id);
                const isRemoving = removingIds.includes(rule.id);
                return (
                    <CustomRuleEntry
                        key={rule.id}
                        rule={rule}
                        checked={checked}
                        onCheck={onCheck}
                        onDelete={handleEntryDelete}
                        isRemoving={isRemoving}
                        hideDeleteButton={allSelected}
                    />
                );
            })}
        </div>
    );
}
