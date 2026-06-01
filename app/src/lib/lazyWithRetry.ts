import { lazy, ComponentType, LazyExoticComponent } from 'react';

/**
 * A lazy component that can also be preloaded imperatively (e.g. on nav hover)
 * to warm its chunk before the user clicks.
 */
export type PreloadableComponent<T extends ComponentType<unknown>> =
  LazyExoticComponent<T> & { preload: () => Promise<{ default: T }> };

/**
 * Wrapper around React.lazy that retries failed dynamic imports.
 *
 * During Vite HMR, dynamic imports can fail when modules are invalidated.
 * This wrapper catches those failures and either retries the import
 * or forces a page reload to get fresh modules.
 *
 * The returned component also exposes `preload()`, which kicks off the same
 * import ahead of render. The import promise is memoized, so preload() on hover
 * followed by React.lazy resolving on click share a single in-flight request.
 */
export function lazyWithRetry<T extends ComponentType<unknown>>(
  importFn: () => Promise<{ default: T }>,
  retries = 2
): PreloadableComponent<T> {
  const loadWithRetry = async (): Promise<{ default: T }> => {
    let lastError: Error | undefined;

    for (let attempt = 0; attempt <= retries; attempt++) {
      try {
        return await importFn();
      } catch (error) {
        lastError = error as Error;

        // Check if this is a dynamic import failure (common during HMR)
        const isChunkError =
          error instanceof Error &&
          (error.message.includes('dynamically imported module') ||
           error.message.includes('Failed to fetch') ||
           error.message.includes('Loading chunk') ||
           error.message.includes('Loading CSS chunk'));

        if (!isChunkError) {
          // Not a chunk loading error, throw immediately
          throw error;
        }

        // Wait a bit before retrying (exponential backoff)
        if (attempt < retries) {
          await new Promise(resolve => setTimeout(resolve, 100 * Math.pow(2, attempt)));
        }
      }
    }

    // All retries failed - force page reload to get fresh modules
    // This handles deployment mismatches where old HTML references non-existent chunks
    if (typeof window !== 'undefined') {
      // Only reload if we haven't recently reloaded to prevent infinite loops
      const lastReloadKey = '__lazy_import_reload_timestamp__';
      const lastReload = sessionStorage.getItem(lastReloadKey);
      const now = Date.now();

      if (!lastReload || now - parseInt(lastReload, 10) > 10000) {
        sessionStorage.setItem(lastReloadKey, now.toString());
        // Cache-busting reload: add timestamp to bypass browser/CDN cache
        // This ensures we fetch fresh index.html with correct chunk references
        const url = window.location.href.split('?')[0];
        window.location.href = `${url}?_=${now}`;
        // Return a never-resolving promise to keep Suspense showing fallback during redirect
        return new Promise(() => {});
      }
    }

    // If we can't reload or it didn't help, throw the error
    throw lastError;
  };

  // Memoize the in-flight import so hover-preload and lazy-on-click dedupe.
  // On failure we clear it so a later attempt can retry rather than replaying
  // the rejected promise.
  let cached: Promise<{ default: T }> | undefined;
  const load = (): Promise<{ default: T }> => {
    if (!cached) {
      cached = loadWithRetry().catch((error) => {
        cached = undefined;
        throw error;
      });
    }
    return cached;
  };

  const Component = lazy(load) as PreloadableComponent<T>;
  Component.preload = load;
  return Component;
}
