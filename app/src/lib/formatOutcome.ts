// formatOutcome — map the proxy-computed resolution-outcome token to a
// human-readable label for the query-log UI.
//
// Source of truth: docs/specs/query-log-outcomes-behaviour.md (rows O1-O10).
// If the mapping changes, update that spec and formatOutcome.test.ts with
// matching `tableRef: query-log-outcomes-behaviour <row>` annotations.
//
// Distinct from formatReasons: reasons explain *filter decisions* (chips),
// outcome describes the *resolution state* of the answer.

const OUTCOME_LABELS: Record<string, string> = {
    resolved: 'Resolved',                       // O1
    nodata: 'No records of this type',          // O2
    nxdomain: 'Domain not found',               // O3
    blocked: 'Blocked',                         // O4
    servfail_dnssec: 'DNSSEC validation failure', // O5
    servfail_upstream: 'Upstream failure',      // O6
    timeout: 'Upstream timeout',                // O7
    network_error: 'Upstream unreachable',      // O8
    refused: 'Refused',                         // O9
};

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

export default formatOutcome;
