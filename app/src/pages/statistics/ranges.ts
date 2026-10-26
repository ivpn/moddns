// Timeframe picker model.
//
// Source of truth: docs/specs/statistics-behaviour.md Section K.

import { STATS_RETENTION_OPTIONS, type StatsRetention } from "@/components/data-collection/model";
import type {
    ApiV1ProfilesIdLogsTopBlocklistsGetTimespanEnum,
    ApiV1ProfilesIdLogsTopClientsGetTimespanEnum,
    ApiV1ProfilesIdLogsTopDomainsGetTimespanEnum,
    ApiV1ProfilesIdStatisticsGetTimespanEnum as StatsTimespan,
} from "@/api/client";

export type RangeKey = "3h" | "6h" | "24h" | "7d" | "30d" | "3m" | "12m";

export const DEFAULT_RANGE: RangeKey = "24h";

export interface RangeDef {
    key: RangeKey;
    /** Accessible name of the segment. */
    label: string;
    /** For sentences: "No queries in the last 3 hours." */
    words: string;
    seconds: number;
}

const DAY = 86400;

export const RANGES: RangeDef[] = [
    { key: "3h", label: "Last 3 hours", words: "last 3 hours", seconds: 3 * 3600 },
    { key: "6h", label: "Last 6 hours", words: "last 6 hours", seconds: 6 * 3600 },
    { key: "24h", label: "Last 24 hours", words: "last 24 hours", seconds: DAY },
    { key: "7d", label: "Last 7 days", words: "last 7 days", seconds: 7 * DAY },
    { key: "30d", label: "Last 30 days", words: "last 30 days", seconds: 30 * DAY },
    { key: "3m", label: "Last 3 months", words: "last 3 months", seconds: 90 * DAY },
    { key: "12m", label: "Last 12 months", words: "last 12 months", seconds: 365 * DAY },
];

export function rangeDef(key: RangeKey): RangeDef {
    return RANGES.find(r => r.key === key) ?? RANGES.find(r => r.key === DEFAULT_RANGE)!;
}

/** K2: the view last picked in this browser session; the page opens on it when the URL names none. */
let remembered: RangeKey | null = null;

export const rememberRange = (key: RangeKey) => {
    remembered = key;
};
export const rememberedRange = (): RangeKey | null => remembered;

/** Forgets the remembered view (tests). */
export const resetRememberedRange = () => {
    remembered = null;
};

/** K2: an invalid or missing value reads as the default. */
export function parseRange(value: string | null | undefined): RangeKey {
    return RANGES.some(r => r.key === value) ? (value as RangeKey) : DEFAULT_RANGE;
}

/** K11: the views whose window the statistics retention covers, shortest first. */
export function offeredRanges(retention: StatsRetention): RangeDef[] {
    const days = STATS_RETENTION_OPTIONS.find(o => o.value === retention)?.days ?? 30;
    return RANGES.filter(r => r.seconds <= days * DAY);
}

/** K2, K11: the view to show for a `?range=` value; a hidden view reads as the longest one offered. */
export function resolveRange(value: string | null | undefined, retention: StatsRetention): RangeKey {
    const asked = parseRange(value);
    const offered = offeredRanges(retention);
    return offered.some(r => r.key === asked) ? asked : offered[offered.length - 1].key;
}

export function nextLongerRange(key: RangeKey): RangeKey | null {
    const i = RANGES.findIndex(r => r.key === key);
    return i >= 0 && i < RANGES.length - 1 ? RANGES[i + 1].key : null;
}

// Wire values: generated enum member names are not stable across generator versions; the value
// unions are, and they still type-check these maps.

/** One timespan serves all three top lists, so it must be valid for each. */
type LogsTimespan = ApiV1ProfilesIdLogsTopDomainsGetTimespanEnum &
    ApiV1ProfilesIdLogsTopClientsGetTimespanEnum &
    ApiV1ProfilesIdLogsTopBlocklistsGetTimespanEnum;

/** K3 */
export const STATS_TIMESPAN: Record<RangeKey, StatsTimespan> = {
    "3h": "LAST_3_HOURS",
    "6h": "LAST_6_HOURS",
    "24h": "LAST_1_DAY",
    "7d": "LAST_7_DAYS",
    "30d": "LAST_MONTH",
    "3m": "LAST_3_MONTHS",
    "12m": "LAST_YEAR",
};

/** K4: logs never exceed one month, so the long views read LAST_MONTH. */
export const LOGS_TIMESPAN: Record<RangeKey, LogsTimespan> = {
    "3h": "LAST_3_HOURS",
    "6h": "LAST_6_HOURS",
    "24h": "LAST_1_DAY",
    "7d": "LAST_7_DAYS",
    "30d": "LAST_MONTH",
    "3m": "LAST_MONTH",
    "12m": "LAST_MONTH",
};
