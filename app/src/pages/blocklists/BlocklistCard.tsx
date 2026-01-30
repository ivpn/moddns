import React from "react";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { ExternalLinkIcon } from "lucide-react";
import { Tooltip } from "@/components/ui/tooltip";

interface BlocklistCardProps {
    title: string;
    description: string;
    entries: string;
    updated: string;
    onSwitchChange?: (checked: boolean) => void;
    switchChecked?: boolean;
    switchDisabled?: boolean;
    homepage?: string;
}

const BlocklistCard: React.FC<BlocklistCardProps> = ({
    title,
    description,
    entries,
    updated,
    onSwitchChange,
    switchChecked,
    switchDisabled,
    homepage,
}) => {
    return (
        <Card data-testid="blocklist-card" className="bg-transparent dark:bg-[var(--variable-collection-surface)] p-3 border border-[var(--tailwind-colors-slate-light-300)] dark:border-transparent rounded-[var(--tailwind-primitives-border-radius-rounded)] shadow-sm flex flex-col justify-between h-[196px] lg:h-[180px] w-full overflow-hidden">
            <CardContent className="p-0 flex flex-col justify-between h-full">
                <div className="flex flex-col gap-1">
                    <div className="flex items-start justify-between gap-2">
                        <div className="text-tailwind-colors-slate-50 font-semibold text-base leading-tight max-w-[70%] md:max-w-[75%] lg:max-w-[80%] truncate break-words">
                            {title}
                        </div>
                        <Switch
                            checked={switchChecked}
                            onCheckedChange={onSwitchChange}
                            disabled={switchDisabled}
                            className="w-9 h-5
                            data-[state=unchecked]:bg-[var(--tailwind-colors-slate-700)]
                            data-[state=checked]:bg-[var(--tailwind-colors-rdns-600)]
                            [&>[data-slot=switch-thumb]]:bg-background
                            data-[state=checked]:[&>[data-slot=switch-thumb]]:bg-[var(--tailwind-colors-slate-50)]
                            data-[state=unchecked]:[&>[data-slot=switch-thumb]]:bg-[var(--tailwind-colors-slate-400)]
                            data-[state=checked]:[&>[data-slot=switch-thumb]]:translate-x-4"
                        />
                    </div>
                    <div className="pt-2 font-text-xs-leading-5-normal text-[var(--tailwind-colors-slate-100)] text-xs h-[72px] overflow-hidden text-ellipsis [display:-webkit-box] [-webkit-line-clamp:3] [-webkit-box-orient:vertical] break-words hyphens-auto">
                        {description}
                    </div>
                </div>
                <div className="mt-4 flex flex-col gap-1">
                    {/* First row: entries (and updated inline only on >= xl); homepage button hidden here below xl */}
                    <div className="flex items-center justify-end gap-2 xl:justify-between">
                        <div className="hidden xl:flex items-center flex-shrink-0">
                            <Tooltip content={homepage || "Open homepage"} maxWidthClassName="max-w-[280px] md:max-w-[320px]" side="right" align="center" shiftY={-25}>
                                <Button
                                    variant="ghost"
                                    size="sm"
                                    className="p-1 h-auto text-[var(--tailwind-colors-rdns-600)]"
                                    onClick={() => {
                                        if (homepage)
                                            window.open(homepage, "_blank", "noopener,noreferrer");
                                    }}
                                    aria-label={homepage ? `Open ${homepage}` : "Open homepage"}
                                >
                                    <ExternalLinkIcon className="h-4 w-4" />
                                </Button>
                            </Tooltip>
                        </div>
                        <div className="flex items-center text-xs text-[var(--tailwind-colors-slate-200)] min-w-0 flex-1 justify-end gap-2">
                            <span className="truncate translate-y-[6px] xl:translate-y-0" title={`${entries} entries`}>{entries} entries</span>
                            <div className="hidden xl:block h-3 w-px bg-[var(--tailwind-colors-slate-400)]" />
                            <span className="hidden xl:inline truncate" title={`Updated ${updated}`}>Updated {updated}</span>
                        </div>
                    </div>
                    {/* Second row: homepage button + Updated for mobile/tablet (below xl) */}
                    <div className="flex items-center justify-between xl:hidden text-xs text-[var(--tailwind-colors-slate-200)] leading-tight">
                        <div className="flex items-center">
                            <Tooltip content={homepage || "Open homepage"} maxWidthClassName="max-w-[240px] md:max-w-[300px]" side="top" align="start">
                                <Button
                                    variant="ghost"
                                    size="sm"
                                    className="p-1 h-auto text-[var(--tailwind-colors-rdns-600)]"
                                    onClick={() => {
                                        if (homepage)
                                            window.open(homepage, "_blank", "noopener,noreferrer");
                                    }}
                                    aria-label={homepage ? `Open ${homepage}` : "Open homepage"}
                                >
                                    <ExternalLinkIcon className="h-4 w-4" />
                                </Button>
                            </Tooltip>
                        </div>
                        <span className="truncate ml-2" title={`Updated ${updated}`}>Updated {updated}</span>
                    </div>
                </div>
            </CardContent>
        </Card>
    );
};

export default BlocklistCard;
