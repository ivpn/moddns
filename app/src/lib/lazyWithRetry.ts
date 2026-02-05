import { lazy, ComponentType } from 'react';

/**
 * Wrapper around React.lazy that retries failed dynamic imports.
 *
 * During Vite HMR, dynamic imports can fail when modules are invalidated.
 * This wrapper catches those failures and either retries the import
 * or forces a page reload to get fresh modules.
 */
export function lazyWithRetry<T extends ComponentType<unknown>>(
  importFn: () => Promise<{ default: T }>,
  retries = 2
): React.LazyExoticComponent<T> {
  return lazy(async () => {
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
  });
}
