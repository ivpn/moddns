import { describe, it, expect } from 'vitest';
import {
    buildBuckets,
    countingSince,
    countsState,
    deviceLabel,
    expectedFirstCounts,
    isClamped,
    normalizeStats,
} from '@/pages/statistics/derive';
import { statsRetentionWords } from '@/components/data-collection/model';
import { LOGS_TIMESPAN, RANGES, STATS_TIMESPAN, logsWindowCaption, nextLongerRange, parseRange } from '@/pages/statistics/ranges';
import { axisTicks, formatRangeCaption, bucketUnitWord } from '@/pages/statistics/time';
import { createStatsResponse } from '../mocks/statisticsMocks';

describe('ranges', () => {
    it('offers seven views with full accessible names', () => {
        // tableRef: statistics-behaviour #K1
        expect(RANGES.map(r => r.key)).toEqual(['3h', '6h', '24h', '7d', '30d', '3m', '12m']);
        expect(RANGES.map(r => r.label)).toEqual([
            'Last 3 hours', 'Last 6 hours', 'Last 24 hours', 'Last 7 days', 'Last 30 days', 'Last 3 months', 'Last 12 months',
        ]);
    });

    it('defaults to 7d and reads an invalid value as 7d', () => {
        // tableRef: statistics-behaviour #K2
        expect(parseRange(null)).toBe('7d');
        expect(parseRange('banana')).toBe('7d');
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

    it('captions the logs window only when retention is shorter than the view', () => {
        // tableRef: statistics-behaviour #P18
        expect(logsWindowCaption('7d', '1h')).toBe('From the last 1 hour of query logs');
        expect(logsWindowCaption('3h', '1d')).toBeNull();
        expect(logsWindowCaption('30d', '1m')).toBeNull();
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
    it('turns points before the enabling instant into gaps, not zeros', () => {
        // tableRef: statistics-behaviour #P9
        const raw = createStatsResponse({ points: 6, enabledAt: '2026-10-06T13:20:00Z' });
        const d = normalizeStats(raw);
        const b = buildBuckets(d);
        const nulls = b.filter(x => x.total === null).length;
        expect(nulls).toBeGreaterThan(0);
        expect(b[0].total).toBeNull();
        expect(b[b.length - 1].total).not.toBeNull();
        expect(countingSince(d)).toBe(Date.parse('2026-10-06T13:20:00Z'));
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

describe('isClamped', () => {
    it('detects a view shorter than requested', () => {
        // tableRef: statistics-behaviour #P11
        const d = normalizeStats(createStatsResponse({ points: 30, bucketSeconds: 86400 }));
        expect(isClamped(d, '3m')).toBe(true);
        expect(isClamped(d, '30d')).toBe(false);
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
        // tableRef: statistics-behaviour #K5
        expect(bucketUnitWord(900)).toBe('15-minute');
        expect(bucketUnitWord(3600)).toBe('hourly');
        expect(bucketUnitWord(86400)).toBe('daily');
        expect(formatRangeCaption(Date.UTC(2026, 8, 29, 15), Date.UTC(2026, 9, 6, 15), 3600)).toMatch(/ · hourly$/);
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
