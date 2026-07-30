import { describe, it, expect } from 'vitest';
import { formatOutcome, outcomePairs, hasUnansweredMember } from '@/lib/formatOutcome';
import type { ModelQueryLog } from '@/api/client';

const member = (queryType?: string, outcome?: string, responseCode?: string): ModelQueryLog => ({
    outcome,
    dns_request: { query_type: queryType, response_code: responseCode },
});

describe('formatOutcome', () => {
    it('maps every outcome token to its label', () => {
        // tableRef: query-log-outcomes-behaviour O1-O9
        expect(formatOutcome('resolved')).toBe('Resolved');
        expect(formatOutcome('nodata')).toBe('No records');
        expect(formatOutcome('nxdomain')).toBe('Domain not found');
        expect(formatOutcome('blocked')).toBe('Blocked');
        expect(formatOutcome('servfail_dnssec')).toBe('DNSSEC validation failure');
        expect(formatOutcome('servfail_upstream')).toBe('Upstream failure');
        expect(formatOutcome('timeout')).toBe('Upstream timeout');
        expect(formatOutcome('network_error')).toBe('Upstream unreachable');
        expect(formatOutcome('refused')).toBe('Refused');
    });

    it('falls back to a response-code derived label for legacy entries', () => {
        // tableRef: query-log-outcomes-behaviour O10
        expect(formatOutcome(undefined, 'NOERROR')).toBe('Resolved');
        expect(formatOutcome('', 'NXDOMAIN')).toBe('Domain not found');
        expect(formatOutcome(undefined, 'SERVFAIL')).toBe('Upstream failure');
        expect(formatOutcome(undefined, 'REFUSED')).toBe('Refused');
        expect(formatOutcome(undefined, undefined)).toBe('Unknown');
        expect(formatOutcome('', '')).toBe('Unknown');
    });

    it('shows an unmapped response code verbatim instead of Unknown', () => {
        // tableRef: query-log-outcomes-behaviour OE4 — rare rcodes (FORMERR,
        // NOTIMP, ...) surface as-is; "Unknown" is reserved for entries with
        // neither outcome nor response code.
        expect(formatOutcome(undefined, 'FORMERR')).toBe('FORMERR');
        expect(formatOutcome('', 'NOTIMP')).toBe('NOTIMP');
    });

    it('shows an unknown token verbatim rather than hiding it', () => {
        // tableRef: query-log-outcomes-behaviour O10 (forward-compat: newer proxy than app)
        expect(formatOutcome('future_token')).toBe('future_token');
    });
});

describe('outcomePairs', () => {
    it('collapses exact duplicates and keeps member order', () => {
        // tableRef: query-log-outcomes-behaviour C1
        const r = outcomePairs([
            member('A', 'resolved'),
            member('AAAA', 'nodata'),
            member('HTTPS', 'nodata'),
            member('A', 'resolved'), // duplicate collapses
        ]);
        expect(r).toEqual([
            { queryType: 'A', label: 'Resolved', failure: false },
            { queryType: 'AAAA', label: 'No records', failure: false },
            { queryType: 'HTTPS', label: 'No records', failure: false },
        ]);
    });

    it('a uniform run collapses to one chip per query type', () => {
        // tableRef: query-log-outcomes-behaviour C1 — e.g. ×20 repeated blocked A queries
        const r = outcomePairs([
            member('A', 'blocked'),
            member('A', 'blocked'),
            member('A', 'blocked'),
        ]);
        expect(r).toEqual([{ queryType: 'A', label: 'Blocked', failure: true }]);
    });

    it('keeps same-type members with different outcomes as separate pairs', () => {
        // tableRef: query-log-outcomes-behaviour C1 — resolved query + timed-out retry
        const r = outcomePairs([
            member('A', 'resolved'),
            member('A', 'timeout'),
        ]);
        expect(r).toEqual([
            { queryType: 'A', label: 'Resolved', failure: false },
            { queryType: 'A', label: 'Upstream timeout', failure: true },
        ]);
    });

    it('falls back per member for legacy entries without outcome', () => {
        // tableRef: query-log-outcomes-behaviour C1, O10
        const r = outcomePairs([
            member('A', undefined, 'NOERROR'),
            member('AAAA', 'timeout'),
        ]);
        expect(r).toEqual([
            { queryType: 'A', label: 'Resolved', failure: false },
            { queryType: 'AAAA', label: 'Upstream timeout', failure: true },
        ]);
    });

    it('legacy blocked entries read the status, not the synthesized NOERROR rcode', () => {
        // tableRef: query-log-outcomes-behaviour O10 — a blocked response is a
        // synthesized NOERROR (0.0.0.0/::), so the rcode fallback alone would
        // wrongly render "Resolved" under a red Blocked pill.
        const legacyBlocked: ModelQueryLog = {
            status: 'blocked',
            dns_request: { query_type: 'A', response_code: 'NOERROR' },
        };
        expect(outcomePairs([legacyBlocked])).toEqual([
            { queryType: 'A', label: 'Blocked', failure: true },
        ]);
    });
});

