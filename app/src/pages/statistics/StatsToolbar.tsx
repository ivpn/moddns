import { cn } from "@/lib/utils";
import { RANGES, type RangeKey } from "./ranges";
import { mutedText } from "./primitives";

/** K1: a segmented control at every width; segments are at least 44 px tall and wide. */
export function RangePicker({ value, onChange, disabled }: { value: RangeKey; onChange: (k: RangeKey) => void; disabled?: boolean }) {
    return (
        <div role="radiogroup" aria-label="Time range" className="grid grid-cols-7 gap-1 w-full sm:w-auto sm:inline-grid">
            {RANGES.map(r => (
                <label
                    key={r.key}
                    className={cn(
                        "flex items-center justify-center min-h-11 min-w-11 sm:px-3 lg:min-h-9 rounded border text-sm font-medium cursor-pointer select-none tabular-nums",
                        "has-[:focus-visible]:ring-2 has-[:focus-visible]:ring-[var(--tailwind-colors-rdns-600)]",
                        value === r.key
                            ? "bg-[var(--tailwind-colors-rdns-600)] border-[var(--tailwind-colors-rdns-600)] text-white"
                            : "border-[var(--tailwind-colors-slate-600)] text-[var(--tailwind-colors-slate-50)]",
                    )}
                >
                    <input
                        type="radio"
                        className="sr-only"
                        name="stats-range"
                        value={r.key}
                        checked={value === r.key}
                        disabled={disabled}
                        aria-label={r.label}
                        onChange={() => onChange(r.key)}
                    />
                    <span aria-hidden>{r.key}</span>
                </label>
            ))}
        </div>
    );
}

export function StatsToolbar({
    range,
    onRange,
    caption,
    trailing,
}: {
    range: RangeKey;
    onRange: (k: RangeKey) => void;
    caption: string | null;
    /** Reserved for the retention control (#713). */
    trailing?: React.ReactNode;
}) {
    return (
        <div className="flex flex-col gap-2">
            <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
                <RangePicker value={range} onChange={onRange} />
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
