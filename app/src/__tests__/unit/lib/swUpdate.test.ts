import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

const registerSWMock = vi.hoisted(() => vi.fn());
const toastInfoMock = vi.hoisted(() => vi.fn());

vi.mock('virtual:pwa-register', () => ({ registerSW: registerSWMock }));
vi.mock('sonner', () => ({ toast: { info: toastInfoMock } }));

import { setupSWUpdate } from '@/lib/swUpdate';

type RegisterSWOptions = {
    immediate?: boolean;
    onRegisteredSW?: (swUrl: string, registration?: { update: () => Promise<void> }) => void;
    onNeedRefresh?: () => void;
    onRegisterError?: (error: unknown) => void;
};

function setDocumentHidden(hidden: boolean) {
    Object.defineProperty(document, 'hidden', { value: hidden, configurable: true });
    Object.defineProperty(document, 'visibilityState', {
        value: hidden ? 'hidden' : 'visible',
        configurable: true,
    });
}

describe('setupSWUpdate', () => {
    const updateSWMock = vi.fn().mockResolvedValue(undefined);
    // setupSWUpdate attaches document-level listeners; track them so each test
    // starts with a clean document and stale listeners can't double-fire updateSW.
    let addedListeners: Array<[string, EventListener]> = [];

    beforeEach(() => {
        vi.useFakeTimers();
        registerSWMock.mockReturnValue(updateSWMock);
        // jsdom has no navigator.serviceWorker by default.
        Object.defineProperty(navigator, 'serviceWorker', { value: {}, configurable: true });
        setDocumentHidden(false);
        const originalAdd = document.addEventListener.bind(document);
        vi.spyOn(document, 'addEventListener').mockImplementation((type, listener, options) => {
            addedListeners.push([type, listener as EventListener]);
            originalAdd(type, listener, options);
        });
    });

    afterEach(() => {
        addedListeners.forEach(([type, listener]) => document.removeEventListener(type, listener));
        addedListeners = [];
        vi.useRealTimers();
        vi.clearAllMocks();
        vi.restoreAllMocks();
    });

    function capturedOptions(): RegisterSWOptions {
        setupSWUpdate();
        expect(registerSWMock).toHaveBeenCalledTimes(1);
        return registerSWMock.mock.calls[0][0] as RegisterSWOptions;
    }

    it('registers immediately', () => {
        const options = capturedOptions();
        expect(options.immediate).toBe(true);
    });

    it('does nothing when service workers are unsupported', () => {
        // @ts-expect-error jsdom allows deleting the stubbed property
        delete navigator.serviceWorker;
        setupSWUpdate();
        expect(registerSWMock).not.toHaveBeenCalled();
    });

    it('schedules periodic update checks every 15 minutes', () => {
        const options = capturedOptions();
        const registration = { update: vi.fn().mockResolvedValue(undefined) };
        options.onRegisteredSW?.('/sw.js', registration);

        vi.advanceTimersByTime(15 * 60 * 1000);
        expect(registration.update).toHaveBeenCalledTimes(1);
        vi.advanceTimersByTime(15 * 60 * 1000);
        expect(registration.update).toHaveBeenCalledTimes(2);
    });

    it('checks for updates when the tab becomes visible', () => {
        const options = capturedOptions();
        const registration = { update: vi.fn().mockResolvedValue(undefined) };
        options.onRegisteredSW?.('/sw.js', registration);

        setDocumentHidden(false);
        document.dispatchEvent(new Event('visibilitychange'));
        expect(registration.update).toHaveBeenCalledTimes(1);
    });

    it('applies the update immediately when the tab is hidden', () => {
        const options = capturedOptions();
        setDocumentHidden(true);
        options.onNeedRefresh?.();
        expect(updateSWMock).toHaveBeenCalledTimes(1);
        expect(toastInfoMock).not.toHaveBeenCalled();
    });

    it('shows a refresh toast when the tab is visible', () => {
        const options = capturedOptions();
        options.onNeedRefresh?.();
        expect(updateSWMock).not.toHaveBeenCalled();
        expect(toastInfoMock).toHaveBeenCalledTimes(1);
        const [, toastOptions] = toastInfoMock.mock.calls[0];
        expect(toastOptions.duration).toBe(Infinity);

        toastOptions.action.onClick();
        expect(updateSWMock).toHaveBeenCalledTimes(1);
    });

    it('applies a pending (toasted) update once the user tabs away', () => {
        const options = capturedOptions();
        options.onNeedRefresh?.();
        expect(updateSWMock).not.toHaveBeenCalled();

        setDocumentHidden(true);
        document.dispatchEvent(new Event('visibilitychange'));
        expect(updateSWMock).toHaveBeenCalledTimes(1);

        // The listener is one-shot: a second hide does not re-apply.
        document.dispatchEvent(new Event('visibilitychange'));
        expect(updateSWMock).toHaveBeenCalledTimes(1);
    });
});
