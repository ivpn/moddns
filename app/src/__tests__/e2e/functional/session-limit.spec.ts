import { test, expect } from '@playwright/test';
import { AUTH_TOAST_IDS } from '../../../lib/authToasts';

// Scenario: user hits session limit (429 with specific error), sees dialog, cancels -> remains on login, no success toast.

test.describe('Session limit dialog cancel flow', { tag: '@desktop' }, () => {
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
  await page.route(/\/api\/v1\/profiles(\/?|\?.*)$/i, r => r.fulfill({ status: authed ? 200 : 401, contentType: 'application/json', body: authed ? JSON.stringify([{ id: 'p1', profile_id: 'p1', account_id: 'a1', name: 'Profile 1', settings: { profile_id: 'p1', advanced: {}, logs: { enabled: false }, privacy: { default_rule: 'allow', blocklists_subdomains_rule: 'allow', blocklists: [] }, security: {}, statistics: { enabled: false }, custom_rules: [] } }]) : '[]' }));

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

// A 2FA account at the session limit: removing the other sessions signs in with the code
// already entered; the API has not used it up. specRef: api-endpoint-behaviour A8a
test.describe('Session limit with 2FA', () => {
  test('removing sessions resends the 2FA code', async ({ page }) => {
    const BACKUP = 'GFUKS-Z7KZB-VPNDN-TMWMS-HAKR5';
    const sent: { code?: string; remove?: string }[] = [];
    await page.route(/\/api\/v1\/login(\/?|\?.*)$/i, async route => {
      if (route.request().method() !== 'POST') return route.continue();
      const h = route.request().headers();
      const code = (route.request().postDataJSON() as { otp?: string }).otp;
      sent.push({ code, remove: h['x-sessions-remove'] });
      if (!code) {
        return route.fulfill({ status: 401, contentType: 'application/json', body: JSON.stringify({ error: 'TOTP is required' }) });
      }
      if (h['x-sessions-remove'] !== 'true') {
        return route.fulfill({ status: 429, contentType: 'application/json', body: JSON.stringify({ error: 'maximum number of active sessions reached' }) });
      }
      return route.fulfill({ status: 200, contentType: 'application/json', body: '{}' });
    });

    await page.goto('/login');
    await page.getByTestId('login-page').waitFor();
    if (await page.getByTestId('login-passkey-form').count()) {
      await page.getByTestId('btn-login-toggle-mode').click();
    }
    await page.getByTestId('input-email').fill('user@example.com');
    await page.getByTestId('input-password').fill('secret');
    await page.getByTestId('btn-login-password-submit').click();

    await page.getByTestId('input-otp-mode-toggle').click();
    await page.getByTestId('input-otp').fill(BACKUP);
    await page.getByTestId('btn-login-password-submit').click();
    await expect(page.getByTestId('session-limit-dialog')).toBeVisible();
    await page.getByTestId('session-limit-confirm').click();

    await expect.poll(() => sent.length).toBe(3);
    expect(sent[2]).toEqual({ code: BACKUP, remove: 'true' });
  });
});

// The code can expire while the session-limit dialog is open: the user is asked for a new
// one and the next retry sends it. specRef: api-endpoint-behaviour A8a
test.describe('Session limit with an expired 2FA code', () => {
  test('asks for a new code and retries with it', async ({ page }) => {
    const sent: { code?: string; remove?: string }[] = [];
    await page.route(/\/api\/v1\/login(\/?|\?.*)$/i, async route => {
      if (route.request().method() !== 'POST') return route.continue();
      const h = route.request().headers();
      const code = (route.request().postDataJSON() as { otp?: string }).otp;
      sent.push({ code, remove: h['x-sessions-remove'] });
      if (!code) {
        return route.fulfill({ status: 401, contentType: 'application/json', body: JSON.stringify({ error: 'TOTP is required' }) });
      }
      if (h['x-sessions-remove'] !== 'true') {
        return route.fulfill({ status: 429, contentType: 'application/json', body: JSON.stringify({ error: 'maximum number of active sessions reached' }) });
      }
      if (code === '111111') {
        return route.fulfill({ status: 401, contentType: 'application/json', body: JSON.stringify({ error: 'invalid 2FA code' }) });
      }
      return route.fulfill({ status: 200, contentType: 'application/json', body: '{}' });
    });

    await page.goto('/login');
    await page.getByTestId('login-page').waitFor();
    if (await page.getByTestId('login-passkey-form').count()) {
      await page.getByTestId('btn-login-toggle-mode').click();
    }
    await page.getByTestId('input-email').fill('user@example.com');
    await page.getByTestId('input-password').fill('secret');
    await page.getByTestId('btn-login-password-submit').click();

    await page.getByTestId('input-otp').fill('111111');
    await page.getByTestId('btn-login-password-submit').click();
    await page.getByTestId('session-limit-confirm').click();

    await expect(page.getByTestId(AUTH_TOAST_IDS.loginTotpRetry)).toBeVisible();
    await expect(page.getByTestId('session-limit-dialog')).toHaveCount(0);
    await expect(page.getByTestId('input-otp')).toHaveValue('');

    await page.getByTestId('input-otp').fill('222222');
    await page.getByTestId('btn-login-password-submit').click();
    await page.getByTestId('session-limit-confirm').click();
    await expect.poll(() => sent[sent.length - 1]).toEqual({ code: '222222', remove: 'true' });
  });
});
