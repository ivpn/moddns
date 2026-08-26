import { describe, test, expect, vi, beforeEach, afterEach, type MockInstance } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { createRef } from 'react';
import { useHeaderStackHeight } from '@/lib/useHeaderStackHeight';

class MockResizeObserver {
    observe() { }
    unobserve() { }
    disconnect() { }
}

describe('useHeaderStackHeight', () => {
    let setPropertySpy: MockInstance;
    let height: number;

    beforeEach(() => {
        height = 64;
        vi.stubGlobal('ResizeObserver', MockResizeObserver);
        // Synchronous rAF makes the initial measure observable without timers.
        vi.stubGlobal('requestAnimationFrame', (cb: FrameRequestCallback) => {
            cb(0);
            return 0;
        });
        setPropertySpy = vi.spyOn(document.documentElement.style, 'setProperty');
    });

    afterEach(() => {
        setPropertySpy.mockRestore();
        vi.unstubAllGlobals();
    });

    const makeRef = () => {
        const el = document.createElement('div');
        vi.spyOn(el, 'getBoundingClientRect').mockImplementation(() => ({ height } as DOMRect));
        const ref = createRef<HTMLElement>();
        (ref as { current: HTMLElement | null }).current = el;
        return ref;
    };

    test('initial measure publishes both CSS variables', () => {
        const refs = [makeRef()];
        renderHook(() => useHeaderStackHeight(refs, { reducePx: 10 }));

        expect(setPropertySpy).toHaveBeenCalledWith('--app-header-stack-full', '64px');
        expect(setPropertySpy).toHaveBeenCalledWith('--app-header-stack', '54px');
    });

    test('a resize with unchanged heights writes nothing', () => {
        // Mobile URL-bar collapse/expand fires window resize on every scroll direction
        // change; rewriting an identical value invalidates root style and repositions
        // every var-consuming sticky element a frame late (visible as flicker).
        const refs = [makeRef()];
        renderHook(() => useHeaderStackHeight(refs));
        setPropertySpy.mockClear();

        act(() => {
            window.dispatchEvent(new Event('resize'));
        });

        expect(setPropertySpy).not.toHaveBeenCalled();
    });

    test('a resize with a changed height writes both variables again', () => {
        const refs = [makeRef()];
        renderHook(() => useHeaderStackHeight(refs, { reducePx: 10 }));
        setPropertySpy.mockClear();

        height = 100;
        act(() => {
            window.dispatchEvent(new Event('resize'));
        });

        expect(setPropertySpy).toHaveBeenCalledWith('--app-header-stack-full', '100px');
        expect(setPropertySpy).toHaveBeenCalledWith('--app-header-stack', '90px');
    });
});
