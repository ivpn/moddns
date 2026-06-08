import { describe, it, expect } from 'vitest';
import { computeAnnouncementsIndicator } from '@/hooks/useAnnouncementsIndicator';
import type { AnnouncementsAnnouncement } from '@/api/client';

const a = (id: string, published_at: string, severity: 'info' | 'warning' | 'critical'): AnnouncementsAnnouncement => ({
    id,
    published_at,
    severity,
    category: 'news',
    title: id,
});

// Fixed reference "now" so the 48h clean-browser window is deterministic.
const NOW = new Date('2026-06-08T00:00:00Z').getTime();
const HOUR = 60 * 60 * 1000;
const hoursAgo = (h: number) => new Date(NOW - h * HOUR).toISOString();
const daysAgo = (d: number) => hoursAgo(d * 24);

describe('computeAnnouncementsIndicator', () => {
    describe('clean browser (null lastSeen) — 48h recency floor', () => {
        it('shows a dot only for announcements published within the last 48h', () => {
            const items = [a('recent', hoursAgo(10), 'info'), a('old', daysAgo(5), 'warning')];
            const r = computeAnnouncementsIndicator(items, null, NOW);
            expect(r.hasUnread).toBe(true); // "recent" is within 48h
        });

        it('shows no dot when every announcement is older than 48h', () => {
            const items = [a('old', daysAgo(3), 'info'), a('older', daysAgo(10), 'warning')];
            expect(computeAnnouncementsIndicator(items, null, NOW)).toEqual({
                hasUnread: false,
                hasCriticalUnread: false,
            });
        });

        it('surfaces a recent critical to a fresh browser (no carve-out needed)', () => {
            const items = [a('incident', hoursAgo(6), 'critical'), a('old', daysAgo(4), 'info')];
            expect(computeAnnouncementsIndicator(items, null, NOW)).toEqual({
                hasUnread: true,
                hasCriticalUnread: true,
            });
        });

        it('does not flag an old critical (published >48h ago) on a clean browser', () => {
            const items = [a('incident', daysAgo(4), 'critical')];
            expect(computeAnnouncementsIndicator(items, null, NOW)).toEqual({
                hasUnread: false,
                hasCriticalUnread: false,
            });
        });
    });

    describe('established user (non-null lastSeen) — unbounded "newer than last-seen"', () => {
        const items = [
            a('old', '2026-05-01T00:00:00Z', 'info'),
            a('new', '2026-05-20T00:00:00Z', 'warning'),
        ];

        it('only counts announcements published after lastSeen', () => {
            const r = computeAnnouncementsIndicator(items, '2026-05-10T00:00:00Z', NOW);
            expect(r.hasUnread).toBe(true); // "new" is after lastSeen
        });

        it('clears when lastSeen is after the newest announcement', () => {
            const r = computeAnnouncementsIndicator(items, '2026-06-01T00:00:00Z', NOW);
            expect(r.hasUnread).toBe(false);
            expect(r.hasCriticalUnread).toBe(false);
        });

        it('still flags an unread item published >48h ago (user was away a week)', () => {
            // lastSeen 8 days ago; item published 5 days ago is after lastSeen but well
            // outside the 48h window — the 48h floor must NOT apply here.
            const away = [a('while-away', daysAgo(5), 'warning')];
            const r = computeAnnouncementsIndicator(away, daysAgo(8), NOW);
            expect(r.hasUnread).toBe(true);
        });

        it('flags critical only when an unread announcement is critical', () => {
            const withCritical = [...items, a('incident', '2026-05-25T00:00:00Z', 'critical')];
            // lastSeen before the critical one -> critical unread
            expect(computeAnnouncementsIndicator(withCritical, '2026-05-21T00:00:00Z', NOW)).toEqual({
                hasUnread: true,
                hasCriticalUnread: true,
            });
            // lastSeen after the critical one -> no critical unread
            expect(computeAnnouncementsIndicator(withCritical, '2026-05-26T00:00:00Z', NOW)).toEqual({
                hasUnread: false,
                hasCriticalUnread: false,
            });
        });
    });

    it('returns no unread for an empty list', () => {
        expect(computeAnnouncementsIndicator([], null, NOW)).toEqual({
            hasUnread: false,
            hasCriticalUnread: false,
        });
    });

    it('does not throw on a non-array response (must never crash the nav)', () => {
        // The API/mocks can return an unexpected shape (e.g. {}); the indicator
        // must coerce to empty rather than throw during render.
        for (const input of [{}, null, undefined]) {
            expect(() => computeAnnouncementsIndicator(input as never, null, NOW)).not.toThrow();
            expect(computeAnnouncementsIndicator(input as never, null, NOW)).toEqual({
                hasUnread: false,
                hasCriticalUnread: false,
            });
        }
    });
});
