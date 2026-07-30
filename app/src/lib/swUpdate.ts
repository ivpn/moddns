import { registerSW } from "virtual:pwa-register";
import { toast } from "sonner";

const CHECK_INTERVAL_MS = 15 * 60 * 1000;
// Stable toast id so repeated update signals update one toast instead of stacking.
const UPDATE_TOAST_ID = "sw-update";
// Startup version check is delayed so a normal waiting-SW discovery (which the
// browser kicks off on navigation) gets to fire onNeedRefresh first.
const STARTUP_VERSION_CHECK_MS = 5 * 1000;
// Reload fallback after skip-waiting, for engines where controllerchange
// doesn't arrive reliably.
const APPLY_RELOAD_FALLBACK_MS = 4 * 1000;
// sessionStorage key recording the build id we already auto-reloaded for.
const AUTO_RELOAD_GUARD_KEY = "sw-update-auto-reload";
// SPA navigations also trigger a check (checkForAppUpdate), throttled so
// click-heavy sessions don't hammer the server.
const NAV_CHECK_MIN_INTERVAL_MS = 60 * 1000;

// Injected via `define` (vite.config.ts and the unit vitest config); the build
// also emits /version.json carrying the same id.
declare const __APP_BUILD_ID__: string;

/**
 * Deploy-freshness policy (issue #631):
 * - poll every 15 minutes and whenever the tab regains visibility — by default
 *   the browser only checks on navigation, which a SPA rarely does — for BOTH
 *   a new sw.js (registration.update) and a new build id (version.json). The
 *   version poll covers Safari/iOS, which evicts tabs and applies SW updates
 *   around navigation before page JS runs: the page can be stale with the new
 *   SW already in control and nothing ever reaching "waiting", so the SW
 *   lifecycle events alone never fire there;
 * - when an update is detected: apply it immediately if the tab is hidden,
 *   otherwise show a persistent "Refresh" toast AND apply the moment the user
 *   tabs away. Applying prefers the waiting-SW path (skip-waiting, then reload
 *   every open tab) and falls back to a plain reload when no SW is waiting.
 *   A version mismatch only prompts after the SW pipeline has settled (no
 *   install in flight) — prompting mid-install offers a reload the old worker
 *   answers with the old page, i.e. a second toast after reloading;
 * - reload on `controllerchange` ourselves rather than relying on the
 *   register module's `isUpdate`-gated reload: workbox only sets isUpdate when
 *   the page was already controlled at registration time, so on a first visit
 *   after clearing site data the Refresh click would otherwise do nothing;
 * - hidden-tab fallback reloads are guarded to once per build id so a stale
 *   cache that survives a reload cannot cause a reload loop.
 */
// Bound by setupSWUpdate once registration completes; see checkForAppUpdate.
let activeNavCheck: (() => void) | undefined;

/**
 * Update check for SPA route navigations (mounted in App.tsx). Complements the
 * 15-minute interval: an active user discovers a deploy within about a minute
 * of it landing. Throttled against ALL checks (interval, visibility, previous
 * navigations) and a safe no-op until the service worker is registered.
 */
export function checkForAppUpdate() {
  activeNavCheck?.();
}

