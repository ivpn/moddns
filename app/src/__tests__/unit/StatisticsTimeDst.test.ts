import { afterAll, describe, expect, it } from 'vitest';
import { formatBucketLabel, formatClock, formatDateTime } from '@/pages/statistics/time';

// Set before time.ts creates (and caches) its formatters, which keep the zone they were built in.
const before = process.env.TZ;
process.env.TZ = 'Europe/Berlin';

afterAll(() => {
    if (before === undefined) delete process.env.TZ;
    else process.env.TZ = before;
});

const zoned = (ms: number, withDate: boolean) =>
    new Intl.DateTimeFormat(undefined, {
        month: withDate ? 'short' : undefined,
        day: withDate ? 'numeric' : undefined,
        hour: '2-digit',
        minute: '2-digit',
        timeZoneName: 'short',
        timeZone: 'Europe/Berlin',
    }).format(ms);

describe('formatBucketLabel across a clock change', () => {
    // Berlin leaves CEST at 2026-10-25 01:00 UTC, so 02:00-03:00 local time happens twice.
    const cestTwo = Date.UTC(2026, 9, 25, 0); // 02:00 CEST
    const cetTwo = Date.UTC(2026, 9, 25, 1); // 02:00 CET
    const cetThree = Date.UTC(2026, 9, 25, 2); // 03:00 CET

    it('names the zone on each repeated time, so the two 02:00 hours differ', () => {
        // tableRef: statistics-behaviour #K6
        const first = formatBucketLabel(cestTwo, 3600);
        const second = formatBucketLabel(cetTwo, 3600);
        expect(first).toBe(`${zoned(cestTwo, true)}-${zoned(cetTwo, false)}`);
        expect(second.startsWith(`${zoned(cetTwo, true)}-`)).toBe(true);
        expect(first).not.toBe(second);
    });

    it('leaves the times that occur once as they are', () => {
        // tableRef: statistics-behaviour #K6
        expect(formatBucketLabel(cetTwo, 3600)).toBe(`${zoned(cetTwo, true)}-${formatClock(cetThree)}`);
        const noon = Date.UTC(2026, 9, 20, 10);
        expect(formatBucketLabel(noon, 3600)).toBe(`${formatDateTime(noon)}-${formatClock(noon + 3600_000)}`);
    });

    it('adds no zone when clocks go forward, as no local time repeats', () => {
        // tableRef: statistics-behaviour #K6
        // Berlin enters CEST at 2026-03-29 01:00 UTC.
        for (const [ms, size] of [[Date.UTC(2026, 2, 29, 0), 3600], [Date.UTC(2026, 2, 29, 1), 900]]) {
            expect(formatBucketLabel(ms, size)).toBe(`${formatDateTime(ms)}-${formatClock(ms + size * 1000)}`);
        }
    });
});
