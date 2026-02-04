import { expect } from '@playwright/test';
import { desktopOnly as test } from '../utils/desktopOnly';
import { AUTH_TOAST_IDS } from '../../../lib/authToasts';

// Scenario: user hits session limit (429 with specific error), sees dialog, cancels -> remains on login, no success toast.

test.describe('Session limit dialog cancel flow (desktop only)', () => {
  test('Cancel maintains login state without authenticating', async ({ page }) => {
    const authed = false;

    // Mock login returning session limit on first attempt
    await page.route(/\/api\/v1\/login(\/?|\?.*)$/i, async route => {
      if (route.request().method() === 'POST') {
        return route.fulfill({ status: 429, contentType: 'application/json', body: JSON.stringify({ error: 'maximum number of active sessions reached' }) });
      }
      return route.continue();
    });

    // Auth-dependent resources always 401 since not authenticated
    await page.route(/\/api\/v1\/accounts\/current(\/?|\?.*)$/i, r => r.fulfill({ status: authed ? 200 : 401, contentType: 'application/json', body: authed ? JSON.stringify({ id: 'a1', email: 'user@example.com', email_verified: true, profiles: ['p1'] }) : '{}' }));
  await page.route(/\/api\/v1\/profiles(\/?|\?.*)$/i, r => r.fulfill({ status: authed ? 200 : 401, contentType: 'application/json', body: authed ? JSON.stringify([{ id: 'p1', profile_id: 'p1', account_id: 'a1', name: 'Profile 1', settings: { profile_id: 'p1', advanced: {}, logs: { enabled: false }, privacy: { default_rule: 'allow', subdomains_rule: 'allow', blocklists: [] }, security: {}, statistics: { enabled: false }, custom_rules: [] } }]) : '[]' }));

    await page.goto('/login');
    // Wait for lazy-loaded Login component to render before checking form mode
    await page.getByTestId('login-page').waitFor();

    // Ensure password mode
    if (await page.getByTestId('login-passkey-form').count()) {
      await page.getByTestId('btn-login-toggle-mode').click();
    }

    await page.getByTestId('input-email').fill('user@example.com');
    await page.getByTestId('input-password').fill('secret');
    await page.getByTestId('btn-login-password-submit').click();

  // Dialog should appear exactly once (no duplicate mounts)
  const dialogs = page.getByTestId('session-limit-dialog');
  await expect(dialogs).toBeVisible();
  await expect(dialogs).toHaveCount(1);

    // Cancel path
    await page.getByTestId('session-limit-cancel').click();

    // Back on login page, dialog gone, no success toast
    await expect(page).toHaveURL(/\/login$/);
    await expect(page.getByTestId('session-limit-dialog')).toHaveCount(0);
    await expect(page.getByTestId(AUTH_TOAST_IDS.loginSuccess)).toHaveCount(0);
  });
});
