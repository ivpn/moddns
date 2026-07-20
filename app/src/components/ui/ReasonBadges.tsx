import * as React from "react";
import { Badge } from "@/components/ui/badge";
import { Tooltip } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import { formatReasons } from "@/lib/formatReasons";

interface ReasonBadgesProps {
    reasons: string[];
    blocklistNames?: Record<string, string>;
    serviceNames?: Record<string, string>;
    className?: string;
}

// Show at most this many chips inline; the remainder collapse into a "+N" chip.
const MAX_VISIBLE = 3;

/**
 * Render query-log reason tokens as human-readable chips.
 *
 * Mapping is delegated to `formatReasons` (see
 * docs/specs/logs-reason-display-behaviour.md). Overflow beyond MAX_VISIBLE
 * chips collapses into a tooltip-backed "+N" chip.
 */
export function ReasonBadges({ reasons, blocklistNames, serviceNames, className }: ReasonBadgesProps) {
    const formatted = formatReasons(reasons, blocklistNames, serviceNames);
    if (formatted.length === 0) return null;

    const visible = formatted.slice(0, MAX_VISIBLE);
    const overflow = formatted.slice(MAX_VISIBLE);

    return (
        <div className={cn("flex flex-wrap gap-1.5 min-w-0", className)}>
            {visible.map((reason, i) => (
                <Badge
                    key={`${reason.kind}-${reason.label}-${i}`}
                    variant="secondary"
                    className="max-w-full truncate"
                    data-testid="querylog-reason-badge"
                >
                    {reason.label}
                </Badge>
            ))}
            {overflow.length > 0 && (
                <Tooltip content={overflow.map((r) => r.label).join(", ")}>
                    <Badge
                        variant="secondary"
                        className="max-w-full truncate"
                        data-testid="querylog-reason-badge-overflow"
                    >
                        +{overflow.length}
                    </Badge>
                </Tooltip>
            )}
        </div>
    );
}

export default ReasonBadges;
