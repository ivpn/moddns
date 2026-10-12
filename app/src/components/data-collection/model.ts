// Pure state machine behind DataCollectionControl.
//
// Source of truth: docs/specs/statistics-behaviour.md Sections L, T, C.
// If a row changes, update that spec and the matching tests with
// `tableRef: statistics-behaviour #<row>` annotations.

import {
    ModelProfileUpdateOperationEnum,
    ModelProfileUpdatePathEnum,
    type ModelProfile,
    type ModelProfileUpdate,
} from "@/api/client";

export type LogsRetention = "1h" | "6h" | "1d" | "1w" | "1m";

export interface DataCollectionState {
    /** Mirrors `statistics.enabled`. */
    stats: boolean;
    /** Mirrors `logs.enabled`. */
    logs: boolean;
    statsRetention: StatsRetention;
    domains: boolean;
    ips: boolean;
    retention: LogsRetention;
}

/** S0 Off, S1 Statistics, S2 Statistics + query logs, S3 Query logs only. */
export type StateKey = "S0" | "S1" | "S2" | "S3";

export interface Transition {
    /** Spec row of the state change (T1..T12), or "T13" / "T14" / "T15" when only sub-options change. */
    id: string;
    label: string;
    toast: string;
    note?: string;
    dialog?: { title: string; body: string; confirm: string };
    /** Statistics retention raised (T14) or lowered (T15) in the same save. */
    retention?: "raise" | "lower";
    /** Lowering that deletes counts: how many (C25), for the inline hint. */
    lost?: string;
    /** Lowering with no count older than the new retention yet: nothing is deleted (T15). */
    nothingOlder?: boolean;
}

export const LOGS_RETENTION_WORDS: Record<LogsRetention, string> = {
    "1h": "1 hour",
    "6h": "6 hours",
    "1d": "1 day",
    "1w": "1 week",
    "1m": "1 month",
};

export const LOGS_RETENTION_OPTIONS: { value: LogsRetention; label: string }[] = [
    { value: "1h", label: "1 H" },
    { value: "6h", label: "6 H" },
    { value: "1d", label: "1 D" },
    { value: "1w", label: "1 W" },
    { value: "1m", label: "1 M" },
];

export type StatsRetention = "30d" | "90d" | "1y";

export const STATS_RETENTION_OPTIONS: { value: StatsRetention; label: string; words: string; days: number }[] = [
    { value: "30d", label: "30 D", words: "30 days", days: 30 },
    { value: "90d", label: "90 D", words: "90 days", days: 90 },
    { value: "1y", label: "1 Y", words: "1 year", days: 365 },
];

/** An empty or unknown value reads as 30d (api-endpoint-behaviour.md J48); the API may or may not normalise it. */
export function normalizeStatsRetention(value: unknown): StatsRetention {
    return STATS_RETENTION_OPTIONS.find(o => o.value === value)?.value ?? "30d";
}

export function statsRetentionWords(value: unknown): string {
    return STATS_RETENTION_OPTIONS.find(o => o.value === normalizeStatsRetention(value))!.words;
}

export function profileStatsRetention(profile: ModelProfile | null | undefined): StatsRetention {
    return normalizeStatsRetention(profile?.settings?.statistics?.retention);
}

export const LIMITED_ACCESS_TEXT =
    "In limited access mode you can turn data collection off and delete what is stored, but not turn it on or change it.";
/** C26: inactive and pending_delete accounts (only Account preferences is reachable). */
export const CUT_OFF_TEXT = "Your account can't change data collection now. You can still turn it off and delete what is stored.";
export const STALE_TEXT = "This setting was changed elsewhere. Review it and try again.";

/** S4: `/settings#data-collection` lands on the statistics retention. */
export const DATA_COLLECTION_ANCHOR = "data-collection";

export const SAVE_ERROR_TEXT = "Couldn't save the data collection setting. Showing the current setting.";

export function fromProfile(profile: ModelProfile | null | undefined): DataCollectionState {
    const logs = profile?.settings?.logs;
    const stats = profile?.settings?.statistics;
    const retention = logs?.retention as LogsRetention | undefined;
    return {
        stats: !!stats?.enabled,
        logs: !!logs?.enabled,
        statsRetention: normalizeStatsRetention(stats?.retention),
        domains: logs?.log_domains ?? true,
        ips: logs?.log_clients_ips ?? false,
        retention: retention && retention in LOGS_RETENTION_WORDS ? retention : "1h",
    };
}

export function stateKey(s: Pick<DataCollectionState, "stats" | "logs">): StateKey {
    if (s.logs) return s.stats ? "S2" : "S3";
    return s.stats ? "S1" : "S0";
}

