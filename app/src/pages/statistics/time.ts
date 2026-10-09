// Date and time formatting for the Statistics page.
//
// Source of truth: docs/specs/statistics-behaviour.md Section K (K5, K6).
// Sub-day buckets use browser local time; daily buckets are UTC days.

import type { RangeKey } from "./ranges";

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

export interface AxisTick {
    ts: number;
    label: string;
}

/** Rough rendered label widths, enough to decide how many ticks fit. */
const TICK_LABEL_PX: Record<RangeKey, number> = { "3h": 64, "6h": 64, "24h": 64, "7d": 52, "30d": 50, "3m": 50, "12m": 64 };

/** Sub-day tick steps in minutes of the local day, usual step first (K12). */
const SUBDAY_STEPS: Record<"3h" | "6h" | "24h" | "7d", number[]> = {
    "3h": [30, 15],
    "6h": [60, 30, 15],
    "24h": [180, 60],
    "7d": [1440, 360, 180, 60],
};

const utcDay = (ms: number) => new Date(ms).getUTCDay();
const utcDate = (ms: number) => new Date(ms).getUTCDate();

type DayStep = { kind: "days"; n: number } | { kind: "monday"; every: number } | { kind: "month"; every: number };

/** Smallest step first; the first one whose label count fits wins. */
const DAILY_LADDER: DayStep[] = [
    { kind: "days", n: 1 },
    { kind: "days", n: 2 },
    { kind: "days", n: 3 },
    { kind: "monday", every: 1 },
    { kind: "monday", every: 2 },
    { kind: "month", every: 1 },
    { kind: "month", every: 2 },
    { kind: "month", every: 3 },
];

function stepIndexes(tsList: number[], step: DayStep): number[] {
    const out: number[] = [];
    if (step.kind === "days") {
        for (let i = 0; i < tsList.length; i += step.n) out.push(i);
    } else if (step.kind === "monday") {
        let k = 0;
        tsList.forEach((ts, i) => {
            if (utcDay(ts) === 1 && k++ % step.every === 0) out.push(i);
        });
    } else {
        let k = 0;
        tsList.forEach((ts, i) => {
            if (utcDate(ts) === 1 && k++ % step.every === 0) out.push(i);
        });
    }
    return out;
}

/**
 * K6: ticks for the daily (UTC-day) views, chosen from the span actually shown, not from the view
 * name: retention can clamp a 3m or 12m view to a month.
 */
function dailyTicks(tsList: number[], widthPx: number): AxisTick[] {
    if (tsList.length === 0) return [];
    const crossesYear = new Date(tsList[0]).getUTCFullYear() !== new Date(tsList[tsList.length - 1]).getUTCFullYear();
    const label = (ts: number, step: DayStep, first: boolean) =>
        step.kind === "month"
            ? fmt("utcmy", { month: "short", year: "numeric", timeZone: "UTC" }).format(ts)
            : formatUtcDay(ts, crossesYear && (first || (utcDate(ts) === 1 && new Date(ts).getUTCMonth() === 0)));
    const wanted = Math.min(tsList.length, 4);
    type Pick = { step: DayStep; idx: number[] };
    let chosen: Pick | null = null;
    let previous: Pick | null = null;
    for (const step of DAILY_LADDER) {
        const idx = stepIndexes(tsList, step);
        const px = step.kind === "month" ? 64 : crossesYear ? 74 : 54;
        const fit = Math.floor((widthPx - 40) / (px + 4));
        const max = Math.max(4, Math.min(fit, widthPx < 520 ? 5 : 10));
        if (idx.length <= max) {
            chosen = idx.length < wanted && previous !== null ? previous : { step, idx };
            break;
        }
        previous = { step, idx };
    }
    chosen ??= previous;
    if (chosen === null) return [];
    const { step, idx } = chosen;
    return idx.map((i, k) => ({ ts: tsList[i], label: label(tsList[i], step, k === 0) }));
}

/**
 * K6: explicit X-axis ticks for each view, taken from the bucket timestamps `tsList`.
 * Sub-day views use the browser's local clock; daily views are UTC days. Given the chart width in
 * px, ticks are thinned to what fits, and below 520 px at least every other one is dropped.
 */
export function axisTicks(range: RangeKey, tsList: number[], widthPx?: number): AxisTick[] {
    if (range === "30d" || range === "3m" || range === "12m") return dailyTicks(tsList, widthPx ?? 1000);
    if (tsList.length === 0) return [];
    // K12: the usual step first; when it leaves fewer than three labels, the next finer one.
    const minutes = (ts: number) => new Date(ts).getHours() * 60 + new Date(ts).getMinutes();
    const label = (ts: number): string => {
        const midnight = minutes(ts) === 0;
        if (midnight && range === "7d") return fmt("wd", { weekday: "short", day: "numeric" }).format(ts);
        if (midnight && range === "24h") return fmt("md", { month: "short", day: "numeric" }).format(ts);
        return formatClock(ts);
    };
    const steps = SUBDAY_STEPS[range];
    const want = Math.min(3, tsList.length);
    let ticks: AxisTick[] = [];
    for (const step of steps) {
        ticks = tsList.filter(ts => minutes(ts) % step === 0).map(ts => ({ ts, label: label(ts) }));
        if (ticks.length >= want) break;
    }
    if (widthPx !== undefined && ticks.length > 1) {
        const fit = Math.max(1, Math.floor((widthPx - 40) / (TICK_LABEL_PX[range] + 8)));
        let step = Math.max(Math.ceil(ticks.length / fit), widthPx < 520 ? 2 : 1);
        while (step > 1 && Math.ceil(ticks.length / step) < Math.min(3, ticks.length)) step--;
        ticks = ticks.filter((_, i) => i % step === 0);
    }
    return ticks;
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
