import { describe, it, expect } from 'vitest';
import {
    buildBuckets,
    countingSince,
    countsState,
    deviceLabel,
    isClamped,
    normalizeStats,
    statsRetentionWords,
} from '@/pages/statistics/derive';
import { LOGS_TIMESPAN, RANGES, STATS_TIMESPAN, logsWindowCaption, nextLongerRange, parseRange } from '@/pages/statistics/ranges';
import { formatBucketTick, formatRangeCaption, bucketUnitWord } from '@/pages/statistics/time';
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

    it('labels daily ticks by UTC date regardless of the browser zone', () => {
        // tableRef: statistics-behaviour #K6
        expect(formatBucketTick(Date.UTC(2026, 8, 30, 0), 86400)).toBe(
            new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric', timeZone: 'UTC' }).format(Date.UTC(2026, 8, 30)),
        );
    });
});