function statsDays(r: StatsRetention): number {
    return STATS_RETENTION_OPTIONS.find(o => o.value === r)!.days;
}

/** T14 / T15 apply only while statistics stay on; with statistics off nothing is stored to move (T16). */
function retentionChange(saved: DataCollectionState, pending: DataCollectionState): "raise" | "lower" | null {
    if (!saved.stats || !pending.stats || saved.statsRetention === pending.statsRetention) return null;
    return statsDays(pending.statsRetention) > statsDays(saved.statsRetention) ? "raise" : "lower";
}

function logsSubOptionsChanged(saved: DataCollectionState, pending: DataCollectionState): boolean {
    return (
        pending.logs &&
        (pending.domains !== saved.domains || pending.ips !== saved.ips || pending.retention !== saved.retention)
    );
}

const UNDONE = "This action cannot be undone.";
const DAY_MS = 86_400_000;

/** What the stored counts cover (C25): the oldest count, and the first daily bucket the server keeps. */
export interface StatsHeld {
    nowMs: number;
    oldestMs: number;
    firstDayMs: number;
}

const parseTime = (v: unknown): number | null => {
    const t = typeof v === "string" ? Date.parse(v) : NaN;
    return Number.isNaN(t) ? null : t;
};
const floorDay = (ms: number) => Math.floor(ms / DAY_MS) * DAY_MS;

/** C25: null without a counting start, which keeps the general copy. */
export function statsHeld(profile: ModelProfile | null | undefined, nowMs: number): StatsHeld | null {
    const s = profile?.settings?.statistics;
    if (!s?.enabled) return null;
    const enabledAt = parseTime(s.enabled_at);
    const deletedAt = parseTime(s.history_deleted_at);
    if (enabledAt === null && deletedAt === null) return null;
    const limit = nowMs - statsDays(normalizeStatsRetention(s.retention)) * DAY_MS;
    // J53: the daily bucket holding a history delete also held older counts, so it is gone too.
    const afterDelete = deletedAt !== null && (enabledAt === null || deletedAt > enabledAt);
    const start = afterDelete ? deletedAt : enabledAt!;
    const startDay = afterDelete ? floorDay(deletedAt) + DAY_MS : floorDay(enabledAt!);
    return { nowMs, oldestMs: Math.max(start, limit), firstDayMs: Math.max(startDay, floorDay(limit)) };
}

const days = (n: number) => `${n} ${n === 1 ? "day" : "days"}`;

function localDate(ms: number, nowMs: number): string {
    const sameYear = new Date(ms).getFullYear() === new Date(nowMs).getFullYear();
    return new Intl.DateTimeFormat(undefined, { month: "short", day: "numeric", year: sameYear ? undefined : "numeric" }).format(ms);
}

function localTime(ms: number, nowMs: number): string {
    const clock: Intl.DateTimeFormatOptions = { hour: "2-digit", minute: "2-digit" };
    const today = new Date(ms).toDateString() === new Date(nowMs).toDateString();
    return new Intl.DateTimeFormat(undefined, today ? clock : { month: "short", day: "numeric", ...clock }).format(ms);
}

const utcDate = (ms: number) => new Intl.DateTimeFormat(undefined, { month: "short", day: "numeric", timeZone: "UTC" }).format(ms);

/** C25: every stored count, for turning statistics off. */
export function heldSentence(h: StatsHeld): string {
    const n = Math.floor((h.nowMs - h.oldestMs) / DAY_MS);
    return n < 1 ? `The counts go back to ${localTime(h.oldestMs, h.nowMs)}.` : `The counts go back to ${localDate(h.oldestMs, h.nowMs)} (${days(n)}).`;
}

/** C25: the sentence goes after the first one. */
export function afterFirstSentence(body: string, sentence: string): string {
    const i = body.indexOf(". ");
    return i < 0 ? `${body} ${sentence}` : `${body.slice(0, i + 1)} ${sentence}${body.slice(i + 1)}`;
}

/** C25: the daily buckets a lowering deletes (J50: before floor_day(now - N)); null when there are none. */
function lostSentence(h: StatsHeld, keepDays: number): string | null {
    const cutoff = floorDay(h.nowMs - keepDays * DAY_MS);
    const n = Math.round((cutoff - h.firstDayMs) / DAY_MS);
    if (n < 1) return null;
    const first = utcDate(h.firstDayMs);
    return n === 1 ? `That is 1 day of counts, ${first}.` : `That is ${days(n)} of counts, from ${first} to ${utcDate(cutoff - DAY_MS)}.`;
}

