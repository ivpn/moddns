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

describe('computeAnnouncementsIndicator', () => {
    const items = [
        a('old', '2026-05-01T00:00:00Z', 'info'),
        a('new', '2026-05-20T00:00:00Z', 'warning'),
    ];

    it('marks everything unread when never seen (null lastSeen)', () => {
        const r = computeAnnouncementsIndicator(items, null);
        expect(r.hasUnread).toBe(true);
        expect(r.hasCriticalUnread).toBe(false);
    });

    it('only counts announcements published after lastSeen', () => {
        const r = computeAnnouncementsIndicator(items, '2026-05-10T00:00:00Z');
        expect(r.hasUnread).toBe(true); // "new" is after lastSeen
    });

    it('clears when lastSeen is after the newest announcement', () => {
        const r = computeAnnouncementsIndicator(items, '2026-06-01T00:00:00Z');
        expect(r.hasUnread).toBe(false);
        expect(r.hasCriticalUnread).toBe(false);
    });

    it('flags critical only when an unread announcement is critical', () => {
        const withCritical = [...items, a('incident', '2026-05-25T00:00:00Z', 'critical')];
        // lastSeen before the critical one -> critical unread
        expect(computeAnnouncementsIndicator(withCritical, '2026-05-21T00:00:00Z')).toEqual({
            hasUnread: true,
            hasCriticalUnread: true,
        });
        // lastSeen after the critical one -> no critical unread
        expect(computeAnnouncementsIndicator(withCritical, '2026-05-26T00:00:00Z')).toEqual({
            hasUnread: false,
            hasCriticalUnread: false,
        });
    });

    it('returns no unread for an empty list', () => {
        expect(computeAnnouncementsIndicator([], null)).toEqual({
            hasUnread: false,
            hasCriticalUnread: false,
        });
    });

    it('does not throw on a non-array response (must never crash the nav)', () => {
        // The API/mocks can return an unexpected shape (e.g. {}); the indicator
        // must coerce to empty rather than throw during render.
        for (const input of [{}, null, undefined]) {
            expect(() => computeAnnouncementsIndicator(input as never, null)).not.toThrow();
            expect(computeAnnouncementsIndicator(input as never, null)).toEqual({
                hasUnread: false,
                hasCriticalUnread: false,
            });
        }
    });
});
