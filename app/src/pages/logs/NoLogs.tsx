import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { ListX } from "lucide-react";
import React, { type JSX } from "react";
import { useNavigate } from "react-router-dom";

interface NoLogsProps {
    isSearchActive?: boolean;
    /** A non-default status/device/sort/timespan filter is applied. */
    hasActiveFilters?: boolean;
    /** Resets every filter and the search; enables the "Clear filters" action. */
    onClearFilters?: () => void;
}

interface EmptyStateContent {
    title: string;
    description?: string;
    buttonText?: string;
}

const emptyStateVariants: Record<"default" | "search" | "filters", EmptyStateContent> = {
    default: {
        title: "No logs to display",
        description: "Set up modDNS on your devices to start analysing queries.",
        buttonText: "DNS Setup",
    },
    search: {
        title: "No matching logs",
        description: "No logs match your search. Try updating the keywords or filters.",
    },
    filters: {
        title: "No matching logs",
        description: "No results for the current filters — try clearing them.",
    },
};

const NoLogs = ({ isSearchActive = false, hasActiveFilters = false, onClearFilters }: NoLogsProps): JSX.Element => {
    const navigate = useNavigate();
    // The DNS-setup onboarding CTA is only correct when the list is empty with NOTHING
    // narrowing it — an empty filtered view means "no matches", not "not set up yet".
    const isFiltered = isSearchActive || hasActiveFilters;
    const emptyStateData = isSearchActive
        ? emptyStateVariants.search
        : hasActiveFilters
            ? emptyStateVariants.filters
            : emptyStateVariants.default;

    return (
        <Card className="flex flex-col relative w-full bg-transparent dark:bg-[var(--variable-collection-surface)] rounded-lg overflow-hidden !border-0 !shadow-none !outline-none !ring-0
            items-center md:items-start mx-auto md:mx-0 max-w-[560px] md:max-w-full flex-1 self-stretch grow">
            <div className="flex flex-col h-auto md:h-[652px] items-center md:items-start gap-4 md:gap-8 p-4 pt-2 md:pt-4 relative w-full self-stretch">
                <div className="flex flex-col items-center justify-start md:justify-center gap-3 md:gap-2.5 relative w-full md:flex-1 md:grow mt-10 md:mt-0">
                    {/* Icon container */}
                    <div className="flex w-12 h-12 items-center justify-center gap-2.5 relative rounded-sm">
                        <ListX className="!relative !w-9 !h-9 text-[var(--tailwind-colors-rdns-600)]" />
                    </div>

                    {/* Text content */}
                    <div className="flex flex-col items-center justify-center gap-2 relative w-full max-w-[360px] md:max-w-sm px-1.5 md:px-0">
                        <h3 className="text-lg font-semibold text-[var(--tailwind-colors-slate-50)] text-center leading-6 md:leading-7">
                            {emptyStateData.title}
                        </h3>
                        {emptyStateData.description && (
                            <p className="px-2 md:p-4 text-sm text-[var(--tailwind-colors-slate-100)] text-center font-normal font-['Roboto_Flex-Regular',Helvetica] leading-5">
                                {emptyStateData.description}
                            </p>
                        )}
                    </div>

                    {/* Action button */}
                    {isFiltered && onClearFilters && (
                        <Button
                            variant="cancel"
                            onClick={onClearFilters}
                            data-testid="logs-empty-clear-filters"
                            className="h-9 min-h-11 md:h-auto rounded-md px-6 py-2"
                        >
                            <span className="text-sm md:text-xs font-medium whitespace-nowrap">Clear filters</span>
                        </Button>
                    )}
                    {!isFiltered && emptyStateData.buttonText && (
                        <Button
                            className="h-9 min-h-11 md:h-auto bg-[var(--tailwind-colors-rdns-600)] text-[var(--tailwind-colors-slate-900)] rounded-md px-6 py-2 md:px-6 md:py-2 cursor-pointer transition-colors"
                            style={{
                                '--hover-bg': 'var(--tailwind-colors-rdns-800)',
                            } as React.CSSProperties}
                            onMouseEnter={e => (e.currentTarget.style.background = 'var(--tailwind-colors-rdns-800)')}
                            onMouseLeave={e => (e.currentTarget.style.background = 'var(--tailwind-colors-rdns-600)')}
                            onClick={() => navigate("/setup")}
                        >
                            <span className="text-sm md:text-xs font-medium whitespace-nowrap">{emptyStateData.buttonText}</span>
                        </Button>
                    )}
                </div>
            </div>
        </Card>
    );
};

export default NoLogs;
