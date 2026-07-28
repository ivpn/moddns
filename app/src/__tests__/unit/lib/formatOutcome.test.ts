import { describe, it, expect } from 'vitest';
import { formatOutcome, consolidatedOutcomePairs } from '@/lib/formatOutcome';
import type { ModelQueryLog } from '@/api/client';

const member = (queryType?: string, outcome?: string, responseCode?: string): ModelQueryLog => ({
    outcome,
    dns_request: { query_type: queryType, response_code: responseCode },
});

describe('formatOutcome', () => {
    it('maps every outcome token to its label', () => {
        // tableRef: query-log-outcomes-behaviour O1-O9
        expect(formatOutcome('resolved')).toBe('Resolved');
        expect(formatOutcome('nxdomain')).toBe('Domain not found');
        expect(formatOutcome('blocked')).toBe('Blocked');
        expect(formatOutcome('servfail_dnssec')).toBe('DNSSEC validation failure');
        expect(formatOutcome('servfail_upstream')).toBe('Upstream failure');
        expect(formatOutcome('timeout')).toBe('Upstream timeout');
        expect(formatOutcome('network_error')).toBe('Upstream unreachable');
        expect(formatOutcome('refused')).toBe('Refused');
    });

    it('renders nodata type-aware when the query type is known', () => {
        // tableRef: query-log-outcomes-behaviour O2
        expect(formatOutcome('nodata', undefined, 'AAAA')).toBe('No AAAA records');
        expect(formatOutcome('nodata', undefined, 'HTTPS')).toBe('No HTTPS records');
        expect(formatOutcome('nodata')).toBe('Empty answer');
        expect(formatOutcome('nodata', 'NOERROR', '')).toBe('Empty answer');
    });

    it('renders nodata generically for paired-chip contexts', () => {
        // tableRef: query-log-outcomes-behaviour O2, C2 — the chip's type prefix
        // supplies the "which", so the label stays generic.
        expect(formatOutcome('nodata', undefined, 'AAAA', { generic: true })).toBe('No records');
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

describe('consolidatedOutcomePairs', () => {
    it('reports a uniform group with its single label and no pairs to render', () => {
        // tableRef: query-log-outcomes-behaviour C1
        const r = consolidatedOutcomePairs([
            member('A', 'blocked'),
            member('AAAA', 'blocked'),
            member('A', 'blocked'),
        ]);
        expect(r.uniform).toBe(true);
        expect(r.uniformLabel).toBe('Blocked');
    });

    it('returns distinct type·outcome pairs for a mismatched group', () => {
        // tableRef: query-log-outcomes-behaviour C2
        const r = consolidatedOutcomePairs([
            member('A', 'resolved'),
            member('AAAA', 'nodata'),
            member('HTTPS', 'nodata'),
            member('A', 'resolved'), // duplicate collapses
        ]);
        expect(r.uniform).toBe(false);
        expect(r.pairs).toEqual([
            { queryType: 'A', label: 'Resolved', failure: false },
            { queryType: 'AAAA', label: 'No records', failure: false },
            { queryType: 'HTTPS', label: 'No records', failure: false },
        ]);
    });

    it('keeps same-type members with different outcomes as separate pairs', () => {
        // tableRef: query-log-outcomes-behaviour C2 — resolved query + timed-out retry
        const r = consolidatedOutcomePairs([
            member('A', 'resolved'),
            member('A', 'timeout'),
        ]);
        expect(r.uniform).toBe(false);
        expect(r.pairs).toEqual([
            { queryType: 'A', label: 'Resolved', failure: false },
            { queryType: 'A', label: 'Upstream timeout', failure: true },
        ]);
    });

    it('falls back per member for legacy entries without outcome', () => {
        // tableRef: query-log-outcomes-behaviour C2, O10
        const r = consolidatedOutcomePairs([
            member('A', undefined, 'NOERROR'),
            member('AAAA', 'timeout'),
        ]);
        expect(r.uniform).toBe(false);
        expect(r.pairs).toEqual([
            { queryType: 'A', label: 'Resolved', failure: false },
            { queryType: 'AAAA', label: 'Upstream timeout', failure: true },
        ]);
    });
});
