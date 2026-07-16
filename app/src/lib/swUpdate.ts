import { registerSW } from "virtual:pwa-register";
import { toast } from "sonner";

const CHECK_INTERVAL_MS = 15 * 60 * 1000;
// Stable toast id so repeated onNeedRefresh calls update one toast instead of stacking.
const UPDATE_TOAST_ID = "sw-update";

/**
 * Deploy-freshness policy (issue #631):
 * - poll for a new sw.js every 15 minutes and whenever the tab regains
 *   visibility — by default the browser only checks on navigation, which a
 *   SPA rarely does, so long-lived tabs would never discover a deploy;
 * - when an update is waiting: apply it immediately if the tab is hidden,
 *   otherwise show a persistent "Refresh" toast AND apply the moment the
 *   user tabs away. The plugin's register module reloads every open tab on
 *   the workbox `controlling` (isUpdate) event, so multi-tab reload and
 *   loop-safety are handled by the library.
 */
export function setupSWUpdate() {
  if (!("serviceWorker" in navigator)) return;

  const applyUpdate = () => {
    void updateSW();
  };

  const updateSW = registerSW({
    immediate: true,
    onRegisteredSW(_swUrl, registration) {
      if (!registration) return;
      const check = () => {
        if (navigator.onLine) {
          registration.update().catch(() => {
            // Transient network error — the next tick retries.
          });
        }
      };
      setInterval(check, CHECK_INTERVAL_MS);
      document.addEventListener("visibilitychange", () => {
        if (document.visibilityState === "visible") check();
      });
    },
    onNeedRefresh() {
      if (document.hidden) {
        applyUpdate();
        return;
      }
      // Single entry point for both triggers (toast click, tab-away) that
      // detaches the listener first, so the update is only ever applied once.
      const applyOnce = () => {
        document.removeEventListener("visibilitychange", onHidden);
        applyUpdate();
      };
      const onHidden = () => {
        if (!document.hidden) return;
        applyOnce();
      };
      document.addEventListener("visibilitychange", onHidden);
      toast.info("A new version of modDNS is available.", {
        id: UPDATE_TOAST_ID,
        duration: Infinity,
        action: { label: "Refresh", onClick: applyOnce },
      });
    },
    onRegisterError() {
      // Non-fatal: the app works without a service worker.
    },
  });
}
