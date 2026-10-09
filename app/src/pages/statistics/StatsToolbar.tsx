import { useId, useRef } from "react";
import { ChevronDown, EllipsisVertical, Trash2 } from "lucide-react";
import type { ModelProfile } from "@/api/client";
import { cn } from "@/lib/utils";
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuRadioGroup,
    DropdownMenuRadioItem,
    DropdownMenuSeparator,
    DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useDeleteStatisticsHistory } from "@/components/data-collection/DeleteStatisticsHistory";
import { STALE_TEXT, STATS_RETENTION_OPTIONS } from "@/components/data-collection/model";
import { useSubscriptionGuard } from "@/hooks/useSubscriptionGuard";
import { RANGES, type RangeDef, type RangeKey } from "./ranges";
import { LimitedAccessNote, mutedText, titleText } from "./primitives";

const DAY = 86400;
const trigger = cn(
    "inline-flex items-center justify-center rounded-md border border-[var(--tailwind-colors-slate-light-300)] dark:border-[var(--tailwind-colors-slate-600)] bg-[var(--shadcn-ui-app-background)] hover:bg-muted min-h-11 md:min-h-9 disabled:opacity-60 disabled:cursor-not-allowed",
    titleText,
    "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--tailwind-colors-rdns-600)]",
);

/** K11: the shortest retention that covers a view. */
function neededRetention(r: RangeDef): string {
    return (STATS_RETENTION_OPTIONS.find(o => o.days * DAY >= r.seconds) ?? STATS_RETENTION_OPTIONS[STATS_RETENTION_OPTIONS.length - 1]).words;
}

/** K1, K11: a dropdown of all seven views; the ones the retention doesn't cover are disabled with the reason. */
export function RangePicker({
    offered,
    value,
    onChange,
    onChangeRetention,
}: {
    offered: RangeDef[];
    value: RangeKey;
    onChange: (k: RangeKey) => void;
    /** Receives the range trigger, where focus returns when the dialog closes. */
    onChangeRetention: (trigger: HTMLElement | null) => void;
}) {
    // Opened only after the menu has closed, so the menu's focus handling can't pull focus out of the dialog.
    const retentionRequested = useRef(false);
    const triggerRef = useRef<HTMLButtonElement>(null);
    const current = RANGES.find(r => r.key === value) ?? RANGES[3];
    const isOffered = (k: RangeKey) => offered.some(r => r.key === k);
    const anyDisabled = RANGES.some(r => !isOffered(r.key));
    return (
        <DropdownMenu>
            <DropdownMenuTrigger asChild>
                <button
                    ref={triggerRef}
                    type="button"
                    aria-label={`Time range: ${current.label}`}
                    data-testid="stats-range-trigger"
                    className={cn(trigger, "justify-between gap-2 px-3 text-sm font-medium flex-1 sm:flex-none sm:min-w-44")}
                >
                    {current.label}
                    <ChevronDown className="w-4 h-4 opacity-70" aria-hidden />
                </button>
            </DropdownMenuTrigger>
            <DropdownMenuContent
                align="end"
                className="min-w-[var(--radix-dropdown-menu-trigger-width)] sm:min-w-60"
                onCloseAutoFocus={() => {
                    if (!retentionRequested.current) return;
                    retentionRequested.current = false;
                    onChangeRetention(triggerRef.current);
                }}
            >
                <DropdownMenuRadioGroup value={value} onValueChange={v => onChange(v as RangeKey)}>
                    {RANGES.map(r =>
                        isOffered(r.key) ? (
                            <DropdownMenuRadioItem key={r.key} value={r.key} className="min-h-11 md:min-h-8">
                                {r.label}
                            </DropdownMenuRadioItem>
                        ) : (
                            <DropdownMenuRadioItem key={r.key} value={r.key} disabled className="min-h-11 md:min-h-8 flex-col items-start gap-0">
                                <span>{r.label}</span>
                                <span className="text-xs">{`Needs statistics kept for ${neededRetention(r)}`}</span>
                            </DropdownMenuRadioItem>
                        ),
                    )}
                </DropdownMenuRadioGroup>
                {anyDisabled && (
                    <>
                        <DropdownMenuSeparator />
                        <DropdownMenuItem
                            inset
                            aria-haspopup="dialog"
                            onSelect={() => (retentionRequested.current = true)}
                            className="min-h-11 md:min-h-8 text-[var(--tailwind-colors-rdns-600)] focus:text-[var(--tailwind-colors-rdns-600)]"
                        >
                            Change retention…
                        </DropdownMenuItem>
                    </>
                )}
            </DropdownMenuContent>
        </DropdownMenu>
    );
}

/** S3: the ⋮ menu with "Delete statistics history". */
export function StatsActions({ profile, onHistoryDeleted }: { profile: ModelProfile; onHistoryDeleted: () => void }) {
    const { isRestricted } = useSubscriptionGuard();
    const noteId = useId();
    const menuRef = useRef<HTMLButtonElement>(null);
    const history = useDeleteStatisticsHistory(profile, {
        onDeleted: onHistoryDeleted,
        onClosed: deleted => !deleted && requestAnimationFrame(() => menuRef.current?.focus()),
    });
    return (
        <>
            <DropdownMenu>
                <DropdownMenuTrigger asChild>
                    <button
                        type="button"
                        ref={menuRef}
                        aria-label="More statistics actions"
                        aria-describedby={isRestricted ? noteId : undefined}
                        disabled={isRestricted || history.busy}
                        className={cn(trigger, "flex-none min-w-11 md:min-w-9")}
                    >
                        <EllipsisVertical className="w-4 h-4" aria-hidden />
                    </button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                    <DropdownMenuItem variant="destructive" onSelect={() => void history.request()} className="min-h-11 lg:min-h-8">
                        <Trash2 className="w-4 h-4" aria-hidden />
                        Delete statistics history
                    </DropdownMenuItem>
                </DropdownMenuContent>
            </DropdownMenu>
            {history.dialog}
            {isRestricted && (
                <div className="basis-full">
                    <LimitedAccessNote id={noteId} />
                </div>
            )}
            {history.stale && (
                <p role="status" className={cn("basis-full text-[13px] leading-[18px] sm:text-right", mutedText)}>
                    {STALE_TEXT}
                </p>
            )}
        </>
    );
}

/** K1, K5: availability line on the left; time range and the ⋮ menu on the right with the caption under them. */
export function StatsToolbar({
    offered,
    range,
    onRange,
    caption,
    leading,
    trailing,
    onChangeRetention,
}: {
    offered: RangeDef[];
    range: RangeKey;
    onRange: (k: RangeKey) => void;
    /** Receives the range trigger, where focus returns when the dialog closes. */
    onChangeRetention: (trigger: HTMLElement | null) => void;
    caption: string | null;
    leading?: React.ReactNode;
    trailing?: React.ReactNode;
}) {
    return (
        <div className="flex flex-col sm:flex-row sm:items-start sm:justify-between gap-3 w-full">
            <div className="min-w-0 flex-1 sm:pt-2">{leading}</div>
            <div className="flex flex-col gap-2 w-full sm:w-auto sm:items-end">
                <div className="flex flex-wrap items-center gap-2 w-full sm:w-auto sm:justify-end">
                    <RangePicker offered={offered} value={range} onChange={onRange} onChangeRetention={onChangeRetention} />
                    {trailing}
                </div>
                {caption && (
                    <p className={cn("text-[13px] leading-[18px] tabular-nums sm:text-right", mutedText)} data-testid="stats-range-caption">
                        {caption}
                    </p>
                )}
            </div>
        </div>
    );
}
