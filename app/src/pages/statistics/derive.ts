// Pure derivations from a statistics response.
//
// Source of truth: docs/specs/statistics-behaviour.md Sections P, K, X.

import type { ModelStatisticsResponse } from "@/api/client";
import { rangeDef, type RangeKey } from "./ranges";
import { formatDateTime } from "./time";
import type { BlocklistItem } from "./panels/BlocklistsPanel";

/** X7: the rows a top list loads; the API serves at most 50, all from one cached aggregation. */
export const TOP_LIST_LIMIT = 50;

const rangeSeconds = (k: RangeKey) => rangeDef(k).seconds;

export const REASON_CLASSES = [
    { key: "blocklist", label: "Blocklists", cat: 1 },
    { key: "service", label: "Services", cat: 2 },
    { key: "custom_rule", label: "Custom rules", cat: 3 },
    { key: "default_rule", label: "Default rule", cat: 4 },
    { key: "rebinding", label: "DNS rebinding", cat: 5 },
    { key: "other", label: "Other", cat: 6 },
] as const;

export const PROTOCOLS = [
    { key: "doh", label: "DoH", cat: 1 },
    { key: "dot", label: "DoT", cat: 2 },
    { key: "doq", label: "DoQ", cat: 3 },
] as const;

export type ReasonKey = (typeof REASON_CLASSES)[number]["key"];
export type ProtocolKey = (typeof PROTOCOLS)[number]["key"];

export interface StatsPoint {
    ts: number;
    total: number;
    blocked: number;
    dnssec: number;
}

export interface StatsDevice {
    id: string;
    total: number;
    blocked: number;
}

export interface StatsData {
    enabled: boolean;
    /** When counting started: the later of enabled_at and history_deleted_at. */
    enabledAt: number | null;
    /** The raw timestamps behind `enabledAt`; the expected-first-counts time depends on which one wins. */
    turnedOnAt: number | null;
    historyDeletedAt: number | null;
    retention: string;
    fromMs: number;
    toMs: number;
    bucketSeconds: number;
    totals: { total: number; blocked: number; dnssec: number };
    series: StatsPoint[];
    reasons: Record<ReasonKey, number>;
    protocols: Record<ProtocolKey, number>;
    devices: StatsDevice[];
}

const num = (v: unknown): number => (typeof v === "number" && Number.isFinite(v) ? v : 0);
const time = (v: unknown): number | null => {
    if (typeof v !== "string") return null;
    const t = Date.parse(v);
    return Number.isNaN(t) ? null : t;
};

function latest(a: number | null, b: number | null): number | null {
    return a === null ? b : b === null ? a : Math.max(a, b);
}

/** Tolerates absent fields: an off profile answers with empty collections. */
export function normalizeStats(raw: unknown): StatsData {
    const r = (raw && typeof raw === "object" && !Array.isArray(raw) ? raw : {}) as ModelStatisticsResponse;
    const totals = r.totals ?? {};
    const reasons = {} as Record<ReasonKey, number>;
    for (const c of REASON_CLASSES) reasons[c.key] = num(r.reasons?.[c.key]);
    const protocols = {} as Record<ProtocolKey, number>;
    for (const p of PROTOCOLS) protocols[p.key] = num(r.protocols?.[p.key]);
    return {
        enabled: r.enabled === true,
        enabledAt: latest(time(r.enabled_at), time(r.history_deleted_at)),
        turnedOnAt: time(r.enabled_at),
        historyDeletedAt: time(r.history_deleted_at),
        retention: typeof r.retention === "string" ? r.retention : "",
        fromMs: time(r.from) ?? 0,
        toMs: time(r.to) ?? 0,
        bucketSeconds: num(r.bucket_seconds),
        totals: { total: num(totals.total), blocked: num(totals.blocked), dnssec: num(totals.dnssec) },
        series: (Array.isArray(r.series) ? r.series : []).flatMap(p => {
            const ts = time(p.ts);
            return ts === null ? [] : [{ ts, total: num(p.total), blocked: num(p.blocked), dnssec: num(p.dnssec) }];
        }),
        reasons,
        protocols,
        devices: (Array.isArray(r.devices) ? r.devices : []).map(d => ({
            id: d.device_id ?? "",
            total: num(d.total),
            blocked: num(d.blocked),
        })),
    };
}