/** C25: turning statistics off deletes every stored count. */
const DELETES_STATS = ["T4", "T6", "T7", "T9"];

function withHeld(t: Transition | null, h: StatsHeld | null | undefined): Transition | null {
    if (!t?.dialog || !h || !DELETES_STATS.includes(t.id)) return t;
    return { ...t, dialog: { ...t.dialog, body: afterFirstSentence(t.dialog.body, heldSentence(h)) } };
}

const FIRST = " The first counts appear at the next quarter hour.";

function stateTransition(from: StateKey, to: StateKey, logsWords: string, statsWords: string): Transition | null {
    switch (`${from}>${to}`) {
        case "S0>S1":
            return {
                id: "T1",
                label: "Turn on statistics",
                toast: "Statistics turned on.",
                note: `modDNS will start counting this profile's queries per device and keep the counts for ${statsWords}. No domains or addresses are stored.${FIRST}`,
            };
        case "S0>S2":
            return {
                id: "T2",
                label: "Turn on statistics and query logs",
                toast: "Statistics and query logs turned on.",
                note: `modDNS will record each query for ${logsWords} and keep counts per device for ${statsWords}.${FIRST}`,
            };
        case "S0>S3":
            return {
                id: "T3",
                label: "Turn on query logs",
                toast: "Query logs turned on.",
                note: `modDNS will record each query for ${logsWords}. No counts are kept.`,
            };
        case "S1>S0":
            return {
                id: "T4",
                label: "Turn off",
                toast: "Statistics turned off and deleted.",
                dialog: {
                    title: "Turn off statistics?",
                    body: `All statistics for this profile will be permanently deleted. ${UNDONE}`,
                    confirm: "Turn off and delete",
                },
            };
        case "S1>S2":
            return {
                id: "T5",
                label: "Turn on query logs",
                toast: "Query logs turned on.",
                note: `modDNS will also record each query for ${logsWords}.`,
            };
        case "S1>S3":
            return {
                id: "T6",
                label: "Save changes",
                toast: "Data collection updated.",
                note: `modDNS will record each query for ${logsWords}. No counts are kept.`,
                dialog: {
                    title: "Switch to query logs only?",
                    body: `All statistics for this profile will be permanently deleted, and modDNS will start recording each query for ${logsWords}. ${UNDONE}`,
                    confirm: "Switch and delete statistics",
                },
            };
        case "S2>S0":
            return {
                id: "T7",
                label: "Turn off",
                toast: "Data collection turned off. Logs and statistics deleted.",
                dialog: {
                    title: "Turn off data collection?",
                    body: `All query logs and statistics for this profile will be permanently deleted. ${UNDONE}`,
                    confirm: "Turn off and delete",
                },
            };
        case "S2>S1":
            return {
                id: "T8",
                label: "Save changes",
                toast: "Query logs turned off and deleted.",
                dialog: {
                    title: "Turn off query logs?",
                    body: `All query logs for this profile will be permanently deleted. Statistics stay on. ${UNDONE}`,
                    confirm: "Delete logs",
                },
            };
        case "S2>S3":
            return {
                id: "T9",
                label: "Save changes",
                toast: "Statistics turned off and deleted.",
                dialog: {
                    title: "Stop keeping statistics?",
                    body: `All statistics for this profile will be permanently deleted. Query logs stay on. ${UNDONE}`,
                    confirm: "Delete statistics",
                },
            };
        case "S3>S0":
            return {
                id: "T10",
                label: "Turn off",
                toast: "Query logs turned off and deleted.",
                dialog: {
                    title: "Turn off query logs?",
                    body: `All query logs for this profile will be permanently deleted. ${UNDONE}`,
                    confirm: "Turn off and delete",
                },
            };
        case "S3>S1":
            return {
                id: "T11",
                label: "Save changes",
                toast: "Data collection updated.",
                note: `modDNS will start counting this profile's queries per device and keep the counts for ${statsWords}.${FIRST}`,
                dialog: {
                    title: "Switch to statistics?",
                    body: `All query logs for this profile will be permanently deleted, and modDNS will start counting queries per device for ${statsWords}. ${UNDONE}`,
                    confirm: "Switch and delete logs",
                },
            };
        case "S3>S2":
            return {
                id: "T12",
                label: "Turn on statistics",
                toast: "Statistics turned on.",
                note: `modDNS will also keep counts per device for ${statsWords}.${FIRST}`,
            };
        default:
            return null;
    }
}

