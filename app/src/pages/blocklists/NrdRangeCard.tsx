import { type JSX, useMemo } from "react";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { Tooltip } from "@/components/ui/tooltip";
import { ExternalLinkIcon } from "lucide-react";
import type { ModelBlocklist } from "@/api/client/api";
import { formatUpdatedRelative } from "./MainContentSection";
import {
    NRD_GROUP,
    nrdDepthFromEnabled,
    nrdDepthTargets,
} from "./nrdGroup";

interface NrdRangeCardProps {
    /** NRD blocklists present, ordered shortest→longest window. */
    items: ModelBlocklist[];
    enabledBlocklists: string[];
    /** Apply an exact target: enable some ids, disable others, in one action. */
    onApplySet: (enableIds: string[], disableIds: string[]) => void;
    updating: string | null;
    restricted?: boolean;
}

function formatEntries(n: number): string {
    if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
    if (n >= 1_000) return `${(n / 1_000).toFixed(0)}K`;
    return String(n);
}

/**
 * Single card representing the Hagezi NRD list family as a stepped
 * registration-recency range (Off · 7d · … · 35d). Each step cumulatively
 * enables the corresponding windows; the depth math lives in `nrdGroup.ts`.
 */
export default function NrdRangeCard({
    items,
    enabledBlocklists,
    onApplySet,
    updating,
    restricted = false,
}: NrdRangeCardProps): JSX.Element {
    const orderedIds = useMemo(() => items.map((bl) => bl.blocklist_id), [items]);
    const labelById = useMemo(
        () => new Map(NRD_GROUP.map((t) => [t.id, t.label])),
        [],
    );

    const depth = nrdDepthFromEnabled(orderedIds, enabledBlocklists);
    const disabled = updating !== null || restricted;

    // Aggregate entries and freshness for the currently selected depth.
    const selected = items.slice(0, depth);
    const selectedEntries = selected.reduce(
        (sum, bl) => sum + (typeof bl.entries === "number" ? bl.entries : 0),
        0,
    );
    const mostRecent = selected.reduce((latest, bl) => {
        if (!bl.last_modified) return latest;
        if (!latest) return bl.last_modified;
        return new Date(bl.last_modified) > new Date(latest)
            ? bl.last_modified
            : latest;
    }, "" as string);

    const homepage = items[0]?.homepage;

    const handleChange = (value: string) => {
        // Radix single toggle yields "" when the active item is re-pressed; ignore
        // (the explicit "Off" step is the way to disable everything).
        if (value === "") return;
        const next = Number(value);
        if (Number.isNaN(next) || next === depth) return;
        const { enable, disable } = nrdDepthTargets(orderedIds, next);
        onApplySet(enable, disable);
    };

    return (
        <Card
            data-testid="nrd-range-card"
            className="bg-transparent dark:bg-[var(--variable-collection-surface)] p-3 border border-[var(--tailwind-colors-slate-light-300)] dark:border-transparent rounded-[var(--tailwind-primitives-border-radius-rounded)] shadow-sm flex flex-col gap-3 w-full overflow-hidden"
        >
            <CardContent className="p-0 flex flex-col gap-3">
                <div className="flex items-start justify-between gap-2">
                    <div className="min-w-0">
                        <div
                            className="text-tailwind-colors-slate-50 font-semibold text-base leading-tight truncate"
                            title="Newly Registered Domains (NRD)"
                        >
                            Newly Registered Domains (NRD)
                        </div>
                        <div className="pt-1 text-xs text-[var(--tailwind-colors-slate-100)] break-words">
                            Block recently registered domains, often abused for
                            phishing, malware and scams. Choose how far back to
                            block by registration date.{" "}
                            <span className="text-[var(--tailwind-colors-amber-400,#fbbf24)]">
                                Aggressive — expect false positives; whitelist
                                critical services as needed.
                            </span>
                        </div>
                    </div>
                    {homepage && (
                        <Tooltip content={homepage} side="left" align="center">
                            <Button
                                variant="ghost"
                                size="sm"
                                className="p-1 h-auto shrink-0 text-[var(--tailwind-colors-rdns-600)]"
                                onClick={() =>
                                    window.open(
                                        homepage,
                                        "_blank",
                                        "noopener,noreferrer",
                                    )
                                }
                                aria-label={`Open ${homepage}`}
                            >
                                <ExternalLinkIcon className="h-4 w-4" />
                            </Button>
                        </Tooltip>
                    )}
                </div>

                <div className="flex flex-col gap-2">
                    <span className="text-[11px] font-medium uppercase tracking-wider text-[var(--tailwind-colors-slate-400)]">
                        Registration window
                    </span>
                    <ToggleGroup
                        type="single"
                        value={String(depth)}
                        onValueChange={handleChange}
                        disabled={disabled}
                        variant="outline"
                        className="w-full border border-[var(--tailwind-colors-slate-700)]"
                        aria-label="Newly registered domains range"
                    >
                        <ToggleGroupItem
                            value="0"
                            aria-label="Off — block no newly registered domains"
                            className="text-xs cursor-pointer disabled:cursor-not-allowed data-[state=on]:!bg-[var(--tailwind-colors-slate-700)] data-[state=on]:!text-[var(--tailwind-colors-slate-50)]"
                        >
                            Off
                        </ToggleGroupItem>
                        {items.map((bl, i) => (
                            <ToggleGroupItem
                                key={bl.blocklist_id}
                                value={String(i + 1)}
                                aria-label={`Block domains registered in the ${labelById.get(bl.blocklist_id) ?? `${i + 1}`} window`}
                                className="text-xs cursor-pointer disabled:cursor-not-allowed data-[state=on]:!bg-[var(--tailwind-colors-rdns-600)] data-[state=on]:!text-[var(--tailwind-colors-slate-50)]"
                            >
                                {labelById.get(bl.blocklist_id) ?? `T${i + 1}`}
                            </ToggleGroupItem>
                        ))}
                    </ToggleGroup>
                </div>

                <div className="flex items-center justify-between text-xs text-[var(--tailwind-colors-slate-200)]">
                    <span>
                        {depth === 0
                            ? "Not blocking newly registered domains"
                            : `Blocking domains registered in the ${NRD_GROUP[depth - 1]?.window ?? "selected window"}`}
                    </span>
                    <span className="truncate ml-2">
                        {depth === 0
                            ? "0 entries"
                            : `${formatEntries(selectedEntries)} entries${mostRecent ? ` · updated ${formatUpdatedRelative(mostRecent)}` : ""}`}
                    </span>
                </div>
            </CardContent>
        </Card>
    );
}
