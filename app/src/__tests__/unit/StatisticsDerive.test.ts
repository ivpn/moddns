import { describe, it, expect } from 'vitest';
import {
    buildBuckets,
    shorterViewForStart,
    countingSince,
    countsState,
    deviceLabel,
    expectedFirstCounts,
    normalizeStats,
    logsScope,
    logsReachBeforeCounts,
    scopeIn,
    scopeTitle,
} from '@/pages/statistics/derive';
import { statsRetentionWords } from '@/components/data-collection/model';
import { LOGS_TIMESPAN, RANGES, offeredRanges, resolveRange, STATS_TIMESPAN, nextLongerRange, parseRange } from '@/pages/statistics/ranges';
import { axisTicks, formatDateTime, bucketUnitWord } from '@/pages/statistics/time';
import { createStatsResponse } from '../mocks/statisticsMocks';

describe('ranges', () => {
    it('offers seven views with full accessible names', () => {
        // tableRef: statistics-behaviour #K1
        expect(RANGES.map(r => r.key)).toEqual(['3h', '6h', '24h', '7d', '30d', '3m', '12m']);
        expect(RANGES.map(r => r.label)).toEqual([
            'Last 3 hours', 'Last 6 hours', 'Last 24 hours', 'Last 7 days', 'Last 30 days', 'Last 3 months', 'Last 12 months',
        ]);
    });

    it('defaults to 24h and reads an invalid value as 24h', () => {
        // tableRef: statistics-behaviour #K2
        expect(parseRange(null)).toBe('24h');
        expect(parseRange('banana')).toBe('24h');
        expect(parseRange('12m')).toBe('12m');
    });

    it('maps views to the statistics timespans', () => {
        // tableRef: statistics-behaviour #K3
        expect(STATS_TIMESPAN).toEqual({
            '3h': 'LAST_3_HOURS', '6h': 'LAST_6_HOURS', '24h': 'LAST_1_DAY', '7d': 'LAST_7_DAYS',
            '30d': 'LAST_MONTH', '3m': 'LAST_3_MONTHS', '12m': 'LAST_YEAR',
        });
    });

    it('maps views to the logs timespans, long views clamp to one month', () => {
        // tableRef: statistics-behaviour #K4
        expect(LOGS_TIMESPAN).toEqual({
            '3h': 'LAST_3_HOURS', '6h': 'LAST_6_HOURS', '24h': 'LAST_1_DAY', '7d': 'LAST_7_DAYS',
            '30d': 'LAST_MONTH', '3m': 'LAST_MONTH', '12m': 'LAST_MONTH',
        });
    });

    it('suggests the next longer view', () => {
        expect(nextLongerRange('3h')).toBe('6h');
        expect(nextLongerRange('12m')).toBeNull();
    });
});

describe('normalizeStats', () => {
    it('tolerates an empty or non-object answer', () => {
        expect(normalizeStats({}).enabled).toBe(false);
        expect(normalizeStats([]).series).toEqual([]);
        expect(normalizeStats(null).totals).toEqual({ total: 0, blocked: 0, dnssec: 0 });
    });
});

describe('buildBuckets', () => {
    it('keeps the full view and turns points before the start into gaps, not zeros', () => {
        // tableRef: statistics-behaviour #P9
        const d = normalizeStats(createStatsResponse({ points: 6, enabledAt: '2026-10-06T13:20:00Z' }));
        const b = buildBuckets(d);
        expect(b).toHaveLength(d.series.length);
        expect(b[0].total).toBeNull();
        expect(b.filter(x => x.total === null).length).toBeGreaterThan(0);
        expect(b[b.length - 1].total).not.toBeNull();
        expect(countingSince(d)).toBe(Date.parse('2026-10-06T13:20:00Z'));
    });

    it('uses history_deleted_at as the start when it is later than enabled_at', () => {
        // tableRef: statistics-behaviour #P9
        const raw = { ...createStatsResponse({ points: 8, enabledAt: '2026-09-14T09:12:00Z' }), history_deleted_at: '2026-10-06T14:10:00Z' };
        const d = normalizeStats(raw);
        const firstCounted = buildBuckets(d).find(x => x.total !== null)!;
        expect(firstCounted.ts).toBe(Date.parse('2026-10-06T14:00:00Z'));
        expect(buildBuckets(d)).toHaveLength(8);
    });

    it('marks the not-yet-flushed tail as in progress', () => {
        // tableRef: statistics-behaviour #P10
        const d = normalizeStats(createStatsResponse({ points: 6 }));
        const b = buildBuckets(d);
        expect(b[b.length - 1].inProgress).toBe(true);
        expect(b[0].inProgress).toBe(false);
    });

    it('has no gaps when statistics were enabled before the range', () => {
        const d = normalizeStats(createStatsResponse({ points: 6 }));
        expect(buildBuckets(d).every(b => b.total !== null)).toBe(true);
        expect(countingSince(d)).toBeNull();
    });
});

