// Stub for the `virtual:pwa-register` module provided by vite-plugin-pwa at
// build time. The unit-test vitest config does not run the PWA plugin, so the
// virtual module must be aliased here to resolve at all; tests replace it via
// vi.mock('virtual:pwa-register', ...).
export const registerSW = () => () => Promise.resolve();
