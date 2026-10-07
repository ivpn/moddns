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

export type Level = "off" | "stats" | "logs";
export type LogsRetention = "1h" | "6h" | "1d" | "1w" | "1m";

export interface DataCollectionState {
    level: Level;
    /** Mirrors `statistics.enabled`; only editable under Query logs. */
    keep: boolean;
    domains: boolean;
    ips: boolean;
    retention: LogsRetention;
}

/** S0 Off, S1 Statistics, S2 Query logs + statistics, S3 Query logs only. */
export type StateKey = "S0" | "S1" | "S2" | "S3";

export interface Transition {
    /** Spec row: T1..T12, or "T13" for sub-option-only changes. */
    id: string;
    label: string;
    toast: string;
    note?: string;
    dialog?: { title: string; body: string; confirm: string };
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

export const STATISTICS_RETENTION_WORDS = "30 days";

export const LIMITED_ACCESS_TEXT =
    "Data collection can't be changed in limited access mode. You can still clear query logs.";
export const SAVE_ERROR_TEXT = "Couldn't save the data collection setting. Showing the current setting.";

export function fromProfile(profile: ModelProfile | null | undefined): DataCollectionState {
    const logs = profile?.settings?.logs;
    const stats = profile?.settings?.statistics;
    const level: Level = logs?.enabled ? "logs" : stats?.enabled ? "stats" : "off";
    const retention = logs?.retention as LogsRetention | undefined;
    return {
        level,
        keep: !!stats?.enabled,
        domains: logs?.log_domains ?? true,
        ips: logs?.log_clients_ips ?? false,
        retention: retention && retention in LOGS_RETENTION_WORDS ? retention : "1h",
    };
}

export function stateKey(s: DataCollectionState): StateKey {
    if (s.level === "off") return "S0";
    if (s.level === "stats") return "S1";
    return s.keep ? "S2" : "S3";
}

function subOptionsChanged(saved: DataCollectionState, pending: DataCollectionState): boolean {
    return (
        pending.level === "logs" &&
        (pending.domains !== saved.domains || pending.ips !== saved.ips || pending.retention !== saved.retention)
    );
}

const UNDONE = "This action cannot be undone.";

export function transitionFor(
    saved: DataCollectionState,
    pending: DataCollectionState,
    statsWords: string = STATISTICS_RETENTION_WORDS,
): Transition | null {
    const from = stateKey(saved);
    const to = stateKey(pending);
    const logsWords = LOGS_RETENTION_WORDS[pending.retention];

    if (from === to) {
        return subOptionsChanged(saved, pending)
            ? { id: "T13", label: "Save changes", toast: "Data collection updated." }
            : null;
    }

    switch (`${from}>${to}`) {
        case "S0>S1":
            return {
                id: "T1",
                label: "Turn on statistics",
                toast: "Statistics turned on.",
                note: `modDNS will start counting this profile's queries per device and keep the counts for ${statsWords}. No domains or addresses are stored.`,
            };
        case "S0>S2":
            return {
                id: "T2",
                label: "Turn on query logs",
                toast: "Query logs turned on.",
                note: `modDNS will record each query for ${logsWords} and keep counts per device for ${statsWords}.`,
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
                note: `modDNS will start counting this profile's queries per device and keep the counts for ${statsWords}.`,
                dialog: {
                    title: "Switch to statistics?",
                    body: `All query logs for this profile will be permanently deleted, and modDNS will start counting queries per device for ${statsWords}. ${UNDONE}`,
                    confirm: "Switch and delete logs",
                },
            };
        case "S3>S2":
            return {
                id: "T12",
                label: "Save changes",
                toast: "Statistics turned on.",
                note: `modDNS will also keep counts per device for ${statsWords}.`,
            };
        default:
            return null;
    }
}

function replace(path: ModelProfileUpdatePathEnum, value: boolean | string): ModelProfileUpdate {
    return { operation: ModelProfileUpdateOperationEnum.Replace, path, value: value as unknown as object };
}

/** One PATCH body: both enabled paths on a level change, plus any staged sub-options (T-section preamble). */
export function buildUpdates(saved: DataCollectionState, pending: DataCollectionState): ModelProfileUpdate[] {
    const updates: ModelProfileUpdate[] = [];
    if (stateKey(saved) !== stateKey(pending)) {
        const statsOn = pending.level === "stats" || (pending.level === "logs" && pending.keep);
        updates.push(replace(ModelProfileUpdatePathEnum.SettingsStatisticsEnabled, statsOn));
        updates.push(replace(ModelProfileUpdatePathEnum.SettingsLogsEnabled, pending.level === "logs"));
    }
    if (pending.level === "logs") {
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
