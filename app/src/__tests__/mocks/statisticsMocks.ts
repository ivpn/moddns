// Fixtures for the Statistics page: one deterministic dataset for unit and E2E tests.

export interface StatsFixtureOptions {
    /** "Now" of the response; defaults to a fixed instant so snapshots are stable. */
    to?: Date;
    bucketSeconds?: number;
    points?: number;
    enabled?: boolean;
    enabledAt?: string | null;
    retention?: string;
    /** Zero every counter (no queries yet / empty range). */
    empty?: boolean;
    blocked?: boolean;
    timespan?: string;
}

export const FIXED_TO = new Date('2026-10-06T15:20:00Z');

export function createStatsResponse(opts: StatsFixtureOptions = {}) {
    const {
        to = FIXED_TO,
        bucketSeconds = 3600,
        points = 24,
        enabled = true,
        enabledAt = '2026-09-14T09:12:00Z',
        retention = '30d',
        empty = false,
        blocked = true,
        timespan = 'LAST_7_DAYS',
    } = opts;
    const width = bucketSeconds * 1000;
    const end = Math.floor(to.getTime() / width) * width + width;
    const series = Array.from({ length: points }, (_, i) => {
        const ts = end - (points - i) * width;
        const total = empty ? 0 : 100 + ((i * 37) % 80);
        const blk = empty || !blocked ? 0 : Math.floor(total / 10);
        return { ts: new Date(ts).toISOString(), total, blocked: blk, dnssec: empty ? 0 : Math.floor(total * 0.7) };
    });
    const sum = (k: 'total' | 'blocked' | 'dnssec') => series.reduce((s, p) => s + p[k], 0);
    const totals = { total: sum('total'), blocked: sum('blocked'), dnssec: sum('dnssec') };
    return {
        enabled,
        enabled_at: enabled ? enabledAt : null,
        retention,
        timespan,
        from: new Date(end - points * width).toISOString(),
        to: to.toISOString(),
        bucket_seconds: bucketSeconds,
        totals,
        series,
        reasons: empty || !blocked
            ? { blocklist: 0, service: 0, custom_rule: 0, rebinding: 0, default_rule: 0, other: 0 }
            : { blocklist: Math.floor(totals.blocked * 0.6), service: Math.floor(totals.blocked * 0.2), custom_rule: Math.floor(totals.blocked * 0.1), rebinding: 0, default_rule: Math.floor(totals.blocked * 0.1), other: 0 },
        protocols: empty ? { doh: 0, dot: 0, doq: 0 } : { doh: Math.floor(totals.total * 0.6), dot: Math.floor(totals.total * 0.35), doq: Math.floor(totals.total * 0.05) },
        devices: empty
            ? []
            : [
                  { device_id: 'laptop', total: Math.floor(totals.total * 0.5), blocked: Math.floor(totals.blocked * 0.5) },
                  { device_id: 'phone', total: Math.floor(totals.total * 0.3), blocked: Math.floor(totals.blocked * 0.3) },
                  { device_id: '', total: Math.floor(totals.total * 0.15), blocked: Math.floor(totals.blocked * 0.15) },
                  { device_id: '_other', total: Math.floor(totals.total * 0.05), blocked: Math.floor(totals.blocked * 0.05) },
              ],
    };
}

export const topBlocked = {
    enabled: true,
    items: [
        { domain: 'ads.example.com', count: 120 },
        { domain: 'tracker.example.net', count: 80 },
    ],
};

export const topResolved = {
    enabled: true,
    items: [
        { domain: 'example.org', count: 400 },
        { domain: 'cdn.example.com', count: 250 },
    ],
};

export const topClients = {
    enabled: true,
    items: [
        { ip: '203.0.113.7', count: 300, asn: 64500, as_org: 'Example ISP', country: 'PL' },
        { ip: '198.51.100.2', count: 120, asn: null, as_org: null, country: null },
    ],
};

export const topBlocklists = {
    enabled: true,
    items: [
        { blocklist_id: 'bl-basic', name: 'Basic Protection', count: 90 },
        { blocklist_id: 'bl-nameless', count: 12 },
    ],
};

export const devicesList = [
    { device_id: 'laptop', last_seen: '2026-10-06T15:00:00Z' },
    { device_id: 'phone', last_seen: '2026-10-06T10:00:00Z' },
];
