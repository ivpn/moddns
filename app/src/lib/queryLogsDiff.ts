// queryLogsDiff — diff a freshly fetched page 1 against the displayed logs list.
//
// Used by the auto-refresh background tick: instead of replacing the list wholesale
// (which reset scroll and collapsed expanded cards), the tick computes which fetched
// entries are genuinely new and stages them behind a "N new queries" pill.

import type { ModelQueryLog } from "@/api/client";

// How many entries from the head of the displayed list to index when looking for the
// overlap point. Ticks fetch 100 rows, so the overlap — if any — sits within the first
// 100 displayed entries; 150 leaves slack for pill merges between ticks.
const OVERLAP_WINDOW = 150;

export interface QueryLogsDiff {
    /** Entries in `fetched` newer than the displayed head, in fetched (newest-first) order. */
    newLogs: ModelQueryLog[];
    /** False when `fetched` shares no entry with the displayed head — the lists don't touch. */
    overlapFound: boolean;
}

// Identity of one log entry for diffing. Prefers the server id; the fallback composite
// includes the timestamp AND the consolidation-signature fields plus query_type, because
// timestamps alone cannot discriminate — DNS bursts (A + AAAA) land in the same second.
export const logIdentity = (log: ModelQueryLog): string =>
    log.id ||
    [
        log.timestamp ?? "",
        log.dns_request?.domain ?? "",
        log.dns_request?.query_type ?? "",
        log.status ?? "",
        log.device_id ?? "",
        log.client_ip ?? "",
        log.protocol ?? "",
    ].join("|");

/**
 * Walk `fetched` (newest first) until the first entry already present near the head of
 * `current`; the prefix before that point is new. No overlap means `fetched` is entirely
 * unseen — at a full page size that implies a gap, which the caller must handle by
 * replacing instead of prepending. Pure, O(n).
 */
export function computeNewQueryLogs(
    fetched: ModelQueryLog[],
    current: ModelQueryLog[]
): QueryLogsDiff {
    const known = new Set(current.slice(0, OVERLAP_WINDOW).map(logIdentity));
    const newLogs: ModelQueryLog[] = [];
    for (const log of fetched) {
        if (known.has(logIdentity(log))) {
            return { newLogs, overlapFound: true };
        }
        newLogs.push(log);
    }
    return { newLogs, overlapFound: false };
}
