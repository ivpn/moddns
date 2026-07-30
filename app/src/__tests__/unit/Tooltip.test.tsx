import { render, screen, fireEvent, act } from '@testing-library/react';
import '@testing-library/jest-dom';
import { describe, test, expect, vi, beforeEach, afterEach } from 'vitest';
import Tooltip from '@/components/ui/tooltip';

function renderTooltip(delay = 0) {
    return render(
        <div>
            <Tooltip content="Helpful info" delay={delay}>
                <button aria-label="info trigger">i</button>
            </Tooltip>
            <button aria-label="outside">outside</button>
        </div>
    );
}

function trigger() {
    // Handlers live on the wrapper span around the child button
    return screen.getByLabelText('info trigger').parentElement as HTMLElement;
}

// jsdom has no real PointerEvent, so fireEvent.pointerDown drops pointerType;
// dispatch a hand-built event carrying it instead.
function firePointerDown(el: HTMLElement | Document, pointerType: string) {
    const ev = new Event('pointerdown', { bubbles: true, cancelable: true });
    Object.defineProperty(ev, 'pointerType', { value: pointerType });
    fireEvent(el, ev);
}

function tap(el: HTMLElement) {
    firePointerDown(el, 'touch');
    fireEvent.click(el);
}

describe('Tooltip touch support (#127)', () => {
    test('tap shows the tooltip immediately', () => {
        renderTooltip();
        tap(trigger());
        expect(screen.getByRole('tooltip')).toBeInTheDocument();
    });

    test('second tap on the trigger hides the tooltip', () => {
        renderTooltip();
        tap(trigger());
        expect(screen.getByRole('tooltip')).toBeInTheDocument();
        tap(trigger());
        expect(screen.queryByRole('tooltip')).not.toBeInTheDocument();
    });

    test('tap outside hides a tap-opened tooltip', () => {
        renderTooltip();
        tap(trigger());
        expect(screen.getByRole('tooltip')).toBeInTheDocument();
        firePointerDown(screen.getByLabelText('outside'), 'touch');
        expect(screen.queryByRole('tooltip')).not.toBeInTheDocument();
    });

    test('Escape hides a tap-opened tooltip', () => {
        renderTooltip();
        tap(trigger());
        expect(screen.getByRole('tooltip')).toBeInTheDocument();
        fireEvent.keyDown(document, { key: 'Escape' });
        expect(screen.queryByRole('tooltip')).not.toBeInTheDocument();
    });

    test('tap-opened tooltip survives the synthetic mouseleave a tap can emit', () => {
        renderTooltip();
        tap(trigger());
        fireEvent.mouseLeave(trigger());
        expect(screen.getByRole('tooltip')).toBeInTheDocument();
    });
});

describe('Tooltip hover regression', () => {
    beforeEach(() => vi.useFakeTimers());
    afterEach(() => vi.useRealTimers());

    test('mouse hover still shows after delay and hides on leave', () => {
        renderTooltip(150);
        fireEvent.mouseEnter(trigger());
        expect(screen.queryByRole('tooltip')).not.toBeInTheDocument();
        act(() => { vi.advanceTimersByTime(200); });
        expect(screen.getByRole('tooltip')).toBeInTheDocument();
        fireEvent.mouseLeave(trigger());
        expect(screen.queryByRole('tooltip')).not.toBeInTheDocument();
    });

    test('mouse click does not toggle a hover-opened tooltip closed', () => {
        renderTooltip(0);
        fireEvent.mouseEnter(trigger());
        act(() => { vi.advanceTimersByTime(50); });
        expect(screen.getByRole('tooltip')).toBeInTheDocument();
        firePointerDown(trigger(), 'mouse');
        fireEvent.click(trigger());
        expect(screen.getByRole('tooltip')).toBeInTheDocument();
    });
});
