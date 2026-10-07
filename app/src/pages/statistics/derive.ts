// Pure derivations from a statistics response.
//
// Source of truth: docs/specs/statistics-behaviour.md Sections P, K, X.

import type { ModelStatisticsResponse } from "@/api/client";
import { rangeDef, type RangeKey } from "./ranges";

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
    enabledAt: number | null;
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
        enabledAt: time(r.enabled_at),
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
    /** null before statistics were enabled (P9): a gap, not a zero. */
    total: number | null;
    blocked: number | null;
    dnssec: number | null;
    /** P10: the bucket has not been fully flushed yet. */
    inProgress: boolean;
}

export function buildBuckets(d: StatsData): Bucket[] {
    const width = d.bucketSeconds * 1000;
    const enabledFloor = d.enabledAt !== null && width > 0 ? Math.floor(d.enabledAt / width) * width : null;
    return d.series.map(p => {
        const gap = enabledFloor !== null && p.ts < enabledFloor;
        return {
            ts: p.ts,
            total: gap ? null : p.total,
            blocked: gap ? null : p.blocked,
            dnssec: gap ? null : p.dnssec,
            inProgress: !gap && p.ts + width > d.toMs - FLUSH_LAG_MS,
        };
    });
}

/** P9: the first bucket that counts, when statistics were enabled inside the view. */
export function countingSince(d: StatsData): number | null {
    if (d.enabledAt === null || d.enabledAt <= d.fromMs) return null;
    const first = d.series[0]?.ts;
    return first !== undefined && d.enabledAt > first ? d.enabledAt : null;
}

/** P11: the view is shorter than requested because retention clamps `from`. */
export function isClamped(d: StatsData, range: RangeKey): boolean {
    if (d.toMs <= d.fromMs) return false;
    return (d.toMs - d.fromMs) / 1000 < rangeDef(range).seconds - d.bucketSeconds;
}

export type CountsState = "data" | "no-queries-yet" | "empty-range";

/** P5 / P6: `total = 0` means one card for the whole Counts group. */
export function countsState(d: StatsData): CountsState {
    if (d.totals.total > 0) return "data";
    return d.enabledAt !== null && d.enabledAt >= d.fromMs ? "no-queries-yet" : "empty-range";
}

const RETENTION_WORDS: Record<string, string> = { "30d": "30 days", "90d": "90 days", "1y": "1 year" };

/** An empty or unknown retention reads as 30 days (api-endpoint-behaviour.md J41). */
export function statsRetentionWords(retention: string | undefined): string {
    return RETENTION_WORDS[retention ?? ""] ?? RETENTION_WORDS["30d"];
}

export function deviceLabel(id: string): { kind: "id" | "none" | "other"; text: string; hint?: string } {
    if (id === "") return { kind: "none", text: "No device ID", hint: "Queries that arrived without a device identifier" };
    if (id === "_other") return { kind: "other", text: "Other devices", hint: "Devices beyond the 99 busiest in this range, grouped" };
    return { kind: "id", text: id };
}

export function sum(values: (number | null)[]): number {
    return values.reduce<number>((s, v) => s + (v ?? 0), 0);
}
