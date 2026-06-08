import { describe, it, expect, vi, beforeEach, afterEach, type Mock } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import type { AnnouncementsAnnouncement } from '@/api/client';

vi.mock('@/api/api', () => ({
    default: {
        Client: {
            announcementsApi: {
                apiV1AnnouncementsGet: vi.fn(),
            },
        },
    },
}));

const NOW = new Date('2026-06-08T00:00:00Z');
const critical = (): AnnouncementsAnnouncement[] => [
    { id: 'incident', title: 'incident', category: 'incident', severity: 'critical', published_at: NOW.toISOString() },
];

// eslint-disable-next-line @typescript-eslint/no-explicit-any
const resp = (data: AnnouncementsAnnouncement[]) => Promise.resolve({ data } as any);

async function flush() {
    // let the fetch promise + setState settle
    await act(async () => {
        await Promise.resolve();
        await Promise.resolve();
    });
}

// The hook keeps a module-level dedupe cache. Rather than expose a test-only
// reset on production code, each test re-imports the module graph fresh via
// vi.resetModules() so the cache (and the mocked api fn) start clean. Because the
// hook also imports the store, we must pull useAppStore from the SAME post-reset
// graph or setState would target a different store instance than the hook reads.
let useAnnouncementsIndicator: typeof import('@/hooks/useAnnouncementsIndicator')['useAnnouncementsIndicator'];
let useAppStore: typeof import('@/store/general')['useAppStore'];
let mockGet: Mock;

describe('useAnnouncementsIndicator — live refresh (fix 2)', () => {
    beforeEach(async () => {
        vi.useFakeTimers();
        vi.setSystemTime(NOW);
        vi.resetModules();

        const apiMod = await import('@/api/api');
        mockGet = apiMod.default.Client.announcementsApi.apiV1AnnouncementsGet as unknown as Mock;
        mockGet.mockReset();

        ({ useAppStore } = await import('@/store/general'));
        // clean browser: 48h recency window applies, recent critical counts as unread
        useAppStore.setState({ announcementsLastSeenAt: null });

        ({ useAnnouncementsIndicator } = await import('@/hooks/useAnnouncementsIndicator'));
    });

    afterEach(() => {
        vi.useRealTimers();
    });

    it('picks up a newly-published announcement when the tab regains visibility', async () => {
        mockGet.mockReturnValueOnce(resp([])); // initial: nothing
        const { result } = renderHook(() => useAnnouncementsIndicator());
        await flush();
        expect(result.current.hasUnread).toBe(false);
        expect(mockGet).toHaveBeenCalledTimes(1);

        // A critical announcement is published; advance past the 60s dedupe cache.
        mockGet.mockReturnValueOnce(resp(critical()));
        await act(async () => {
            vi.advanceTimersByTime(61_000);
            document.dispatchEvent(new Event('visibilitychange'));
            await Promise.resolve();
        });
        await flush();

        expect(mockGet).toHaveBeenCalledTimes(2);
        expect(result.current.hasUnread).toBe(true);
        expect(result.current.hasCriticalUnread).toBe(true);
    });

    it('picks up a newly-published announcement on the 5-minute poll', async () => {
        mockGet.mockReturnValueOnce(resp([]));
        const { result } = renderHook(() => useAnnouncementsIndicator());
        await flush();
        expect(result.current.hasUnread).toBe(false);

        mockGet.mockReturnValueOnce(resp(critical()));
        await act(async () => {
            vi.advanceTimersByTime(5 * 60_000); // interval fires; cache is also stale
            await Promise.resolve();
        });
        await flush();

        expect(mockGet).toHaveBeenCalledTimes(2);
        expect(result.current.hasUnread).toBe(true);
    });

    it('removes its listeners and interval on unmount', async () => {
        mockGet.mockReturnValue(resp([]));
        const removeSpy = vi.spyOn(document, 'removeEventListener');
        const { unmount } = renderHook(() => useAnnouncementsIndicator());
        await flush();
        unmount();
        expect(removeSpy).toHaveBeenCalledWith('visibilitychange', expect.any(Function));
        removeSpy.mockRestore();
    });
});