describe('countsState', () => {
    it('is no-queries-yet when enabled inside the range with nothing counted', () => {
        // tableRef: statistics-behaviour #P5
        const d = normalizeStats(createStatsResponse({ empty: true, points: 4, enabledAt: '2026-10-06T14:50:00Z' }));
        expect(countsState(d)).toBe('no-queries-yet');
    });

    it('is empty-range when enabled before the range with nothing counted', () => {
        // tableRef: statistics-behaviour #P6
        const d = normalizeStats(createStatsResponse({ empty: true, points: 4 }));
        expect(countsState(d)).toBe('empty-range');
        expect(countsState(normalizeStats(createStatsResponse({ empty: true, points: 4, enabledAt: null })))).toBe('empty-range');
    });

    it('is data when something was counted', () => {
        expect(countsState(normalizeStats(createStatsResponse()))).toBe('data');
    });
});

describe('expectedFirstCounts', () => {
    const at = (iso: string) => Date.parse(iso);
    const iso = (ms: number | undefined) => (ms === undefined ? null : new Date(ms).toISOString());

    it('after turning on, every view waits for the next quarter hour and shows boundary + 1 min', () => {
        // tableRef: statistics-behaviour #P24
        for (const r of ['3h', '6h', '24h', '7d', '30d', '3m', '12m'] as const) {
            const e = expectedFirstCounts(r, at('2026-10-06T10:43:00Z'), null);
            expect(iso(e?.boundaryMs)).toBe('2026-10-06T10:45:00.000Z');
            expect(iso(e?.displayMs)).toBe('2026-10-06T10:46:00.000Z');
            expect(iso(e?.collectingUntilMs)).toBe('2026-10-06T10:48:00.000Z');
        }
    });

    it('turning on exactly on a boundary waits for the next one', () => {
        // tableRef: statistics-behaviour #P24
        expect(iso(expectedFirstCounts('3h', at('2026-10-06T10:45:00Z'), null)?.boundaryMs)).toBe('2026-10-06T11:00:00.000Z');
    });

    it('uses the later of enabled_at and history_deleted_at, and enabled_at wins when later', () => {
        // tableRef: statistics-behaviour #P24
        expect(iso(expectedFirstCounts('7d', at('2026-10-06T10:43:00Z'), at('2026-10-06T09:00:00Z'))?.boundaryMs)).toBe('2026-10-06T10:45:00.000Z');
        expect(expectedFirstCounts('7d', null, null)).toBeNull();
    });

    it('after Delete history the tiers differ: quarter, hour + 15 min, UTC day + 15 min', () => {
        // tableRef: statistics-behaviour #P24
        const en = at('2026-09-14T09:12:00Z');
        const del = at('2026-10-06T10:43:00Z');
        expect(iso(expectedFirstCounts('3h', en, del)?.boundaryMs)).toBe('2026-10-06T10:45:00.000Z');
        expect(iso(expectedFirstCounts('6h', en, del)?.boundaryMs)).toBe('2026-10-06T10:45:00.000Z');
        expect(iso(expectedFirstCounts('24h', en, del)?.boundaryMs)).toBe('2026-10-06T11:15:00.000Z');
        expect(iso(expectedFirstCounts('7d', en, del)?.boundaryMs)).toBe('2026-10-06T11:15:00.000Z');
        for (const r of ['30d', '3m', '12m'] as const) {
            expect(iso(expectedFirstCounts(r, en, del)?.boundaryMs)).toBe('2026-10-07T00:15:00.000Z');
            expect(iso(expectedFirstCounts(r, en, del)?.displayMs)).toBe('2026-10-07T00:16:00.000Z');
        }
    });

    it('after a quick off and on, the buckets open at the turn-off wait like a history delete', () => {
        // tableRef: statistics-behaviour #P24
        // J53, G22: history_deleted_at = the turn-off, just before enabled_at.
        const sameQuarter = { en: at('2026-10-06T10:44:00Z'), off: at('2026-10-06T10:41:00Z') };
        expect(iso(expectedFirstCounts('3h', sameQuarter.en, sameQuarter.off)?.boundaryMs)).toBe('2026-10-06T10:45:00.000Z');
        expect(iso(expectedFirstCounts('24h', sameQuarter.en, sameQuarter.off)?.boundaryMs)).toBe('2026-10-06T11:15:00.000Z');
        expect(iso(expectedFirstCounts('30d', sameQuarter.en, sameQuarter.off)?.boundaryMs)).toBe('2026-10-07T00:15:00.000Z');
        // Off in the previous hour: the hour of the turn-on holds no older counts, the day still does.
        const prevHour = { en: at('2026-10-06T10:04:00Z'), off: at('2026-10-06T09:58:00Z') };
        expect(iso(expectedFirstCounts('3h', prevHour.en, prevHour.off)?.boundaryMs)).toBe('2026-10-06T10:15:00.000Z');
        expect(iso(expectedFirstCounts('24h', prevHour.en, prevHour.off)?.boundaryMs)).toBe('2026-10-06T10:15:00.000Z');
        expect(iso(expectedFirstCounts('30d', prevHour.en, prevHour.off)?.boundaryMs)).toBe('2026-10-07T00:15:00.000Z');
    });

    it('computes UTC day edges from instants, independent of the local timezone', () => {
        // tableRef: statistics-behaviour #P24
        // 23:50 at UTC-5 is 04:50 UTC the next day; the next UTC day starts on the 8th.
        expect(iso(expectedFirstCounts('30d', null, at('2026-10-06T23:50:00-05:00'))?.boundaryMs)).toBe('2026-10-08T00:15:00.000Z');
        // 01:10 at UTC+2 is 23:10 UTC the previous day.
        expect(iso(expectedFirstCounts('12m', null, at('2026-10-07T01:10:00+02:00'))?.boundaryMs)).toBe('2026-10-07T00:15:00.000Z');
        expect(iso(expectedFirstCounts('24h', null, at('2026-10-06T23:50:00Z'))?.boundaryMs)).toBe('2026-10-07T00:15:00.000Z');
    });
});

