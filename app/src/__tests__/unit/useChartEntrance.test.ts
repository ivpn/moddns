// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act, renderHook } from '@testing-library/react';
import { useChartEntrance } from '@/pages/statistics/useChartEntrance';

function motion(reduce: boolean | null) {
    if (reduce === null) {
        // @ts-expect-error jsdom has no matchMedia by default
        delete window.matchMedia;
        return;
    }
    window.matchMedia = ((q: string) => ({ matches: reduce && q.includes('reduce'), media: q, addEventListener() {}, removeEventListener() {} })) as unknown as typeof window.matchMedia;
}

describe('useChartEntrance', () => {
    beforeEach(() => {
        vi.useFakeTimers();
        motion(false);
    });
    afterEach(() => {
        vi.useRealTimers();
        motion(null);
    });

    it('plays once on first render and stays off for refetches of the same window', () => {
        // tableRef: statistics-behaviour #X8
        const { result, rerender } = renderHook(({ k }) => useChartEntrance(k), { initialProps: { k: '900:168' } });
        expect(result.current).toBe(true);
        act(() => void vi.advanceTimersByTime(800));
        expect(result.current).toBe(false);
        rerender({ k: '900:168' });
        expect(result.current).toBe(false);
    });

    it('plays again when the window changes', () => {
        // tableRef: statistics-behaviour #X8
        const { result, rerender } = renderHook(({ k }) => useChartEntrance(k), { initialProps: { k: '900:3' } });
        act(() => void vi.advanceTimersByTime(800));
        expect(result.current).toBe(false);
        rerender({ k: '3600:168' });
        expect(result.current).toBe(true);
        act(() => void vi.advanceTimersByTime(800));
        expect(result.current).toBe(false);
    });

    it('never plays under prefers-reduced-motion or without matchMedia', () => {
        // tableRef: statistics-behaviour #X8
        motion(true);
        expect(renderHook(() => useChartEntrance('a')).result.current).toBe(false);
        motion(null);
        expect(renderHook(() => useChartEntrance('a')).result.current).toBe(false);
    });
});
