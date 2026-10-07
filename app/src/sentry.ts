import * as Sentry from "@sentry/react";
import { useAppStore } from '@/store/general';
import { createPrivacyHooks } from '@/lib/sentryPrivacy';
import { REPLAY_SESSION_SAMPLE_RATE, createReplayConsentSync } from '@/lib/sentryReplay';

const hasConsent = () => !!useAppStore.getState().account?.error_reports_consent;
const hooks = createPrivacyHooks(hasConsent);

Sentry.init({
    dsn: import.meta.env.VITE_SENTRY_DSN,
    environment: import.meta.env.VITE_SENTRY_ENVIRONMENT,
    // Adds request headers and IP for users, for more info visit:
    // https://docs.sentry.io/platforms/javascript/guides/react/configuration/options/#sendDefaultPii
    sendDefaultPii: false,

    // Tracing is sampled to zero and replay is not installed until the account consents.
    integrations: [Sentry.browserTracingIntegration()],
    tracesSampler: hooks.tracesSampler,
    replaysSessionSampleRate: REPLAY_SESSION_SAMPLE_RATE,
    beforeSend: hooks.beforeSend,
    beforeSendTransaction: hooks.beforeSendTransaction,
    beforeBreadcrumb: hooks.beforeBreadcrumb,
    // tracePropagationTargets: ["localhost:3000"],
});

const syncReplay = createReplayConsentSync(hasConsent);
useAppStore.subscribe((state, prev) => {
    if (!!state.account?.error_reports_consent !== !!prev.account?.error_reports_consent) syncReplay();
});
syncReplay();
