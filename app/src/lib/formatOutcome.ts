// formatOutcome — map the proxy-computed resolution-outcome token to a
// human-readable label for the query-log UI.
//
// Source of truth: docs/specs/query-log-outcomes-behaviour.md (rows O1-O10,
// Queries-display row C1). If the mapping changes, update that spec and
// formatOutcome.test.ts with matching
// `tableRef: query-log-outcomes-behaviour <row>` annotations.
//
// Distinct from formatReasons: reasons explain *filter decisions* (chips),
// outcome describes the *resolution state* of the answer. Outcomes are always
// rendered as `type · outcome` pair chips in the card's "Queries" block, so
// the nodata label stays generic — the chip's type prefix supplies the "which".

import type { ModelQueryLog } from '@/api/client';

const OUTCOME_LABELS: Record<string, string> = {
    resolved: 'Resolved',                         // O1
    nodata: 'No records',                         // O2
    nxdomain: 'Domain not found',                 // O3
    blocked: 'Blocked',                           // O4
    servfail_dnssec: 'DNSSEC validation failure', // O5
    servfail_upstream: 'Upstream failure',        // O6
    timeout: 'Upstream timeout',                  // O7
    network_error: 'Upstream unreachable',        // O8
    refused: 'Refused',                           // O9
};

// Failure-class tokens get the red tint on pair chips.
const FAILURE_OUTCOMES = new Set([
    'blocked', 'servfail_dnssec', 'servfail_upstream', 'timeout', 'network_error', 'refused',
]);

// O10 legacy fallback: entries written before the outcome field existed only
// carry a response code.
const LEGACY_RCODE_LABELS: Record<string, string> = {
    NOERROR: 'Resolved',
    NXDOMAIN: 'Domain not found',
    SERVFAIL: 'Upstream failure',
    REFUSED: 'Refused',
};

/**
 * @param outcome      Raw token from `ModelQueryLog.outcome` (may be absent).
 * @param responseCode Raw rcode string, used only as the legacy fallback.
 */
export function formatOutcome(outcome?: string, responseCode?: string): string {
    if (outcome) {
        // Unknown tokens (a newer proxy than this app) render verbatim rather
        // than disappearing — mirrors the O10 forward-compat rule.
        return OUTCOME_LABELS[outcome] ?? outcome;
    }
    if (responseCode) {
        // Rare rcodes outside the map (FORMERR, NOTIMP, ...) surface verbatim —
        // "Unknown" is reserved for entries with neither outcome nor rcode.
        return LEGACY_RCODE_LABELS[responseCode] ?? responseCode;
    }
    return 'Unknown';
}

export interface OutcomePair {
    queryType: string;
    label: string;
    failure: boolean;
}

/**
 * Distinct (query type, outcome label) pairs for the always-rendered "Queries"
 * chip block (C1). Works for a single entry (pass `[log]`) and consolidated
 * groups alike; exact duplicates collapse, member order is preserved, legacy
 * members fall back per member via formatOutcome (O10).
 */
export function outcomePairs(members: ModelQueryLog[]): OutcomePair[] {
    const pairs: OutcomePair[] = [];
    const seen = new Set<string>();
    for (const m of members) {
        const queryType = m.dns_request?.query_type ?? '';
        // O10: legacy blocked entries have no outcome but a synthesized NOERROR
        // rcode — the status is the truthful signal, never "Resolved".
        const effectiveOutcome = !m.outcome && m.status === 'blocked' ? 'blocked' : m.outcome;
        const label = formatOutcome(effectiveOutcome, m.dns_request?.response_code);
        const key = `${queryType} ${label}`;
        if (seen.has(key)) continue;
        seen.add(key);
        pairs.push({ queryType, label, failure: FAILURE_OUTCOMES.has(effectiveOutcome ?? '') });
    }
    return pairs;
}

// Collapsed-card "Not answered" chip trigger set (C3). Deliberately narrower
// than FAILURE_OUTCOMES: `blocked` is owned by the red Blocked pill and
// `servfail_dnssec` by the red DNSSEC text label already on the collapsed row.
const UNANSWERED_OUTCOMES = new Set([
    'servfail_upstream', 'timeout', 'network_error', 'refused',
]);

// O10 legacy entries carry only an rcode; these two mean the query went
// unanswered. NOERROR/NXDOMAIN (and unmapped rcodes) do not trigger the chip.
const UNANSWERED_LEGACY_RCODES = new Set(['SERVFAIL', 'REFUSED']);

/**
 * Should the collapsed row show the amber "Not answered" chip? True when ANY
 * member went unanswered (C3) — `outcome` is not part of the consolidation
 * signature, so a group can mix e.g. a resolved query with a timed-out retry
 * and the representative alone would hide the failure.
 */
export function hasUnansweredMember(members: ModelQueryLog[]): boolean {
    return members.some((m) => {
        if (m.status === 'blocked') return false; // O4/O10: Blocked pill owns it
        if (m.outcome) return UNANSWERED_OUTCOMES.has(m.outcome);
        // Legacy DNSSEC failures are SERVFAIL + dnssec_failed reason (the O5
        // signal) — the red DNSSEC label covers them, like modern servfail_dnssec.
        if (m.reasons?.includes('dnssec_failed')) return false;
        return UNANSWERED_LEGACY_RCODES.has(m.dns_request?.response_code ?? '');
    });
}

export default formatOutcome;
