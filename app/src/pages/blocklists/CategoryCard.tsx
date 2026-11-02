import React from "react";
import { ChevronDown } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { Switch } from "@/components/ui/switch";

type CategoryState = "all" | "none" | "partial";

interface CategoryCardProps {
    label: string;
    description: string;
    totalLists: number;
    enabledLists: number;
    totalEntries: string;
    lastUpdated: string;
    onToggle: () => void;
    toggleDisabled: boolean;
    expanded: boolean;
    onExpandToggle: () => void;
}

function getCategoryState(enabledLists: number, totalLists: number): CategoryState {
    if (enabledLists === 0) return "none";
    if (enabledLists >= totalLists) return "all";
    return "partial";
}

const CategoryCard: React.FC<CategoryCardProps> = ({
    label,
    description,
    totalLists,
    enabledLists,
    totalEntries,
    lastUpdated,
    onToggle,
    toggleDisabled,
    expanded,
    onExpandToggle,
}) => {
    const state = getCategoryState(enabledLists, totalLists);

    const switchBgClass = "data-[state=checked]:bg-[var(--tailwind-colors-rdns-600)]";

    return (
        <Card
            data-testid="category-card"
            className={`bg-transparent dark:bg-[var(--variable-collection-surface)] p-3 border shadow-sm flex flex-col justify-between h-[196px] lg:h-[180px] w-full overflow-hidden transition-all duration-200 ${
                expanded
                    ? "border-[var(--tailwind-colors-rdns-600)]/80 shadow-[0_0_12px_rgba(18,164,149,0.2)]"
                    : "border-[var(--tailwind-colors-slate-light-300)] dark:border-transparent"
            } rounded-[var(--tailwind-primitives-border-radius-rounded)]`}
        >
            <CardContent className="p-0 flex flex-col justify-between h-full">
                <div className="flex flex-col gap-1">
                    {/* Top row: icon + label + optional badge + switch */}
                    <div className="flex items-start justify-between gap-2">
                        <div className="flex items-start gap-2 min-w-0 flex-1">
                            <div className="flex items-center gap-1.5 min-w-0">
                                <span className="text-tailwind-colors-slate-50 font-semibold text-base leading-tight line-clamp-2 break-words" title={label}>
                                    {label}
                                    {state !== "none" && (
                                        <span className="ml-1.5 inline-flex items-center align-middle px-1.5 py-0.5 rounded text-[10px] font-medium leading-none bg-[var(--tailwind-colors-rdns-600)]/15 text-[var(--tailwind-colors-rdns-600)] border border-[var(--tailwind-colors-rdns-600)]/25">
                                            {enabledLists}/{totalLists}
                                        </span>
                                    )}
                                </span>
                            </div>
                        </div>
                        <Switch
                            checked={state !== "none"}
                            onCheckedChange={onToggle}
                            disabled={toggleDisabled}
                            className={`w-9 h-5
                                data-[state=unchecked]:bg-[var(--tailwind-colors-slate-700)]
                                ${switchBgClass}
                                [&>[data-slot=switch-thumb]]:bg-background
                                data-[state=checked]:[&>[data-slot=switch-thumb]]:bg-[var(--tailwind-colors-slate-50)]
                                data-[state=unchecked]:[&>[data-slot=switch-thumb]]:bg-[var(--tailwind-colors-slate-400)]
                                data-[state=checked]:[&>[data-slot=switch-thumb]]:translate-x-4`}
                        />
                    </div>
                    {/* Description */}
                    <div className="pt-2 font-text-xs-leading-5-normal text-[var(--tailwind-colors-slate-100)] text-xs h-[72px] overflow-hidden text-ellipsis [display:-webkit-box] [-webkit-line-clamp:3] [-webkit-box-orient:vertical] break-words hyphens-none">
                        {description}
                    </div>
                </div>

                {/* Bottom row: expand trigger + stats */}
                <div className="mt-4 flex items-center justify-between gap-3 text-xs text-[var(--tailwind-colors-slate-200)]">
                    <button
                        type="button"
                        onClick={onExpandToggle}
                        className="flex shrink-0 items-center gap-1 whitespace-nowrap text-[var(--tailwind-colors-rdns-600)] hover:text-[var(--tailwind-colors-rdns-500)] transition-colors cursor-pointer"
                    >
                        <ChevronDown
                            className={`h-3.5 w-3.5 transition-transform duration-200 ${expanded ? "rotate-180" : ""}`}
                        />
                        <span className="text-xs font-medium">
                            {totalLists} {totalLists === 1 ? "list" : "lists"}
                        </span>
                    </button>
                    <div className="flex items-center gap-2 min-w-0">
                        <span className="shrink-0 whitespace-nowrap" title={`${totalEntries} entries`}>
                            {totalEntries} entries
                        </span>
                        {lastUpdated && (
                            <>
                                <span className="hidden sm:inline-block h-3 w-px shrink-0 bg-[var(--tailwind-colors-slate-600)]" />
                                <span className="truncate hidden sm:inline" title={`Updated ${lastUpdated}`}>
                                    {lastUpdated}
                                </span>
                            </>
                        )}
                    </div>
                </div>
            </CardContent>
        </Card>
    );
};

export default CategoryCard;
