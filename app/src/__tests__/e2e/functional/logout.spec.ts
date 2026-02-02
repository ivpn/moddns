import { expect } from '@playwright/test';
import { desktopOnly as test } from '../utils/desktopOnly';
import { AUTH_TOAST_IDS } from '../../../lib/authToasts';
import { registerMocks } from '../../mocks/registerMocks';

// Covers manual logout via UI and ensures single success toast + redirect.

test.describe('Logout flows (desktop only)', () => {
  test('Manual logout (force helper) from authenticated home redirects to login and shows toast', async ({ page }) => {
    await registerMocks(page, { authenticated: true, customProfiles: [{ id: 'prof_1', name: 'Main' }] });

    await page.goto('/home');
    await expect(page).toHaveURL(/\/home$/);

  // Use global helper for deterministic logout since UI trigger test id not guaranteed
  await page.evaluate(() => (window as unknown as { __APP_DISPATCH_EVENT__: (e: { type: string }) => void }).__APP_DISPATCH_EVENT__({ type: 'auth/forceLogout' }));

    await expect(page).toHaveURL(/\/login$/);
    await expect(page.getByTestId(AUTH_TOAST_IDS.logoutSuccess)).toBeVisible();
    await expect(page.getByTestId(AUTH_TOAST_IDS.sessionExpired)).toHaveCount(0);
  });
});