describe('hasUnansweredMember', () => {
    it('flags each unanswered outcome token', () => {
        // tableRef: query-log-outcomes-behaviour C3 — collapsed-card chip trigger set
        for (const outcome of ['servfail_upstream', 'timeout', 'network_error', 'refused']) {
            expect(hasUnansweredMember([member('A', outcome)])).toBe(true);
        }
    });

    it('does not flag answered outcomes or nxdomain', () => {
        // tableRef: query-log-outcomes-behaviour C3 — nxdomain/nodata are healthy
        // protocol answers; resolved obviously so.
        for (const outcome of ['resolved', 'nodata', 'nxdomain']) {
            expect(hasUnansweredMember([member('A', outcome)])).toBe(false);
        }
    });

    it('does not flag servfail_dnssec — the red DNSSEC label owns that signal', () => {
        // tableRef: query-log-outcomes-behaviour C3 — the collapsed row already
        // shows a red "DNSSEC" text label for validation failures.
        expect(hasUnansweredMember([member('A', 'servfail_dnssec')])).toBe(false);
    });

    it('does not flag blocked entries — the Blocked pill owns those', () => {
        // tableRef: query-log-outcomes-behaviour C3
        expect(hasUnansweredMember([{ ...member('A', 'blocked'), status: 'blocked' }])).toBe(false);
        // legacy blocked: no outcome, synthesized NOERROR
        expect(hasUnansweredMember([{ ...member('A', undefined, 'NOERROR'), status: 'blocked' }])).toBe(false);
    });

    it('flags a mixed group when any member went unanswered', () => {
        // tableRef: query-log-outcomes-behaviour C3 — outcome is not part of the
        // consolidation signature, so a group can mix e.g. resolved + timeout.
        expect(hasUnansweredMember([member('A', 'resolved'), member('A', 'timeout')])).toBe(true);
        expect(hasUnansweredMember([member('A', 'resolved'), member('AAAA', 'nodata')])).toBe(false);
    });

    it('falls back to the response code for legacy entries', () => {
        // tableRef: query-log-outcomes-behaviour C3, O10 — legacy SERVFAIL/REFUSED
        // entries went unanswered too; NOERROR/NXDOMAIN did not.
        expect(hasUnansweredMember([member('A', undefined, 'SERVFAIL')])).toBe(true);
        expect(hasUnansweredMember([member('A', undefined, 'REFUSED')])).toBe(true);
        expect(hasUnansweredMember([member('A', undefined, 'NOERROR')])).toBe(false);
        expect(hasUnansweredMember([member('A', undefined, 'NXDOMAIN')])).toBe(false);
        expect(hasUnansweredMember([member('A')])).toBe(false);
    });

    it('legacy DNSSEC-failed SERVFAIL entries defer to the DNSSEC label', () => {
        // tableRef: query-log-outcomes-behaviour C3 — pre-outcome entries carry the
        // dnssec_failed reason (same signal as O5); the red DNSSEC label covers them.
        const legacyDnssec: ModelQueryLog = {
            ...member('A', undefined, 'SERVFAIL'),
            reasons: ['dnssec_failed'],
        };
        expect(hasUnansweredMember([legacyDnssec])).toBe(false);
    });
});

