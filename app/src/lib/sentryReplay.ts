import * as Sentry from "@sentry/react";
import { scrubBreadcrumb } from "./sentryPrivacy";

export const REPLAY_SESSION_SAMPLE_RATE = 0.1;

type Replay = ReturnType<typeof Sentry.replayIntegration>;

function createReplay(): Replay {
    return Sentry.replayIntegration({
        // Explicit: these pages show domains and device and profile ids.
        maskAllText: true,
        maskAllInputs: true,
        blockAllMedia: true,
        beforeAddRecordingEvent: event => {
            const data = event.data as { tag?: string; payload?: { message?: string; data?: Record<string, unknown> } } | undefined;
            if (data?.tag === "breadcrumb" && data.payload) {
                const scrubbed = scrubBreadcrumb({ message: data.payload.message, data: data.payload.data });
                data.payload = { ...data.payload, message: scrubbed.message, data: scrubbed.data };
            }
            return event;
        },
    });
}

/**
 * Returns a function to call whenever consent may have changed.
 *
 * The first consent only adds the integration: addIntegration already applies the session sample
 * rate, whereas `start()` records regardless of sampling. After a revoke, re-consent restarts
 * recording only when this page lifetime's own draw against the same rate succeeds.
 */
export function createReplayConsentSync(
    hasConsent: () => boolean,
    random: () => number = Math.random,
): () => void {
    let replay: Replay | null = null;
    return () => {
        if (hasConsent()) {
            if (!replay) {
                replay = createReplay();
                Sentry.addIntegration(replay);
            } else if (random() < REPLAY_SESSION_SAMPLE_RATE) {
                replay.start();
            }
        } else if (replay) {
            void replay.stop();
        }
    };
}
