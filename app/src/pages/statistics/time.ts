// Date and time formatting for the Statistics page.
//
// Source of truth: docs/specs/statistics-behaviour.md Section K (K5, K6).
// Sub-day buckets use browser local time; daily buckets are UTC days.

const cache = new Map<string, Intl.DateTimeFormat>();

function fmt(key: string, opts: Intl.DateTimeFormatOptions): Intl.DateTimeFormat {
    const k = `${key}|${JSON.stringify(opts)}`;
    let f = cache.get(k);
    if (!f) {
        f = new Intl.DateTimeFormat(undefined, opts);
        cache.set(k, f);
    }
    return f;
}

const HM: Intl.DateTimeFormatOptions = { hour: "2-digit", minute: "2-digit" };

export function formatDateTime(ms: number): string {
    return fmt("dt", { month: "short", day: "numeric", ...HM }).format(ms);
}

export function formatClock(ms: number): string {
    return fmt("hm", HM).format(ms);
}

/** "about {expected}" (P5): the clock time today, the date and time on another day. */
export function formatExpected(ms: number, nowMs: number): string {
    return new Date(ms).toDateString() === new Date(nowMs).toDateString() ? formatClock(ms) : formatDateTime(ms);
}

export function formatUtcDay(ms: number, withYear = false): string {
    return fmt("utcday", { month: "short", day: "numeric", year: withYear ? "numeric" : undefined, timeZone: "UTC" }).format(ms);
}

/** K6: only worth saying "Days are UTC days" when local time differs from UTC. */
export function browserOffsetsFromUtc(): boolean {
    return new Date().getTimezoneOffset() !== 0;
}

export function bucketUnitWord(bucketSeconds: number): string {
    if (bucketSeconds >= 86400) return "daily";
    if (bucketSeconds >= 3600) return "hourly";
    return "15-minute";
}

/** K5: "{from} - {to} · hourly" */
export function formatRangeCaption(fromMs: number, toMs: number, bucketSeconds: number): string {
    const daily = bucketSeconds >= 86400;
    const range = daily
        ? `${formatUtcDay(fromMs)} - ${formatUtcDay(toMs)}`
        : `${formatDateTime(fromMs)} - ${formatDateTime(toMs)}`;
    const days = daily && browserOffsetsFromUtc() ? " · Days are UTC days" : "";
    return `${range} · ${bucketUnitWord(bucketSeconds)}${days}`;
}

function isLocalMidnight(ms: number): boolean {
    const d = new Date(ms);
    return d.getHours() === 0 && d.getMinutes() === 0;
}

/** Axis tick label, driven by the bucket width. */
export function formatBucketTick(ms: number, bucketSeconds: number, longYear = false): string {
    if (bucketSeconds >= 86400) {
        return longYear
            ? fmt("utcmy", { month: "short", year: "2-digit", timeZone: "UTC" }).format(ms)
            : formatUtcDay(ms);
    }
    if (bucketSeconds >= 3600 && isLocalMidnight(ms)) {
        return fmt("wd", { weekday: "short", day: "numeric" }).format(ms);
    }
    return formatClock(ms);
}

/** Tooltip and table row header for one bucket. */
export function formatBucketLabel(ms: number, bucketSeconds: number): string {
    if (bucketSeconds >= 86400) return `${formatUtcDay(ms, true)} (UTC day)`;
    return `${formatDateTime(ms)}-${formatClock(ms + bucketSeconds * 1000)}`;
}

export function formatRelative(ms: number, nowMs = Date.now()): string {
    const diffSec = Math.round((ms - nowMs) / 1000);
    const rtf = new Intl.RelativeTimeFormat(undefined, { numeric: "auto" });
    const abs = Math.abs(diffSec);
    if (abs < 60) return rtf.format(diffSec, "second");
    if (abs < 3600) return rtf.format(Math.round(diffSec / 60), "minute");
    if (abs < 86400) return rtf.format(Math.round(diffSec / 3600), "hour");
    return rtf.format(Math.round(diffSec / 86400), "day");
}
