// Loaded into the generated service worker via workbox.importScripts.
// Workbox runtime caches outlive their route (cleanupOutdatedCaches only
// removes precaches); waitUntil holds activation until the delete completes.
self.addEventListener("activate", (event) => {
  event.waitUntil(caches.delete("api").catch(() => {}));
});
