import { describe, it, expect } from 'vitest';
import { consolidateLogs, toSingletonGroup } from '@/lib/consolidateLogs';
import type { ModelQueryLog } from '@/api/client';

// Minimal log factory — override only what a test cares about.
const log = (over: Partial<ModelQueryLog> & { domain?: string; query_type?: string; response_code?: string }): ModelQueryLog => {
    const { domain, query_type, response_code, ...rest } = over;
    return {
        profile_id: 'p1',
        status: 'processed',
        protocol: 'dns',
        device_id: 'dev1',
        client_ip: '10.0.0.1',
        timestamp: '2026-06-15T10:00:00.000Z',
        dns_request: { domain, query_type, response_code },
        ...rest,
    };
};

describe('consolidateLogs', () => {
    it('merges an adjacent A + AAAA run for the same domain into one group', () => {
        const groups = consolidateLogs([
            log({ domain: 'example.com', query_type: 'A', response_code: 'NOERROR', timestamp: '2026-06-15T10:00:02.000Z' }),
            log({ domain: 'example.com', query_type: 'AAAA', response_code: 'NOERROR', timestamp: '2026-06-15T10:00:01.000Z' }),
        ]);
        expect(groups).toHaveLength(1);
        expect(groups[0].count).toBe(2);
        expect(groups[0].queryTypes).toEqual(['A', 'AAAA']);
        expect(groups[0].responseCodes).toEqual(['NOERROR']);
        expect(groups[0].representative.dns_request?.query_type).toBe('A');
        expect(groups[0].firstTimestamp).toBe('2026-06-15T10:00:02.000Z');
        expect(groups[0].lastTimestamp).toBe('2026-06-15T10:00:01.000Z');
    });

    it('keeps non-adjacent same-domain entries separate (X, Y, X -> 3 groups)', () => {
        const groups = consolidateLogs([
            log({ domain: 'x.com' }),
            log({ domain: 'y.com' }),
            log({ domain: 'x.com' }),
        ]);
        expect(groups.map((g) => g.representative.dns_request?.domain)).toEqual(['x.com', 'y.com', 'x.com']);
        expect(groups.every((g) => g.count === 1)).toBe(true);
    });

    it('does not merge across a status boundary', () => {
        const groups = consolidateLogs([
            log({ domain: 'ads.com', status: 'processed' }),
            log({ domain: 'ads.com', status: 'blocked' }),
        ]);
        expect(groups).toHaveLength(2);
    });

    it('does not merge across differing device_id, client_ip, or protocol', () => {
        expect(consolidateLogs([log({ domain: 'a.com', device_id: 'dev1' }), log({ domain: 'a.com', device_id: 'dev2' })])).toHaveLength(2);
        expect(consolidateLogs([log({ domain: 'a.com', client_ip: '10.0.0.1' }), log({ domain: 'a.com', client_ip: '10.0.0.2' })])).toHaveLength(2);
        expect(consolidateLogs([log({ domain: 'a.com', protocol: 'dns' }), log({ domain: 'a.com', protocol: 'doh' })])).toHaveLength(2);
    });

    it('merges an adjacent run of empty-domain rows but never empty with non-empty', () => {
        const merged = consolidateLogs([
            log({ domain: undefined, query_type: 'A' }),
            log({ domain: undefined, query_type: 'AAAA' }),
        ]);
        expect(merged).toHaveLength(1);
        expect(merged[0].count).toBe(2);

        const split = consolidateLogs([
            log({ domain: undefined }),
            log({ domain: 'real.com' }),
        ]);
        expect(split).toHaveLength(2);
    });

    it('normalizes case and a trailing dot when comparing domains', () => {
        const groups = consolidateLogs([
            log({ domain: 'Example.com.', query_type: 'A' }),
            log({ domain: 'example.com', query_type: 'AAAA' }),
        ]);
        expect(groups).toHaveLength(1);
        expect(groups[0].count).toBe(2);
    });

    it('preserves order and assigns count 1 to singletons', () => {
        const groups = consolidateLogs([
            log({ domain: 'a.com', query_type: 'A' }),
            log({ domain: 'a.com', query_type: 'AAAA' }),
            log({ domain: 'b.com' }),
        ]);
        expect(groups.map((g) => g.count)).toEqual([2, 1]);
        expect(groups.map((g) => g.representative.dns_request?.domain)).toEqual(['a.com', 'b.com']);
    });

    it('produces distinct, stable keys for non-adjacent groups with the same signature', () => {
        const groups = consolidateLogs([
            log({ domain: 'x.com' }),
            log({ domain: 'y.com' }),
            log({ domain: 'x.com' }),
        ]);
        expect(new Set(groups.map((g) => g.key)).size).toBe(3);
    });

    it('returns [] for an empty input', () => {
        expect(consolidateLogs([])).toEqual([]);
    });

    it('does not merge same-domain entries more than the span window apart', () => {
        // Blocked-filter scenario: two blocks of the same domain 5 minutes apart become adjacent
        // in the filtered stream, but must NOT merge (default 10s window).
        const groups = consolidateLogs([
            log({ domain: 'ads.tracker.com', status: 'blocked', timestamp: '2026-06-15T11:38:09.000Z' }),
            log({ domain: 'ads.tracker.com', status: 'blocked', timestamp: '2026-06-15T11:33:09.000Z' }),
        ]);
        expect(groups).toHaveLength(2);
        expect(groups.every((g) => g.count === 1)).toBe(true);
    });

    it('splits a domain blocked repeatedly over an hour into one row per block', () => {
        const base = Date.parse('2026-06-15T11:38:09.000Z');
        const items = Array.from({ length: 8 }, (_, i) =>
            // ~8 minutes apart, newest first (created-desc).
            log({ domain: 'ads.tracker.com', status: 'blocked', timestamp: new Date(base - i * 8 * 60_000).toISOString() })
        );
        const groups = consolidateLogs(items);
        expect(groups).toHaveLength(8);
        expect(groups.every((g) => g.count === 1)).toBe(true);
    });

    it('measures the span from the run first member, not the previous member', () => {
        // 12:00:00 anchors the run. 11:59:55 is 5s away → merges. 11:59:48 is only 7s from the
        // previous member but 12s from the anchor → it starts a new group.
        const groups = consolidateLogs([
            log({ domain: 'a.com', query_type: 'A', timestamp: '2026-06-15T12:00:00.000Z' }),
            log({ domain: 'a.com', query_type: 'AAAA', timestamp: '2026-06-15T11:59:55.000Z' }),
            log({ domain: 'a.com', query_type: 'A', timestamp: '2026-06-15T11:59:48.000Z' }),
        ]);
        expect(groups.map((g) => g.count)).toEqual([2, 1]);
    });

    it('respects a custom span window', () => {
        const items = [
            log({ domain: 'a.com', query_type: 'A', timestamp: '2026-06-15T12:00:00.000Z' }),
            log({ domain: 'a.com', query_type: 'AAAA', timestamp: '2026-06-15T11:59:30.000Z' }),
        ];
        // 30s apart: outside the default 10s window (2 groups) but inside a 60s window (1 group).
        expect(consolidateLogs(items)).toHaveLength(2);
        expect(consolidateLogs(items, 60_000)).toHaveLength(1);
    });

    it('still merges a sub-second A + AAAA pair', () => {
        const groups = consolidateLogs([
            log({ domain: 'example.com', query_type: 'A', timestamp: '2026-06-15T10:00:00.400Z' }),
            log({ domain: 'example.com', query_type: 'AAAA', timestamp: '2026-06-15T10:00:00.000Z' }),
        ]);
        expect(groups).toHaveLength(1);
        expect(groups[0].count).toBe(2);
    });

    it('toSingletonGroup wraps one log as a count-1 group', () => {
        const g = toSingletonGroup(log({ domain: 'a.com', query_type: 'A' }), 0);
        expect(g.count).toBe(1);
        expect(g.queryTypes).toEqual(['A']);
        expect(g.representative.dns_request?.domain).toBe('a.com');
    });
});
