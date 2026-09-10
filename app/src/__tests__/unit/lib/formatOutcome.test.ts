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

    it('labels a filtering-unavailable SERVFAIL distinctly from upstream failures', () => {
        // tableRef: query-log-outcomes-behaviour O11 — the proxy synthesized the
        // SERVFAIL itself because a filter stage could not read the settings store.
        expect(formatOutcome('filter_unavailable')).toBe('Filtering unavailable');
    });

    it('shows the response code verbatim when the outcome is absent, Unknown when both are', () => {
        // tableRef: query-log-outcomes-behaviour O10, OE4 — an absent outcome is
        // the proxy's defensive case (e.g. a FORMERR/NOTIMP rcode outside the
        // table); the rcode is shown as-is, never mapped to an outcome label.
        expect(formatOutcome(undefined, 'FORMERR')).toBe('FORMERR');
        expect(formatOutcome('', 'NOTIMP')).toBe('NOTIMP');
        expect(formatOutcome(undefined, 'SERVFAIL')).toBe('SERVFAIL');
        expect(formatOutcome(undefined, undefined)).toBe('Unknown');
        expect(formatOutcome('', '')).toBe('Unknown');
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

    it('renders filter_unavailable as a failure-class chip', () => {
        // tableRef: query-log-outcomes-behaviour C1, O11
        expect(outcomePairs([member('A', 'filter_unavailable')])).toEqual([
            { queryType: 'A', label: 'Filtering unavailable', failure: true },
        ]);
    });

    it('a member without an outcome shows its rcode verbatim and is not failure-tinted', () => {
        // tableRef: query-log-outcomes-behaviour C1, O10, OE4
        const r = outcomePairs([
            member('A', undefined, 'FORMERR'),
            member('AAAA', 'timeout'),
        ]);
        expect(r).toEqual([
            { queryType: 'A', label: 'FORMERR', failure: false },
            { queryType: 'AAAA', label: 'Upstream timeout', failure: true },
        ]);
    });
});

describe('hasUnansweredMember', () => {
    it('flags each unanswered outcome token', () => {
        // tableRef: query-log-outcomes-behaviour C3 — collapsed-card chip trigger set
        // (O11 filter_unavailable is a synthesized SERVFAIL — unanswered too)
        for (const outcome of ['servfail_upstream', 'timeout', 'network_error', 'refused', 'filter_unavailable']) {
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
    });

    it('flags a mixed group when any member went unanswered', () => {
        // tableRef: query-log-outcomes-behaviour C3 — outcome is not part of the
        // consolidation signature, so a group can mix e.g. resolved + timeout.
        expect(hasUnansweredMember([member('A', 'resolved'), member('A', 'timeout')])).toBe(true);
        expect(hasUnansweredMember([member('A', 'resolved'), member('AAAA', 'nodata')])).toBe(false);
    });

    it('never infers "No answer" from the response code alone', () => {
        // tableRef: query-log-outcomes-behaviour C3, O10 — an absent outcome is
        // the defensive case, not a legacy row; the rcode is not a trigger.
        expect(hasUnansweredMember([member('A', undefined, 'SERVFAIL')])).toBe(false);
        expect(hasUnansweredMember([member('A', undefined, 'REFUSED')])).toBe(false);
        expect(hasUnansweredMember([member('A')])).toBe(false);
    });
});

