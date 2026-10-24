// Workbox runtime caches outlive their route: cleanupOutdatedCaches only
// removes precaches, so this cache name is deleted explicitly. The service
// worker also deletes it on activate (public/sw-cache-cleanup.js).
const API_RESPONSE_CACHE = "api";

export async function deleteApiResponseCache() {
  // Cache Storage is absent in insecure contexts.
  if (typeof caches === "undefined") return;
  try {
    await caches.delete(API_RESPONSE_CACHE);
  } catch {
    // Storage blocked (SecurityError); nothing the app can do.
  }
}
