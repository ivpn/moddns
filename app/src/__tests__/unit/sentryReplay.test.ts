import { beforeEach, describe, expect, it, vi } from 'vitest';

const { start, stop, addIntegration, replayIntegration } = vi.hoisted(() => {
    const start = vi.fn();
    const stop = vi.fn().mockResolvedValue(undefined);
    return {
        start,
        stop,
        addIntegration: vi.fn(),
        replayIntegration: vi.fn(() => ({ name: 'Replay', start, stop })),
    };
});

vi.mock('@sentry/react', () => ({ addIntegration, replayIntegration }));

import { REPLAY_SESSION_SAMPLE_RATE, createReplayConsentSync } from '@/lib/sentryReplay';

beforeEach(() => vi.clearAllMocks());

describe('replay consent sync', () => {
    it('never adds replay without consent', () => {
        const sync = createReplayConsentSync(() => false);
        sync();
        sync();
        expect(replayIntegration).not.toHaveBeenCalled();
        expect(addIntegration).not.toHaveBeenCalled();
        expect(stop).not.toHaveBeenCalled();
    });

    it('adds replay once on consent and does not call start', () => {
        let consent = true;
        const sync = createReplayConsentSync(() => consent);
        sync();
        expect(replayIntegration).toHaveBeenCalledTimes(1);
        expect(addIntegration).toHaveBeenCalledTimes(1);
        expect(start).not.toHaveBeenCalled();
        consent = true;
        sync();
        expect(addIntegration).toHaveBeenCalledTimes(1);
    });

    it('configures masking and blocking explicitly', () => {
        createReplayConsentSync(() => true)();
        expect(replayIntegration).toHaveBeenCalledWith(
            expect.objectContaining({ maskAllText: true, maskAllInputs: true, blockAllMedia: true }),
        );
    });

    it('stops on revoke', () => {
        let consent = true;
        const sync = createReplayConsentSync(() => consent);
        sync();
        consent = false;
        sync();
        expect(stop).toHaveBeenCalledTimes(1);
    });

    it('restarts on re-consent only when the sampled branch is taken', () => {
        let consent = true;
        let draw = REPLAY_SESSION_SAMPLE_RATE + 0.01;
        const sync = createReplayConsentSync(() => consent, () => draw);
        sync();
        consent = false;
        sync();
        consent = true;
        sync();
        expect(start).not.toHaveBeenCalled();
        expect(addIntegration).toHaveBeenCalledTimes(1);

        consent = false;
        sync();
        draw = REPLAY_SESSION_SAMPLE_RATE - 0.01;
        consent = true;
        sync();
        expect(start).toHaveBeenCalledTimes(1);
    });

    it('scrubs ids from replay breadcrumb events', () => {
        createReplayConsentSync(() => true)();
        const opts = (replayIntegration.mock.calls[0] as unknown as [{ beforeAddRecordingEvent: (e: unknown) => unknown }])[0];
        const out = opts.beforeAddRecordingEvent({
            data: { tag: 'breadcrumb', payload: { message: 'GET /api/v1/profiles/p1', data: { url: '/profiles/p1/logs?device_id=d' } } },
        }) as { data: { payload: { message: string; data: { url: string } } } };
        expect(out.data.payload.message).toBe('GET /api/v1/profiles/:profile_id');
        expect(out.data.payload.data.url).toBe('/profiles/:profile_id/logs?device_id=[Filtered]');
    });
});
