import { expect } from '@playwright/test';
import { desktopOnly as test } from '../utils/desktopOnly';
import { registerMocks } from '../../mocks/registerMocks';
import { AUTH_TOAST_IDS } from '../../../lib/authToasts';

test.describe('Login basic flows (desktop only)', () => {
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
        body: authed ? JSON.stringify([{ id: 'p1', profile_id: 'p1', account_id: 'a1', name: 'Profile 1', settings: { profile_id: 'p1', advanced: {}, logs: { enabled: false }, privacy: { default_rule: 'allow', subdomains_rule: 'allow', blocklists: [] }, security: {}, statistics: { enabled: false }, custom_rules: [] } }]) : '[]' }
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
});
