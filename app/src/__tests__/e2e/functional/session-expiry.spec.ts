import { test, expect } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';
import { AUTH_TOAST_IDS } from '../../../lib/authToasts';

// The app exposes window.__APP_DISPATCH_EVENT__ so tests can trigger a forced logout
// without a real 401 round-trip. Both variants must land on /login with the login
// page rendered (not a lingering loading screen) and raise exactly one toast.

type ForceLogout = { type: 'auth/forceLogout'; reason?: string; toastType?: string };
const dispatch = (page: import('@playwright/test').Page, event: ForceLogout) =>
  page.evaluate((e) => (window as unknown as { __APP_DISPATCH_EVENT__: (ev: unknown) => void }).__APP_DISPATCH_EVENT__(e), event);

test.describe('@functional Forced logout', () => {
  test.beforeEach(async ({ page }) => {
    await registerMocks(page, { authenticated: true, customProfiles: [{ id: 'prof_1', name: 'Default' }] });
    await page.goto('/home');
    await expect(page).toHaveURL(/\/home$/);
  });

  test('session expiry redirects to login with the session-expired toast', async ({ page }) => {
    await dispatch(page, { type: 'auth/forceLogout', reason: 'Session expired - please log in again.', toastType: 'error' });
    await expect(page).toHaveURL(/\/login$/);
    await expect(page.getByTestId('login-page')).toBeVisible();
    await expect(page.getByTestId(AUTH_TOAST_IDS.sessionExpired)).toBeVisible();
    await expect(page.getByTestId(AUTH_TOAST_IDS.logoutSuccess)).toHaveCount(0);
  });

  test('manual logout redirects to login with the logged-out toast', async ({ page }) => {
    await dispatch(page, { type: 'auth/forceLogout' });
    await expect(page).toHaveURL(/\/login$/);
    await expect(page.getByTestId('login-page')).toBeVisible();
    await expect(page.getByTestId(AUTH_TOAST_IDS.logoutSuccess)).toBeVisible();
    await expect(page.getByTestId(AUTH_TOAST_IDS.sessionExpired)).toHaveCount(0);
  });
});
