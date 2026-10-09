// Timeframe picker model.
//
// Source of truth: docs/specs/statistics-behaviour.md Section K.

import { STATS_RETENTION_OPTIONS, type StatsRetention } from "@/components/data-collection/model";
import {
    ApiV1ProfilesIdLogsTopGetTimespanEnum as LogsTimespan,
    ApiV1ProfilesIdStatisticsGetTimespanEnum as StatsTimespan,
} from "@/api/client";

export type RangeKey = "3h" | "6h" | "24h" | "7d" | "30d" | "3m" | "12m";

export const DEFAULT_RANGE: RangeKey = "7d";

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
    return RANGES.find(r => r.key === key) ?? RANGES[3];
}

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

/** K3 */
export const STATS_TIMESPAN: Record<RangeKey, (typeof StatsTimespan)[keyof typeof StatsTimespan]> = {
    "3h": StatsTimespan._3Hours,
    "6h": StatsTimespan._6Hours,
    "24h": StatsTimespan._1Day,
    "7d": StatsTimespan._7Days,
    "30d": StatsTimespan.Month,
    "3m": StatsTimespan._3Months,
    "12m": StatsTimespan.Year,
};

/** K4: logs never exceed one month, so the long views read LAST_MONTH. */
export const LOGS_TIMESPAN: Record<RangeKey, (typeof LogsTimespan)[keyof typeof LogsTimespan]> = {
    "3h": LogsTimespan._3Hours,
    "6h": LogsTimespan._6Hours,
    "24h": LogsTimespan._1Day,
    "7d": LogsTimespan._7Days,
    "30d": LogsTimespan.Month,
    "3m": LogsTimespan.Month,
    "12m": LogsTimespan.Month,
};

const LOGS_RETENTION_SECONDS: Record<string, number> = {
    "1h": 3600,
    "6h": 6 * 3600,
    "1d": DAY,
    "1w": 7 * DAY,
    "1m": 30 * DAY,
};

const LOGS_RETENTION_PHRASE: Record<string, string> = {
    "1h": "1 hour",
    "6h": "6 hours",
    "1d": "1 day",
    "1w": "1 week",
    "1m": "1 month",
};

/** P18: the phrase to show when the logs window is shorter than the view, else null. */
export function logsWindowCaption(range: RangeKey, logsRetention: string | undefined): string | null {
    const secs = LOGS_RETENTION_SECONDS[logsRetention ?? ""];
    if (!secs || secs >= rangeDef(range).seconds) return null;
    return `From the last ${LOGS_RETENTION_PHRASE[logsRetention as string]} of query logs`;
}