/** The proxy flushes closed 15-minute buckets only (proxy-statistics-behaviour.md). */
export const FLUSH_LAG_MS = 15 * 60 * 1000;

export interface Bucket {
    ts: number;
    /** null before counting started (P9): a gap, not a zero. */
    total: number | null;
    blocked: number | null;
    dnssec: number | null;
    /** P10: the bucket has not been fully flushed yet. */
    inProgress: boolean;
}

/** P9: points before the bucket containing the start of counting are gaps (null), not zeros. */
export function buildBuckets(d: StatsData): Bucket[] {
    const width = d.bucketSeconds * 1000;
    const startFloor = d.enabledAt !== null && width > 0 ? Math.floor(d.enabledAt / width) * width : null;
    return d.series.map(p => {
        const gap = startFloor !== null && p.ts < startFloor;
        return {
            ts: p.ts,
            total: gap ? null : p.total,
            blocked: gap ? null : p.blocked,
            dnssec: gap ? null : p.dnssec,
            inProgress: !gap && p.ts + width > d.toMs - FLUSH_LAG_MS,
        };
    });
}

/**
 * P9: the shortest offered view that still contains the start of counting, when the counted part is
 * under a quarter of the current view and a shorter view exists; else null.
 */
export function shorterViewForStart(d: StatsData, current: RangeKey, offered: { key: RangeKey; seconds: number }[]): RangeKey | null {
    const since = countingSince(d);
    if (since === null) return null;
    const counted = d.toMs - since;
    const viewSpan = rangeSeconds(current) * 1000;
    if (counted >= viewSpan / 4) return null;
    const fit = offered.find(r => r.seconds * 1000 >= counted);
    return fit && fit.seconds * 1000 < viewSpan ? fit.key : null;
}

/** P9: the first bucket that counts, when statistics were enabled inside the view. */
export function countingSince(d: StatsData): number | null {
    if (d.enabledAt === null || d.enabledAt <= d.fromMs) return null;
    const first = d.series[0]?.ts;
    return first !== undefined && d.enabledAt > first ? d.enabledAt : null;
}

export type CountsState = "data" | "no-queries-yet" | "empty-range";

/** P5 / P6: `total = 0` means one card for the whole Counts group. */
export function countsState(d: StatsData): CountsState {
    if (d.totals.total > 0) return "data";
    return d.enabledAt !== null && d.enabledAt >= d.fromMs ? "no-queries-yet" : "empty-range";
}

export function deviceLabel(id: string): { kind: "id" | "none" | "other"; text: string; hint?: string } {
    if (id === "") return { kind: "none", text: "No device ID", hint: "Queries that arrived without a device identifier" };
    if (id === "_other") return { kind: "other", text: "Other devices", hint: "Devices beyond the 99 busiest in this range, grouped" };
    return { kind: "id", text: id };
}

export function sum(values: (number | null)[]): number {
    return values.reduce<number>((s, v) => s + (v ?? 0), 0);
}

const QUARTER_MS = 15 * 60_000;
const HOUR_MS = 60 * 60_000;
const DAY_MS = 24 * HOUR_MS;
const FLUSH_TICK_MS = 60_000;
const COLLECTING_GRACE_MS = 2 * 60_000;

export interface ExpectedFirstCounts {
    /** When the first bucket of the view closes. */
    boundaryMs: number;
    /** What the page says ("about …"): the boundary plus the proxy's 60 s flush tick. */
    displayMs: number;
    /** P5 shows until this moment; after it an empty window is P23. */
    collectingUntilMs: number;
}

/**
 * P24: when the first counts can appear in `range`'s bucket tier (Y15, Y17, Y28, J53).
 * After turning on, every view waits for the next quarter-hour boundary; after Delete history the
 * hourly and daily tiers also wait for their own bucket to close.
 */