export function setupSWUpdate() {
  if (!("serviceWorker" in navigator)) return;
  activeNavCheck = undefined;

  let swRegistration: ServiceWorkerRegistration | undefined;
  let updateHandled = false;
  let reloading = false;

  const reload = () => {
    if (reloading) return;
    reloading = true;
    window.location.reload();
  };

  const applyWaiting = () => {
    navigator.serviceWorker.addEventListener("controllerchange", reload, { once: true });
    setTimeout(reload, APPLY_RELOAD_FALLBACK_MS);
    void updateSW();
  };

  // One automatic reload per deployed build; returns whether it reloaded.
  const autoReload = (buildId: string) => {
    try {
      if (sessionStorage.getItem(AUTO_RELOAD_GUARD_KEY) === buildId) return false;
      sessionStorage.setItem(AUTO_RELOAD_GUARD_KEY, buildId);
    } catch {
      // No storage (e.g. private mode edge cases) → no loop guard → never
      // auto-reload; the visible-tab toast still offers a manual refresh.
      return false;
    }
    reload();
    return true;
  };

  // hasWaiting distinguishes the trigger: onNeedRefresh guarantees a waiting
  // SW; the version poll may fire when the SW already updated silently.
  const onUpdateAvailable = (source: { hasWaiting: boolean; buildId?: string }) => {
    if (updateHandled) return;
    const apply = (manual: boolean) => {
      if (source.hasWaiting || swRegistration?.waiting) {
        applyWaiting();
        return true;
      }
      if (manual) {
        reload();
        return true;
      }
      return autoReload(source.buildId ?? "unknown");
    };
    if (document.hidden) {
      // A guard-skipped auto-reload leaves updateHandled false so the next
      // check after the tab becomes visible surfaces the toast instead.
      updateHandled = apply(false);
      return;
    }
    updateHandled = true;
    // Single entry point for both triggers (toast click, tab-away) that
    // detaches the listener first, so the update is only ever applied once.
    const applyOnce = () => {
      document.removeEventListener("visibilitychange", onHidden);
      apply(true);
    };
    const onHidden = () => {
      if (!document.hidden) return;
      document.removeEventListener("visibilitychange", onHidden);
      apply(false);
    };
    document.addEventListener("visibilitychange", onHidden);
    toast.info("A new version of modDNS is available.", {
      id: UPDATE_TOAST_ID,
      duration: Infinity,
      action: { label: "Refresh", onClick: applyOnce },
    });
  };

  const versionCheck = async () => {
    try {
      const res = await fetch("/version.json", { cache: "no-store" });
      if (!res.ok) return;
      const { buildId } = (await res.json()) as { buildId?: string };
      if (!buildId || buildId === __APP_BUILD_ID__) return;
      // A new build exists. Let the SW pipeline settle before prompting: the
      // poll usually outruns the multi-MB precache install, and a plain
      // reload during that window is answered by the OLD worker with the old
      // page — the user gets the same toast again after reloading (observed
      // as a double toast on Chrome/Safari).
      if (swRegistration) {
        try {
          await swRegistration.update();
        } catch {
          // Transient — fall through with whatever state we can see.
        }
        if (swRegistration.waiting) {
          onUpdateAvailable({ hasWaiting: true });
          return;
        }
        // Install in flight: stay quiet; the 'waiting' event raises the
        // toast once applying it can actually succeed. updateHandled stays
        // false, so a failed install is retried by the next poll tick.
        if (swRegistration.installing) return;
      }
      // Settled with nothing pending: the controller already updated silently
      // (Safari's relaunch path) — a plain reload genuinely gets the new app.
      onUpdateAvailable({ hasWaiting: false, buildId });
    } catch {
      // Offline, or dev server without version.json — the next tick retries.
    }
  };

  const updateSW = registerSW({
    immediate: true,
    onRegisteredSW(_swUrl, registration) {
      if (!registration) return;
      swRegistration = registration;
      // The page load itself just fetched everything fresh, so the first
      // navigation check is only useful once the throttle window has passed.
      let lastCheckAt = Date.now();
      const check = () => {
        if (!navigator.onLine) return;
        lastCheckAt = Date.now();
        registration.update().catch(() => {
          // Transient network error — the next tick retries.
        });
        void versionCheck();
      };
      activeNavCheck = () => {
        if (Date.now() - lastCheckAt < NAV_CHECK_MIN_INTERVAL_MS) return;
        check();
      };
      setInterval(check, CHECK_INTERVAL_MS);
      document.addEventListener("visibilitychange", () => {
        if (document.visibilityState === "visible") check();
      });
    },
    onNeedRefresh() {
      onUpdateAvailable({ hasWaiting: true });
    },
    onRegisterError() {
      // Non-fatal: the app works without a service worker.
    },
  });

  // Catch the stale-page-under-a-new-SW race at startup (Safari can swap the
  // SW mid-navigation, after the old HTML was already served).
  setTimeout(() => {
    if (navigator.onLine) void versionCheck();
  }, STARTUP_VERSION_CHECK_MS);
}