describe('labels', () => {
    it('labels devices', () => {
        // tableRef: statistics-behaviour #P20
        expect(deviceLabel('laptop')).toMatchObject({ kind: 'id', text: 'laptop' });
        expect(deviceLabel('')).toMatchObject({ kind: 'none', text: 'No device ID' });
        expect(deviceLabel('_other')).toMatchObject({ kind: 'other', text: 'Other devices' });
    });

    it('reads retention as words, defaulting to 30 days', () => {
        // tableRef: statistics-behaviour #P13
        expect(statsRetentionWords('30d')).toBe('30 days');
        expect(statsRetentionWords('1y')).toBe('1 year');
        expect(statsRetentionWords('')).toBe('30 days');
    });

    it('words the bucket resolution', () => {
        // tableRef: statistics-behaviour #K6
        expect(bucketUnitWord(900)).toBe('15-minute');
        expect(bucketUnitWord(3600)).toBe('hourly');
        expect(bucketUnitWord(86400)).toBe('daily');
    });
});

describe('axisTicks', () => {
    const every = (startMs: number, count: number, stepMs: number) => Array.from({ length: count }, (_, i) => startMs + i * stepMs);
    const QUARTER = 15 * 60_000;
    const HOUR = 3_600_000;
    const DAY = 86_400_000;
    const hm = (ms: number) => new Date(ms).getHours() * 60 + new Date(ms).getMinutes();

    it('3h: every 30 minutes on the local clock', () => {
        // tableRef: statistics-behaviour #K6
        const ts = every(new Date(2026, 9, 8, 10, 0).getTime(), 12, QUARTER);
        const t = axisTicks('3h', ts);
        expect(t.map(x => hm(x.ts))).toEqual([600, 630, 660, 690, 720, 750]);
        expect(t[0].label).toBe(new Intl.DateTimeFormat(undefined, { hour: '2-digit', minute: '2-digit' }).format(ts[0]));
    });

    it('6h: every hour', () => {
        // tableRef: statistics-behaviour #K6
        const ts = every(new Date(2026, 9, 8, 6, 0).getTime(), 24, QUARTER);
        expect(axisTicks('6h', ts).map(x => hm(x.ts))).toEqual([360, 420, 480, 540, 600, 660]);
    });

    it('24h: every 3 hours, the date at local midnight', () => {
        // tableRef: statistics-behaviour #K6
        const ts = every(new Date(2026, 9, 7, 18, 0).getTime(), 24, HOUR);
        const t = axisTicks('24h', ts);
        expect(t.map(x => new Date(x.ts).getHours())).toEqual([18, 21, 0, 3, 6, 9, 12, 15]);
        const midnight = t.find(x => new Date(x.ts).getHours() === 0)!;
        expect(midnight.label).toBe(new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric' }).format(midnight.ts));
        expect(t[0].label).toBe(new Intl.DateTimeFormat(undefined, { hour: '2-digit', minute: '2-digit' }).format(t[0].ts));
    });

    it('7d: every local midnight labelled weekday and day', () => {
        // tableRef: statistics-behaviour #K6
        const ts = every(new Date(2026, 9, 1, 12, 0).getTime(), 7 * 24, HOUR);
        const t = axisTicks('7d', ts);
        expect(t).toHaveLength(7);
        expect(t.every(x => new Date(x.ts).getHours() === 0)).toBe(true);
        expect(t[0].label).toBe(new Intl.DateTimeFormat(undefined, { weekday: 'short', day: 'numeric' }).format(t[0].ts));
    });

    const fmtDay = (ms: number, year = false) =>
        new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric', year: year ? 'numeric' : undefined, timeZone: 'UTC' }).format(ms);

    it('a 30-day span has 6 to 10 labels on desktop and at least 4 at 375px', () => {
        // tableRef: statistics-behaviour #K6
        const ts = every(Date.UTC(2026, 8, 9), 30, DAY);
        const desktop = axisTicks('30d', ts, 950);
        expect(desktop.length).toBeGreaterThanOrEqual(6);
        expect(desktop.length).toBeLessThanOrEqual(10);
        const phone = axisTicks('30d', ts, 309);
        expect(phone.length).toBeGreaterThanOrEqual(4);
        expect(phone.length).toBeLessThanOrEqual(5);
        expect(desktop[0].label).toBe(fmtDay(Date.UTC(2026, 8, 9)));
    });

    it('day steps count from the first bucket on UTC days', () => {
        // tableRef: statistics-behaviour #K6
        const t = axisTicks('30d', every(Date.UTC(2026, 8, 9), 30, DAY), 950);
        const gaps = t.slice(1).map((x, i) => (x.ts - t[i].ts) / DAY);
        expect(new Set(gaps).size).toBe(1);
        expect(t[0].ts).toBe(Date.UTC(2026, 8, 9));
    });

    it('a 12m view clamped to 30 days behaves like the 30d view', () => {
        // tableRef: statistics-behaviour #K6
        const ts = every(Date.UTC(2026, 8, 9), 30, DAY);
        expect(axisTicks('12m', ts, 950)).toEqual(axisTicks('30d', ts, 950));
        expect(axisTicks('3m', ts, 309)).toEqual(axisTicks('30d', ts, 309));
    });

    it('a 90-day span steps by Mondays or more and keeps 4 or more labels', () => {
        // tableRef: statistics-behaviour #K6
        const t = axisTicks('3m', every(Date.UTC(2026, 6, 11), 90, DAY), 950);
        expect(t.length).toBeGreaterThanOrEqual(6);
        expect(t.length).toBeLessThanOrEqual(10);
        expect(t.every(x => new Date(x.ts).getUTCDay() === 1)).toBe(true);
        expect(axisTicks('3m', every(Date.UTC(2026, 6, 11), 90, DAY), 309).length).toBeGreaterThanOrEqual(4);
    });

    it('a full year uses month starts with the full year, never a two-digit year', () => {
        // tableRef: statistics-behaviour #K6
        const ts = every(Date.UTC(2025, 9, 9), 365, DAY);
        const t = axisTicks('12m', ts, 950);
        expect(t.length).toBeGreaterThanOrEqual(6);
        expect(t.length).toBeLessThanOrEqual(10);
        expect(t.every(x => new Date(x.ts).getUTCDate() === 1)).toBe(true);
        expect(t.every(x => /\b20\d\d\b/.test(x.label))).toBe(true);
        expect(t[0].label).toBe(new Intl.DateTimeFormat(undefined, { month: 'short', year: 'numeric', timeZone: 'UTC' }).format(t[0].ts));
        expect(axisTicks('12m', ts, 309).length).toBeGreaterThanOrEqual(4);
    });

    it('shows the year on the first label and on Jan 1 when the span crosses a year', () => {
        // tableRef: statistics-behaviour #K6
        const t = axisTicks('30d', every(Date.UTC(2026, 11, 18), 30, DAY), 950);
        expect(t[0].label).toBe(fmtDay(t[0].ts, true));
        const jan = t.find(x => x.ts === Date.UTC(2027, 0, 1));
        if (jan) expect(jan.label).toBe(fmtDay(jan.ts, true));
        expect(t.some(x => new Date(x.ts).getUTCFullYear() === 2027)).toBe(true);
        const noCross = axisTicks('30d', every(Date.UTC(2026, 8, 9), 30, DAY), 950);
        expect(noCross.every(x => !/2026/.test(x.label))).toBe(true);
    });

    it('short spans keep one label per day', () => {
        // tableRef: statistics-behaviour #K6
        expect(axisTicks('30d', every(Date.UTC(2026, 9, 6), 3, DAY), 950)).toHaveLength(3);
        expect(axisTicks('30d', [], 950)).toEqual([]);
    });

    it('keeps at most every other tick on narrow screens', () => {
        // tableRef: statistics-behaviour #K6
        const ts = every(new Date(2026, 9, 8, 6, 0).getTime(), 24, QUARTER);
        expect(axisTicks('6h', ts, 300).map(x => hm(x.ts))).toEqual([360, 480, 600]);
    });

    it('labels daily ticks by UTC date whatever the process timezone', () => {
        // tableRef: statistics-behaviour #K6
        const before = process.env.TZ;
        process.env.TZ = 'Pacific/Auckland';
        try {
            const t = axisTicks('30d', every(Date.UTC(2026, 8, 9), 30, DAY), 950);
            expect(t[0].ts).toBe(Date.UTC(2026, 8, 9));
            expect(t[0].label).toBe(new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric', timeZone: 'UTC' }).format(Date.UTC(2026, 8, 9)));
        } finally {
            if (before === undefined) delete process.env.TZ;
            else process.env.TZ = before;
        }
    });
});

describe('offeredRanges and resolveRange', () => {
    it('offers the views whose window fits the retention, shortest first', () => {
        // tableRef: statistics-behaviour #K11
        expect(offeredRanges('30d').map(r => r.key)).toEqual(['3h', '6h', '24h', '7d', '30d']);
        expect(offeredRanges('90d').map(r => r.key)).toEqual(['3h', '6h', '24h', '7d', '30d', '3m']);
        expect(offeredRanges('1y').map(r => r.key)).toEqual(['3h', '6h', '24h', '7d', '30d', '3m', '12m']);
    });

    it('reads a hidden view as the longest offered one and an invalid value as 24h', () => {
        // tableRef: statistics-behaviour #K2, #K11
        expect(resolveRange('12m', '30d')).toBe('30d');
        expect(resolveRange('3m', '30d')).toBe('30d');
        expect(resolveRange('12m', '90d')).toBe('3m');
        expect(resolveRange('12m', '1y')).toBe('12m');
        expect(resolveRange('bogus', '30d')).toBe('24h');
        expect(resolveRange(null, '1y')).toBe('24h');
    });
});

describe('shorterViewForStart', () => {
    const offered = RANGES.filter(r => r.seconds <= 30 * 86400);
    const d = (enabledAt: string, points = 168) => normalizeStats(createStatsResponse({ points, enabledAt }));

    it('points at the shortest offered view that still contains the start, only under a quarter', () => {
        // tableRef: statistics-behaviour #P9
        expect(shorterViewForStart(d('2026-10-06T14:20:00Z'), '7d', offered)).toBe('3h');
        expect(shorterViewForStart(d('2026-10-06T10:00:00Z'), '7d', offered)).toBe('6h');
        expect(shorterViewForStart(d('2026-10-05T20:00:00Z'), '7d', offered)).toBe('24h');
        expect(shorterViewForStart(d('2026-10-04T15:00:00Z'), '7d', offered)).toBeNull();
    });

    it('offers nothing without a start inside the view or without a shorter view', () => {
        // tableRef: statistics-behaviour #P9
        expect(shorterViewForStart(d('2026-09-01T00:00:00Z'), '7d', offered)).toBeNull();
        expect(shorterViewForStart(d('2026-10-06T14:20:00Z'), '3h', offered)).toBeNull();
    });
});

describe('axis tick minimum', () => {
    const every = (startMs: number, count: number, stepMs: number) => Array.from({ length: count }, (_, i) => startMs + i * stepMs);
    const HOUR = 3_600_000;

    it('7d falls back from midnights to finer steps when the span is short', () => {
        // tableRef: statistics-behaviour #K12
        const short = every(new Date(2026, 9, 8, 9, 0).getTime(), 10, HOUR);
        const t = axisTicks('7d', short, 950);
        expect(t.length).toBeGreaterThanOrEqual(3);
        const usual = axisTicks('7d', every(new Date(2026, 9, 1, 12, 0).getTime(), 168, HOUR), 950);
        expect(usual.every(x => new Date(x.ts).getHours() === 0)).toBe(true);
    });

    it('24h falls back from 3 hours to 1 hour', () => {
        // tableRef: statistics-behaviour #K12
        const t = axisTicks('24h', every(new Date(2026, 9, 8, 9, 0).getTime(), 4, HOUR), 950);
        expect(t.length).toBeGreaterThanOrEqual(3);
    });

    it('3h and 6h fall back to quarter hours', () => {
        // tableRef: statistics-behaviour #K12
        const q = every(new Date(2026, 9, 8, 10, 0).getTime(), 4, 900_000);
        expect(axisTicks('3h', q, 950).length).toBeGreaterThanOrEqual(3);
        expect(axisTicks('6h', q, 950).length).toBeGreaterThanOrEqual(3);
    });

    it('never returns fewer than two labels for two or more buckets, also on narrow screens', () => {
        // tableRef: statistics-behaviour #K12
        const two = every(new Date(2026, 9, 8, 9, 0).getTime(), 2, HOUR);
        for (const r of ['3h', '6h', '24h', '7d'] as const) expect(axisTicks(r, two, 309).length).toBeGreaterThanOrEqual(2);
        for (const r of ['30d', '3m', '12m'] as const) expect(axisTicks(r, [Date.UTC(2026, 9, 5), Date.UTC(2026, 9, 6)], 309)).toHaveLength(2);
        expect(axisTicks('7d', every(new Date(2026, 9, 8, 9, 0).getTime(), 12, HOUR), 309).length).toBeGreaterThanOrEqual(3);
    });
});

describe('group scopes', () => {
    const NOW = Date.parse('2026-10-12T12:00:00Z');
    const H = 3600_000;
    const D = 24 * H;

    it('names the logs window as the shorter of the view and the logs retention', () => {
        // tableRef: statistics-behaviour #P18
        const words = (range: Parameters<typeof logsScope>[0], retention: string | undefined) => scopeTitle(logsScope(range, retention, null, NOW));
        expect(words('7d', '1h')).toBe('Last hour');
        expect(words('24h', '6h')).toBe('Last 6 hours');
        expect(words('7d', '1d')).toBe('Last 24 hours');
        expect(words('3h', '1d')).toBe('Last 3 hours');
        expect(words('7d', '1w')).toBe('Last 7 days');
        expect(words('30d', '1m')).toBe('Last 30 days');
        expect(words('12m', '1m')).toBe('Last 30 days');
        expect(words('7d', undefined)).toBe('Last hour');
    });

    it('says since when the oldest log row starts well inside the logs window', () => {
        // tableRef: statistics-behaviour #P18
        expect(logsScope('7d', '1w', NOW - 7 * D + H, NOW)).toEqual({ kind: 'last', words: 'last 7 days' });
        const at = NOW - 2 * D;
        expect(logsScope('7d', '1w', at, NOW)).toEqual({ kind: 'since', at });
        expect(logsScope('7d', '1h', NOW - 58 * 60_000, NOW).kind).toBe('last');
        expect(logsScope('7d', '1h', NOW - 30 * 60_000, NOW).kind).toBe('since');
    });

    it('phrases a scope as a title and inside a sentence', () => {
        // tableRef: statistics-behaviour #P18, #P27
        expect(scopeTitle({ kind: 'last', words: 'last hour' })).toBe('Last hour');
        expect(scopeIn({ kind: 'last', words: 'last hour' })).toBe('in the last hour');
        const at = NOW - 2 * D;
        expect(scopeTitle({ kind: 'since', at })).toBe(`Since ${formatDateTime(at)}`);
        expect(scopeIn({ kind: 'since', at })).toBe(`since ${formatDateTime(at)}`);
    });

    it('flags logs that reach further back than statistics', () => {
        // tableRef: statistics-behaviour #P28
        const start = NOW - D;
        expect(logsReachBeforeCounts('7d', '1w', NOW - 3 * D, start)).toBe(true);
        expect(logsReachBeforeCounts('7d', '1w', start - H, start)).toBe(false);
        expect(logsReachBeforeCounts('7d', '1w', null, start)).toBe(false);
        expect(logsReachBeforeCounts('7d', '1w', NOW - 3 * D, null)).toBe(false);
    });
});
