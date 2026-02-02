import { test, expect } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';
import { AUTH_TOAST_IDS } from '../../../lib/authToasts';

// Failing test (initially) to reproduce session expiry UI bug.
// Expected correct behavior: upon session expiration, user is redirected to /login,
// login page content is visible, and a session expired toast appears.
// Current bug: a persistent loading screen (or non-login state) appears instead.

test.describe('@functional Session Expiry', () => {
  test('redirects to login with session expired toast when force logout helper invoked', async ({ page }) => {
    await registerMocks(page, { authenticated: true, customProfiles: [{ id: 'prof_1', name: 'Default' }] });

  // No console log dependency; production logs removed.

    await page.goto('/home');
    await expect.poll(() => page.url()).toMatch(/\/home$/);

  // Wait until helper is attached (effect mounts after initial render)
  await page.evaluate(() => (window as unknown as { __APP_DISPATCH_EVENT__: (e: { type: string; reason: string; toastType: string }) => void }).__APP_DISPATCH_EVENT__({ type: 'auth/forceLogout', reason: 'Session expired - please log in again.', toastType: 'error' }));

    await expect.poll(() => page.url(), { timeout: 8000 }).toMatch(/\/login$/);
    await expect(page.getByTestId('login-page')).toBeVisible();

  // Toast assertion by test id
  await expect(page.getByTestId(AUTH_TOAST_IDS.sessionExpired)).toBeVisible();

  // Behavior verified by URL + toast only.
  });

  test('session expired toast appears if loader forces logout before navigation to protected page', async ({ page }) => {
    // Start unauthenticated but attempt to visit a protected route, emulate loader forcing logout (side effect already done in app code when account fetch 401 + flag)
    await registerMocks(page, { authenticated: true, customProfiles: [{ id: 'prof_1', name: 'Main' }] });
    await page.goto('/home');
    await expect(page).toHaveURL(/\/home$/);
    // Trigger forced logout
  await page.evaluate(() => (window as unknown as { __APP_DISPATCH_EVENT__: (e: { type: string; reason: string; toastType: string }) => void }).__APP_DISPATCH_EVENT__({ type: 'auth/forceLogout', reason: 'Session expired - please log in again.', toastType: 'error' }));
    await expect(page).toHaveURL(/\/login$/);
    await expect(page.getByTestId('login-page')).toBeVisible();
    await expect(page.getByTestId(AUTH_TOAST_IDS.sessionExpired)).toBeVisible();
  });
});
