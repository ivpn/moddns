import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

const registerSWMock = vi.hoisted(() => vi.fn());
const toastInfoMock = vi.hoisted(() => vi.fn());

vi.mock('virtual:pwa-register', () => ({ registerSW: registerSWMock }));
vi.mock('sonner', () => ({ toast: { info: toastInfoMock } }));

import { setupSWUpdate } from '@/lib/swUpdate';

type RegisterSWOptions = {
    immediate?: boolean;
    onRegisteredSW?: (
        swUrl: string,
        registration?: { update: () => Promise<void>; waiting?: unknown },
    ) => void;
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

function versionResponse(buildId: string) {
    return Promise.resolve({ ok: true, json: () => Promise.resolve({ buildId }) });
}

describe('setupSWUpdate', () => {
    const updateSWMock = vi.fn().mockResolvedValue(undefined);
    const reloadMock = vi.fn();
    const fetchMock = vi.fn();
    // setupSWUpdate attaches document-level listeners; track them so each test
    // starts with a clean document and stale listeners can't double-fire updateSW.
    let addedListeners: Array<[string, EventListener]> = [];
    // Listeners the module attaches on navigator.serviceWorker (controllerchange).
    let swListeners: Array<[string, EventListener]> = [];

    beforeEach(() => {
        vi.useFakeTimers();
        registerSWMock.mockReturnValue(updateSWMock);
        // Matching build by default so existing SW-lifecycle tests are unaffected
        // by the version poll; individual tests override with a mismatch.
        fetchMock.mockImplementation(() => versionResponse('test-build'));
        vi.stubGlobal('fetch', fetchMock);
        // jsdom's location.reload is unimplemented (throws); the module must go
        // through window.location.reload so this stub observes it.
        vi.stubGlobal('location', { ...window.location, reload: reloadMock });
        // jsdom has no navigator.serviceWorker by default.
        Object.defineProperty(navigator, 'serviceWorker', {
            value: {
                addEventListener: (type: string, listener: EventListener) => {
                    swListeners.push([type, listener]);
                },
                removeEventListener: vi.fn(),
            },
            configurable: true,
        });
        sessionStorage.clear();
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
        swListeners = [];
        vi.useRealTimers();
        vi.clearAllMocks();
        vi.restoreAllMocks();
        vi.unstubAllGlobals();
    });

    function capturedOptions(): RegisterSWOptions {
        setupSWUpdate();
        expect(registerSWMock).toHaveBeenCalledTimes(1);
        return registerSWMock.mock.calls[0][0] as RegisterSWOptions;
    }

    function fireControllerChange() {
        const listener = swListeners.find(([type]) => type === 'controllerchange')?.[1];
        expect(listener).toBeDefined();
        listener!(new Event('controllerchange'));
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

    it('does not re-apply via the tab-away listener after Refresh is clicked', () => {
        const options = capturedOptions();
        options.onNeedRefresh?.();

        const [, toastOptions] = toastInfoMock.mock.calls[0];
        toastOptions.action.onClick();
        expect(updateSWMock).toHaveBeenCalledTimes(1);

        // Tabbing away before the reload lands must not apply the update again.
        setDocumentHidden(true);
        document.dispatchEvent(new Event('visibilitychange'));
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

    // Applying a waiting SW must always end in a reload: the plugin's own
    // reload is gated on workbox's isUpdate flag, which is false when the page
    // was uncontrolled at registration time (first visit after clearing site
    // data) — the module reloads on controllerchange itself, with a timer
    // fallback for Safari cases where controllerchange never fires.
    it('reloads once the new SW takes control after Refresh is clicked', () => {
        const options = capturedOptions();
        options.onNeedRefresh?.();
        const [, toastOptions] = toastInfoMock.mock.calls[0];
        toastOptions.action.onClick();
        expect(reloadMock).not.toHaveBeenCalled();

        fireControllerChange();
        expect(reloadMock).toHaveBeenCalledTimes(1);

        // The fallback timer must not produce a second reload.
        vi.advanceTimersByTime(60 * 1000);
        expect(reloadMock).toHaveBeenCalledTimes(1);
    });

    it('falls back to a timed reload when controllerchange never fires', () => {
        const options = capturedOptions();
        options.onNeedRefresh?.();
        const [, toastOptions] = toastInfoMock.mock.calls[0];
        toastOptions.action.onClick();

        vi.advanceTimersByTime(10 * 1000);
        expect(reloadMock).toHaveBeenCalledTimes(1);
    });

    // Safari/iOS can apply a SW update on relaunch before page JS runs: the
    // page is stale, the new SW already controls it, and no waiting SW ever
    // appears — only the version.json poll can detect that state.
    it('shows the refresh toast when version.json reports a new build and no SW is waiting', async () => {
        fetchMock.mockImplementation(() => versionResponse('newer-build'));
        const options = capturedOptions();
        options.onRegisteredSW?.('/sw.js', { update: vi.fn().mockResolvedValue(undefined) });

        await vi.advanceTimersByTimeAsync(15 * 60 * 1000);
        expect(toastInfoMock).toHaveBeenCalled();
        expect(fetchMock).toHaveBeenCalledWith('/version.json', { cache: 'no-store' });

        // No waiting SW → applying is a plain reload, not skip-waiting.
        const [, toastOptions] = toastInfoMock.mock.calls[0];
        toastOptions.action.onClick();
        expect(updateSWMock).not.toHaveBeenCalled();
        expect(reloadMock).toHaveBeenCalledTimes(1);
    });

    it('does not toast when version.json matches the running build', async () => {
        const options = capturedOptions();
        options.onRegisteredSW?.('/sw.js', { update: vi.fn().mockResolvedValue(undefined) });

        await vi.advanceTimersByTimeAsync(15 * 60 * 1000);
        expect(fetchMock).toHaveBeenCalled();
        expect(toastInfoMock).not.toHaveBeenCalled();
    });

    it('prefers the waiting-SW path when one exists at Refresh-click time', async () => {
        fetchMock.mockImplementation(() => versionResponse('newer-build'));
        const options = capturedOptions();
        options.onRegisteredSW?.('/sw.js', {
            update: vi.fn().mockResolvedValue(undefined),
            waiting: {},
        });

        await vi.advanceTimersByTimeAsync(15 * 60 * 1000);
        const [, toastOptions] = toastInfoMock.mock.calls[0];
        toastOptions.action.onClick();
        expect(updateSWMock).toHaveBeenCalledTimes(1);
        expect(reloadMock).not.toHaveBeenCalled();
    });

    it('auto-reloads a hidden stale tab at most once per build', async () => {
        fetchMock.mockImplementation(() => versionResponse('newer-build'));
        setDocumentHidden(true);

        const options = capturedOptions();
        options.onRegisteredSW?.('/sw.js', { update: vi.fn().mockResolvedValue(undefined) });
        await vi.advanceTimersByTimeAsync(15 * 60 * 1000);
        expect(reloadMock).toHaveBeenCalledTimes(1);
        expect(toastInfoMock).not.toHaveBeenCalled();

        // Same build detected again after the "reload" (fresh page load whose
        // sessionStorage guard survived): must not loop, and must fall back to
        // the toast once the tab is visible.
        registerSWMock.mockClear();
        const options2 = capturedOptions();
        options2.onRegisteredSW?.('/sw.js', { update: vi.fn().mockResolvedValue(undefined) });
        await vi.advanceTimersByTimeAsync(15 * 60 * 1000);
        expect(reloadMock).toHaveBeenCalledTimes(1);

        setDocumentHidden(false);
        document.dispatchEvent(new Event('visibilitychange'));
        await vi.advanceTimersByTimeAsync(0);
        expect(reloadMock).toHaveBeenCalledTimes(1);
        expect(toastInfoMock).toHaveBeenCalled();
    });
});
