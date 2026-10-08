import { cn } from "@/lib/utils";
import ToggleGroup from "@/components/general/ToggleGroup";
import type { RangeDef, RangeKey } from "./ranges";
import { mutedText } from "./primitives";

/** K1: pills at every width; segments are at least 44 px tall and wide on touch. */
export function RangePicker({ ranges, value, onChange, disabled }: { ranges: RangeDef[]; value: RangeKey; onChange: (k: RangeKey) => void; disabled?: boolean }) {
    return (
        <ToggleGroup
            ariaLabel="Time range"
            options={ranges.map(r => ({ value: r.key, label: r.key, ariaLabel: r.label }))}
            value={value}
            // Radix reports "" when the selected pill is pressed again.
            onChange={v => v && onChange(v as RangeKey)}
            variant="outline"
            disabled={disabled}
            className="!w-full sm:!w-auto self-start"
            itemClassName="flex-1 sm:flex-initial !h-11 sm:!h-9 !min-w-11 sm:!min-w-[52px] px-1 sm:px-3 tabular-nums [&_span]:!font-bold"
        />
    );
}

export function StatsToolbar({
    ranges,
    range,
    onRange,
    caption,
    trailing,
}: {
    ranges: RangeDef[];
    range: RangeKey;
    onRange: (k: RangeKey) => void;
    caption: string | null;
    /** Reserved for the retention control (#713). */
    trailing?: React.ReactNode;
}) {
    return (
        <div className="flex flex-col gap-2">
            <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
                <RangePicker ranges={ranges} value={range} onChange={onRange} />
                {trailing}
            </div>
            {caption && (
                <p className={cn("text-[13px] leading-[18px] tabular-nums", mutedText)} data-testid="stats-range-caption">
                    {caption}
                </p>
            )}
        </div>
    );
}
