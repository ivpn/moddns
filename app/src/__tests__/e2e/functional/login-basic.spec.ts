import { test, expect } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';
import { AUTH_TOAST_IDS, TOTP_TOO_MANY_ATTEMPTS_MESSAGE } from '../../../lib/authToasts';

test.describe('Login basic flows', { tag: '@desktop' }, () => {
  test('successful password login shows toast and redirects', async ({ page }) => {
    // Dynamic auth state for account/profile routes
    let authed = false;

    await page.route(/\/api\/v1\/login(\/?|\?.*)$/i, async route => {
      if (route.request().method() === 'POST') {
        authed = true; // flip auth so subsequent loader calls succeed
        await route.fulfill({ status: 200, contentType: 'application/json', body: '{}' });
      } else {
        await route.continue();
      }
    });
    await page.route(/\/api\/v1\/accounts\/current(\/?|\?.*)$/i, async route => {
      await route.fulfill({
        status: authed ? 200 : 401,
        contentType: 'application/json',
        body: authed ? JSON.stringify({ id: 'a1', email: 'user@example.com', email_verified: true, profiles: ['p1'] }) : '{}' }
      );
    });
    await page.route(/\/api\/v1\/profiles(\/?|\?.*)$/i, async route => {
      await route.fulfill({
        status: authed ? 200 : 401,
        contentType: 'application/json',
        body: authed ? JSON.stringify([{ id: 'p1', profile_id: 'p1', account_id: 'a1', name: 'Profile 1', settings: { profile_id: 'p1', advanced: {}, logs: { enabled: false }, privacy: { default_rule: 'allow', blocklists_subdomains_rule: 'allow', blocklists: [] }, security: {}, statistics: { enabled: false }, custom_rules: [] } }]) : '[]' }
      );
    });

    await page.goto('/login');
    // Wait for lazy-loaded Login component to render before checking form mode
    await page.getByTestId('login-page').waitFor();
    if (await page.getByTestId('login-passkey-form').count()) {
      await page.getByTestId('btn-login-toggle-mode').click();
    }

    await page.getByTestId('input-email').fill('user@example.com');
    await page.getByTestId('input-password').fill('correct-password');
    await page.getByTestId('btn-login-password-submit').click();

    await page.waitForURL(/\/home$/);
    await expect(page.getByTestId(AUTH_TOAST_IDS.loginSuccess)).toBeVisible();
  });

  test('invalid credentials shows error toast and stays on login', async ({ page }) => {
    await registerMocks(page, { authenticated: false });
    await page.route(/\/api\/v1\/login(\/?|\?.*)$/i, async route => {
      if (route.request().method() === 'POST') {
        await route.fulfill({ status: 400, contentType: 'application/json', body: JSON.stringify({ error: 'invalid credentials' }) });
      } else {
        await route.continue();
      }
    });
    await page.goto('/login');
    await page.getByTestId('login-page').waitFor();
    if (await page.getByTestId('login-passkey-form').count()) {
      await page.getByTestId('btn-login-toggle-mode').click();
    }
    await page.getByTestId('input-email').fill('user@example.com');
    await page.getByTestId('input-password').fill('wrong-password');
    await page.getByTestId('btn-login-password-submit').click();
    await expect(page).toHaveURL(/\/login/);
    await expect(page.getByTestId(AUTH_TOAST_IDS.loginInvalid)).toBeVisible();
  });

  test('401 unauthorized (generic) shows unauthorized toast', async ({ page }) => {
    await registerMocks(page, { authenticated: false });
    await page.route(/\/api\/v1\/login(\/?|\?.*)$/i, async route => {
      if (route.request().method() === 'POST') {
        await route.fulfill({ status: 401, contentType: 'application/json', body: JSON.stringify({ error: 'Unauthorized' }) });
      } else {
        await route.continue();
      }
    });
    await page.goto('/login');
    await page.getByTestId('login-page').waitFor();
    if (await page.getByTestId('login-passkey-form').count()) {
      await page.getByTestId('btn-login-toggle-mode').click();
    }
    await page.getByTestId('input-email').fill('user@example.com');
    await page.getByTestId('input-password').fill('bad');
    await page.getByTestId('btn-login-password-submit').click();
    await expect(page).toHaveURL(/\/login/);
    await expect(page.getByTestId(AUTH_TOAST_IDS.loginUnauthorized)).toBeVisible();
  });

  test('429 rate limit (too many attempts) shows rate limit toast', async ({ page }) => {
    await registerMocks(page, { authenticated: false });
    await page.route(/\/api\/v1\/login(\/?|\?.*)$/i, async route => {
      if (route.request().method() === 'POST') {
        await route.fulfill({ status: 429, contentType: 'application/json', body: JSON.stringify({ error: 'too many attempts' }) });
      } else {
        await route.continue();
      }
    });
    await page.goto('/login');
    await page.getByTestId('login-page').waitFor();
    if (await page.getByTestId('login-passkey-form').count()) {
      await page.getByTestId('btn-login-toggle-mode').click();
    }
    await page.getByTestId('input-email').fill('user@example.com');
    await page.getByTestId('input-password').fill('bad');
    await page.getByTestId('btn-login-password-submit').click();
    await expect(page).toHaveURL(/\/login/);
    await expect(page.getByTestId(AUTH_TOAST_IDS.loginTooManyAttempts)).toBeVisible();
  });

  test('too many failed 2FA attempts says the 2FA code is the problem', async ({ page }) => {
    await registerMocks(page, { authenticated: false });
    let attempt = 0;
    await page.route(/\/api\/v1\/login(\/?|\?.*)$/i, async route => {
      if (route.request().method() !== 'POST') return route.continue();
      attempt++;
      if (attempt === 1) {
        return route.fulfill({ status: 401, contentType: 'application/json', body: JSON.stringify({ error: 'TOTP is required' }) });
      }
      return route.fulfill({ status: 429, contentType: 'application/json', body: JSON.stringify({ error: 'too many failed 2FA attempts, try again later' }) });
    });
    await page.goto('/login');
    await page.getByTestId('login-page').waitFor();
    if (await page.getByTestId('login-passkey-form').count()) {
      await page.getByTestId('btn-login-toggle-mode').click();
    }
    await page.getByTestId('input-email').fill('user@example.com');
    await page.getByTestId('input-password').fill('secret');
    await page.getByTestId('btn-login-password-submit').click();
    await page.getByTestId('input-otp').fill('123456');
    await page.getByTestId('btn-login-password-submit').click();
    // The password was right; the message must point at the 2FA code.
    await expect(page.getByTestId(AUTH_TOAST_IDS.loginTotpTooManyAttempts)).toHaveText(TOTP_TOO_MANY_ATTEMPTS_MESSAGE);
    await expect(page.getByTestId(AUTH_TOAST_IDS.loginTooManyAttempts)).toHaveCount(0);
  });

  test('clicking the modDNS logo navigates to the landing page', async ({ page }) => {
    await registerMocks(page, { authenticated: false });
    await page.goto('/login');
    await page.getByTestId('login-page').waitFor();
    await page.getByRole('link', { name: 'modDNS home' }).click();
    await expect(page).toHaveURL(/\/$/);
    await expect(page.locator('.moddns-landing')).toBeVisible();
  });
});
