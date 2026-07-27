import { describe, it, expect } from 'vitest';
import { formatOutcome } from '@/lib/formatOutcome';

describe('formatOutcome', () => {
    it('maps every outcome token to its label', () => {
        // tableRef: query-log-outcomes-behaviour O1-O9
        expect(formatOutcome('resolved')).toBe('Resolved');
        expect(formatOutcome('nodata')).toBe('No records of this type');
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