/** `held` (statsHeld) adds how much a deleting change loses (C25); without it the copy stays general. */
export function transitionFor(saved: DataCollectionState, pending: DataCollectionState, held?: StatsHeld | null): Transition | null {
    const logsWords = LOGS_RETENTION_WORDS[pending.retention];
    const statsWords = statsRetentionWords(pending.statsRetention);
    const retention = retentionChange(saved, pending);
    const logsSubs = logsSubOptionsChanged(saved, pending);
    const lost = retention === "lower" && held ? lostSentence(held, statsDays(pending.statsRetention)) : null;
    const nothingOlder = retention === "lower" && !!held && lost === null;
    const keepNote = `No counts are older than ${statsWords} yet, so nothing is deleted.`;

    let t = withHeld(stateTransition(stateKey(saved), stateKey(pending), logsWords, statsWords), held);
    if (!t) {
        if (retention === "raise" && !logsSubs) {
            return {
                id: "T14",
                label: `Keep for ${statsWords}`,
                toast: `Statistics are now kept for ${statsWords}.`,
                note: `modDNS will keep this profile's counts for up to ${statsWords}.`,
                retention,
            };
        }
        if (retention === "lower" && !logsSubs) {
            if (nothingOlder) {
                return { id: "T15", label: `Keep for ${statsWords}`, toast: `Statistics are now kept for ${statsWords}.`, note: keepNote, retention, nothingOlder };
            }
            const body = `Counts older than ${statsWords} will be permanently deleted. ${lost ? `${lost} ` : ""}${UNDONE}`;
            return {
                id: "T15",
                label: "Save changes",
                toast: `Statistics are now kept for ${statsWords}. Older counts deleted.`,
                dialog: { title: `Keep statistics for ${statsWords}?`, body, confirm: "Delete older counts" },
                retention,
                ...(lost ? { lost } : {}),
            };
        }
        if (!retention && !logsSubs) return null;
        t = { id: "T13", label: "Save changes", toast: "Data collection updated." };
    }
    if (retention === "raise") {
        const note = `modDNS will keep this profile's counts for up to ${statsWords}.`;
        t = { ...t, note: t.note ? `${t.note} ${note}` : note, retention };
    } else if (nothingOlder) {
        t = t.dialog ? { ...t, retention, nothingOlder } : { ...t, note: t.note ? `${t.note} ${keepNote}` : keepNote, retention, nothingOlder };
    } else if (retention === "lower") {
        const older = `Counts older than ${statsWords} will be permanently deleted.${lost ? ` ${lost}` : ""}`;
        t = {
            ...t,
            retention,
            ...(lost ? { lost } : {}),
            dialog: t.dialog
                ? { ...t.dialog, body: t.dialog.body.replace(UNDONE, `${older} ${UNDONE}`) }
                : { title: `Keep statistics for ${statsWords}?`, body: `${older} ${UNDONE}`, confirm: "Delete older counts" },
        };
    }
    return t;
}

function replace(path: ModelProfileUpdatePathEnum, value: boolean | string): ModelProfileUpdate {
    return { operation: ModelProfileUpdateOperationEnum.Replace, path, value: value as unknown as object };
}

/**
 * One PATCH body: the enabled paths that change, plus the staged sub-options of the checked cards
 * (T preamble, L6). A turn-off then carries only false values, which every subscription state accepts (G27).
 */
export function buildUpdates(saved: DataCollectionState, pending: DataCollectionState): ModelProfileUpdate[] {
    const updates: ModelProfileUpdate[] = [];
    if (pending.stats !== saved.stats) {
        updates.push(replace(ModelProfileUpdatePathEnum.SettingsStatisticsEnabled, pending.stats));
    }
    if (pending.logs !== saved.logs) {
        updates.push(replace(ModelProfileUpdatePathEnum.SettingsLogsEnabled, pending.logs));
    }
    if (pending.stats && pending.statsRetention !== saved.statsRetention) {
        updates.push(replace(ModelProfileUpdatePathEnum.SettingsStatisticsRetention, pending.statsRetention));
    }
    if (pending.logs) {
        if (pending.domains !== saved.domains) {
            updates.push(replace(ModelProfileUpdatePathEnum.SettingsLogsLogDomains, pending.domains));
        }
        if (pending.ips !== saved.ips) {
            updates.push(replace(ModelProfileUpdatePathEnum.SettingsLogsLogClientsIps, pending.ips));
        }
        if (pending.retention !== saved.retention) {
            updates.push(replace(ModelProfileUpdatePathEnum.SettingsLogsRetention, pending.retention));
        }
    }
    return updates;
}
