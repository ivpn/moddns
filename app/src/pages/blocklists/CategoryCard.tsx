import React, { useRef, useEffect, useState, type ReactNode } from "react";
import type { LucideIcon } from "lucide-react";
import { ChevronDown } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { Switch } from "@/components/ui/switch";

type CategoryState = "all" | "none" | "partial";

interface CategoryCardProps {
    icon: LucideIcon;
    label: string;
    description: string;
    totalLists: number;
    enabledLists: number;
    totalRecommended: number;
    totalEntries: string;
    lastUpdated: string;
    onToggle: () => void;
    toggleDisabled: boolean;
    expanded: boolean;
    onExpandToggle: () => void;
    children?: ReactNode;
}

function getCategoryState(enabledLists: number, totalRecommended: number): CategoryState {
    if (enabledLists === 0) return "none";
    if (enabledLists >= totalRecommended) return "all";
    return "partial";
}

const CategoryCard: React.FC<CategoryCardProps> = ({
    icon: Icon,
    label,
    description,
    totalLists,
    enabledLists,
    totalRecommended,
    totalEntries,
    lastUpdated,
    onToggle,
    toggleDisabled,
    expanded,
    onExpandToggle,
    children,
}) => {
    const state = getCategoryState(enabledLists, totalRecommended);
    const expandRef = useRef<HTMLDivElement>(null);
    const [height, setHeight] = useState(0);

    useEffect(() => {
        if (expandRef.current) {
            setHeight(expandRef.current.scrollHeight);
        }
    }, [expanded, children]);

    const switchBgClass =
        state === "partial"
            ? "data-[state=checked]:bg-amber-500"
            : "data-[state=checked]:bg-[var(--tailwind-colors-rdns-600)]";

    return (
        <div className="flex flex-col">
            <Card
                data-testid="category-card"
                className="bg-transparent dark:bg-[var(--variable-collection-surface)] p-3 border border-[var(--tailwind-colors-slate-light-300)] dark:border-transparent rounded-[var(--tailwind-primitives-border-radius-rounded)] shadow-sm flex flex-col justify-between h-[196px] lg:h-[180px] w-full overflow-hidden"
            >
                <CardContent className="p-0 flex flex-col justify-between h-full">
                    <div className="flex flex-col gap-1">
                        {/* Top row: icon + label + optional badge + switch */}
                        <div className="flex items-start justify-between gap-2">
                            <div className="flex items-start gap-2 min-w-0 max-w-[70%] md:max-w-[75%] lg:max-w-[80%]">
                                <Icon className="h-5 w-5 mt-0.5 shrink-0 text-[var(--tailwind-colors-rdns-600)]" />
                                <div className="flex items-center gap-1.5 min-w-0">
                                    <span className="text-tailwind-colors-slate-50 font-semibold text-base leading-tight truncate">
                                        {label}
                                    </span>
                                    {state === "partial" && (
                                        <span className="shrink-0 inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-medium leading-none bg-amber-500/15 text-amber-400 border border-amber-500/25">
                                            {enabledLists}/{totalRecommended}
                                        </span>
                                    )}
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
                        <div className="pt-2 font-text-xs-leading-5-normal text-[var(--tailwind-colors-slate-100)] text-xs h-[72px] overflow-hidden text-ellipsis [display:-webkit-box] [-webkit-line-clamp:3] [-webkit-box-orient:vertical] break-words hyphens-auto">
                            {description}
                        </div>
                    </div>

                    {/* Bottom row: expand trigger + stats */}
                    <div className="mt-4 flex items-center justify-between text-xs text-[var(--tailwind-colors-slate-200)]">
                        <button
                            type="button"
                            onClick={onExpandToggle}
                            className="flex items-center gap-1 text-[var(--tailwind-colors-rdns-600)] hover:text-[var(--tailwind-colors-rdns-500)] transition-colors cursor-pointer"
                        >
                            <ChevronDown
                                className={`h-3.5 w-3.5 transition-transform duration-200 ${expanded ? "rotate-180" : ""}`}
                            />
                            <span className="text-xs font-medium">
                                {totalLists} {totalLists === 1 ? "list" : "lists"}
                            </span>
                        </button>
                        <div className="flex items-center gap-2 min-w-0">
                            <span className="truncate" title={`${totalEntries} entries`}>
                                {totalEntries} entries
                            </span>
                            {lastUpdated && (
                                <>
                                    <span className="h-3 w-px bg-[var(--tailwind-colors-slate-600)]" />
                                    <span className="truncate hidden sm:inline" title={`Updated ${lastUpdated}`}>
                                        {lastUpdated}
                                    </span>
                                </>
                            )}
                        </div>
                    </div>
                </CardContent>
            </Card>

            {/* Expandable blocklist cards section */}
            <div
                ref={expandRef}
                className="overflow-hidden transition-[max-height,opacity] duration-300 ease-in-out"
                style={{
                    maxHeight: expanded ? `${height}px` : "0px",
                    opacity: expanded ? 1 : 0,
                }}
            >
                <div className="pt-3 pb-1">
                    {children}
                </div>
            </div>
        </div>
    );
};

export default CategoryCard;
