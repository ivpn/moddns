// formatOutcome — map the proxy-computed resolution-outcome token to a
// human-readable label for the query-log UI.
//
// Source of truth: docs/specs/query-log-outcomes-behaviour.md (rows O1-O11,
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
    filter_unavailable: 'Filtering unavailable',  // O11
};

// Failure-class tokens get the red tint on pair chips.
const FAILURE_OUTCOMES = new Set([
    'blocked', 'servfail_dnssec', 'servfail_upstream', 'timeout', 'network_error', 'refused',
    'filter_unavailable',
]);

/**
 * @param outcome      Raw token from `ModelQueryLog.outcome`. Every stored row
 *                     carries one (the field predates the longest retention);
 *                     an empty value is the proxy's defensive O10 case.
 * @param responseCode Raw rcode string, shown verbatim when there is no outcome
 *                     (OE4: rcodes outside the outcome table, e.g. FORMERR).
 */
export function formatOutcome(outcome?: string, responseCode?: string): string {
    if (outcome) {
        // Unknown tokens (a newer proxy than this app) render verbatim rather
        // than disappearing — the O10 forward-compat rule.
        return OUTCOME_LABELS[outcome] ?? outcome;
    }
    // "Unknown" is reserved for entries with neither outcome nor rcode.
    return responseCode || 'Unknown';
}

export interface OutcomePair {
    queryType: string;
    label: string;
    failure: boolean;
}

/**
 * Distinct (query type, outcome label) pairs for the always-rendered "Queries"
 * chip block (C1). Works for a single entry (pass `[log]`) and consolidated
 * groups alike; exact duplicates collapse, member order is preserved.
 */
export function outcomePairs(members: ModelQueryLog[]): OutcomePair[] {
    const pairs: OutcomePair[] = [];
    const seen = new Set<string>();
    for (const m of members) {
        const queryType = m.dns_request?.query_type ?? '';
        const label = formatOutcome(m.outcome, m.dns_request?.response_code);
        const key = `${queryType} ${label}`;
        if (seen.has(key)) continue;
        seen.add(key);
        pairs.push({ queryType, label, failure: FAILURE_OUTCOMES.has(m.outcome ?? '') });
    }
    return pairs;
}

// Collapsed-card "No answer" label trigger set (C3), shared with the API's
// `status=unanswered` filter (C5). Deliberately narrower than FAILURE_OUTCOMES:
// `blocked` is owned by the red Blocked pill and `servfail_dnssec` by the red
// DNSSEC text label already on the collapsed row — both are verdicts, not
// failures to answer.
const UNANSWERED_OUTCOMES = new Set([
    'servfail_upstream', 'timeout', 'network_error', 'refused', 'filter_unavailable',
]);

/**
 * Should the collapsed row show the amber "No answer" label? True when ANY
 * member went unanswered (C3) — `outcome` is not part of the consolidation
 * signature, so a group can mix e.g. a resolved query with a timed-out retry
 * and the representative alone would hide the failure.
 */
export function hasUnansweredMember(members: ModelQueryLog[]): boolean {
    return members.some((m) => UNANSWERED_OUTCOMES.has(m.outcome ?? ''));
}

export default formatOutcome;
