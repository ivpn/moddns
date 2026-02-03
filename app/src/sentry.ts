import * as Sentry from "@sentry/react";
import { useAppStore } from '@/store/general';


Sentry.init({
    dsn: import.meta.env.VITE_SENTRY_DSN,
    environment: import.meta.env.VITE_SENTRY_ENVIRONMENT,
  // Adds request headers and IP for users, for more info visit:
  // https://docs.sentry.io/platforms/javascript/guides/react/configuration/options/#sendDefaultPii
    sendDefaultPii: false,

    integrations: [
        Sentry.browserTracingIntegration(),
        // Sentry.reactRouterV5BrowserTracingIntegration(history),
    ],
    replaysSessionSampleRate: 0.1,
    tracesSampleRate: 1.0,
    // tracePropagationTargets: ["localhost:3000"],
    beforeSend(event) {
        // Custom logic to decide whether to send the event
        const generalStore = useAppStore.getState();
        if (!generalStore.account?.error_reports_consent) {
            return null; // Prevent sending the event
        }
        return event;
    },
});

// Lazy-load the replay integration to reduce initial bundle size
Sentry.lazyLoadIntegration('replayIntegration').then((replay) => {
    Sentry.addIntegration(replay());
});
