import { ShieldPlus } from "lucide-react";
import { Tooltip } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import type { QuickRuleAction } from "@/components/custom-rules/QuickRuleSheet";

export const LIMITED_ACCESS_TOOLTIP = "Feature unavailable in limited access mode";

/** What a top-domain list needs to offer quick rules (P25, P26). */
export interface QuickRuleApi {
    restricted: boolean;
    /** Id of a visible or screen-reader text that gives the limited-access reason. */
    describedBy: string;
    open: (domain: string, action: QuickRuleAction, trigger: HTMLElement) => void;
    isAdded: (domain: string) => boolean;
}

/** X10: quiet at rest, takes the card's action colour on row hover or focus. */
export function QuickRuleButton({
    domain,
    action,
    api,
}: {
    domain: string;
    action: QuickRuleAction;
    api: QuickRuleApi;
}) {
    const tone =
        action === "allowlist"
            ? "group-hover:text-[var(--tailwind-colors-rdns-600)] group-focus-within:text-[var(--tailwind-colors-rdns-600)]"
            : "group-hover:text-[var(--stats-blocked)] group-focus-within:text-[var(--stats-blocked)]";
    return (
        <Tooltip content={api.restricted ? LIMITED_ACCESS_TOOLTIP : "Create a custom rule"} side="top" align="center" delay={150}>
            <span className={cn("inline-flex", api.restricted && "cursor-not-allowed")}>
                <button
                    type="button"
                    aria-label={`Create a custom rule for ${domain}`}
                    aria-describedby={api.restricted ? api.describedBy : undefined}
                    disabled={api.restricted}
                    onClick={e => api.open(domain, action, e.currentTarget)}
                    data-testid="stats-quick-rule-button"
                    className={cn(
                        "inline-flex h-11 w-11 md:h-8 md:w-8 flex-none items-center justify-center rounded-md cursor-pointer transition-colors",
                        "text-[var(--stats-axis)] disabled:opacity-40 disabled:cursor-not-allowed",
                        !api.restricted && tone,
                        "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--tailwind-colors-rdns-600)]",
                    )}
                >
                    <ShieldPlus className="size-4" aria-hidden />
                </button>
            </span>
        </Tooltip>
    );
}

export function RuleAddedTag() {
    return <span className="ml-2 text-[12px] font-normal text-[var(--stats-axis)] whitespace-nowrap">Rule added</span>;
}