export function expectedFirstCounts(range: RangeKey, enabledAt: number | null, historyDeletedAt: number | null): ExpectedFirstCounts | null {
    const start = latest(enabledAt, historyDeletedAt);
    if (start === null) return null;
    const afterDelete = historyDeletedAt !== null && (enabledAt === null || historyDeletedAt > enabledAt);
    const nextQuarter = Math.floor(start / QUARTER_MS) * QUARTER_MS + QUARTER_MS;
    let boundaryMs = nextQuarter;
    if (afterDelete) {
        if (range === "24h" || range === "7d") boundaryMs = Math.floor(start / HOUR_MS) * HOUR_MS + HOUR_MS + QUARTER_MS;
        else if (range === "30d" || range === "3m" || range === "12m") boundaryMs = Math.floor(start / DAY_MS) * DAY_MS + DAY_MS + QUARTER_MS;
    }
    const displayMs = boundaryMs + FLUSH_TICK_MS;
    return { boundaryMs, displayMs, collectingUntilMs: displayMs + COLLECTING_GRACE_MS };
}

/** P18: the window a group's numbers cover. */
export type Scope = { kind: "last"; words: string } | { kind: "since"; at: number };

const LOGS_RETENTION_WINDOW: Record<string, { seconds: number; words: string }> = {
    "1h": { seconds: 3600, words: "last hour" },
    "6h": { seconds: 6 * 3600, words: "last 6 hours" },
    "1d": { seconds: 86400, words: "last 24 hours" },
    "1w": { seconds: 7 * 86400, words: "last 7 days" },
    "1m": { seconds: 30 * 86400, words: "last 30 days" },
};

/** P18: the shorter of the view and the logs retention; an unknown retention reads as 1 hour. */
function logsWindow(range: RangeKey, logsRetention: string | undefined): { seconds: number; words: string } {
    const kept = LOGS_RETENTION_WINDOW[logsRetention ?? ""] ?? LOGS_RETENTION_WINDOW["1h"];
    const view = rangeDef(range);
    return kept.seconds < view.seconds ? kept : { seconds: view.seconds, words: view.words };
}

/** P18: how far a start may sit inside the logs window before the scope says "since". */
const toleranceMs = (windowSeconds: number) => (windowSeconds * 1000) / 12;

/** P18: the logs group scope; `coveredFrom` is the oldest log row in the window (J20). */
export function logsScope(range: RangeKey, logsRetention: string | undefined, coveredFrom: number | null, nowMs: number): Scope {
    const w = logsWindow(range, logsRetention);
    const start = nowMs - w.seconds * 1000;
    return coveredFrom !== null && coveredFrom > start + toleranceMs(w.seconds) ? { kind: "since", at: coveredFrom } : { kind: "last", words: w.words };
}

/** P28: the logs start before statistics started counting inside the view. */
export function logsReachBeforeCounts(range: RangeKey, logsRetention: string | undefined, coveredFrom: number | null, countsStart: number | null): boolean {
    if (coveredFrom === null || countsStart === null) return false;
    return coveredFrom < countsStart - toleranceMs(logsWindow(range, logsRetention).seconds);
}

export function scopeTitle(s: Scope): string {
    return s.kind === "last" ? s.words.charAt(0).toUpperCase() + s.words.slice(1) : `Since ${formatDateTime(s.at)}`;
}

/** P27: the scope inside a sentence. */
export function scopeIn(s: Scope): string {
    return s.kind === "last" ? `in the ${s.words}` : `since ${formatDateTime(s.at)}`;
}

/**
 * P22: appends the enabled lists the logs never credited, as 0, sorted by name. Skipped when the
 * hits filled the top-list limit, where a missing list may simply rank lower.
 */
export function withEnabledLists(items: BlocklistItem[], enabled: string[], names: Map<string, string>): BlocklistItem[] {
    if (items.length >= TOP_LIST_LIMIT) return items;
    const credited = new Set(items.map(b => b.id));
    const zero = enabled
        .filter(id => !credited.has(id))
        .map(id => ({ id, name: names.get(id), count: 0 }))
        .sort((a, b) => (a.name ?? a.id).localeCompare(b.name ?? b.id));
    return zero.length ? [...items, ...zero] : items;
}
