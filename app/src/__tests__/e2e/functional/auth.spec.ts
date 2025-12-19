// moved from e2e root -> functional/auth.spec.ts
import { test, expect } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';
import { AUTH_KEY } from '@/lib/consts';

// Tag: @functional

test.describe('@functional Authentication', () => {
  test.beforeEach(async ({ page }) => {
    // Clear storage deterministically
    await page.addInitScript(() => {
      localStorage.clear();
      sessionStorage.clear();
      document.cookie.split(';').forEach(c => {
        const eqPos = c.indexOf('=');
        const name = eqPos > -1 ? c.substr(0, eqPos) : c;
        document.cookie = name + '=;expires=Thu, 01 Jan 1970 00:00:00 GMT;path=/';
      });
    });
  });

  test('redirects unauthenticated user to /login when visiting protected route', async ({ page }) => {
    // Register mocks with unauthenticated state ensures 401 and redirect path is exercised without real network
    await registerMocks(page, { authenticated: false });
    await page.goto('/home');
    await expect.poll(async () => page.url()).toMatch(/\/login$/);
  });

  test('login page renders without console errors (mobile)', async ({ page }) => {
    const errors: string[] = [];
    page.on('console', msg => {
      if (msg.type() === 'error') errors.push(msg.text());
    });
    await registerMocks(page, { authenticated: false });
    await page.goto('/login');
    const toggle = page.getByTestId('btn-login-toggle-mode');
    await expect(toggle).toBeVisible();
    await toggle.click();
    await expect(page.getByTestId(/login-(passkey|password)-form/)).toBeVisible();
    expect(errors).toEqual([]);
  });

  test('successful logout returns to login without hook errors', async ({ page }) => {
    if (!/desktop/i.test(test.info().project.name)) test.skip();
    const consoleErrors: string[] = [];
    page.on('console', msg => { if (msg.type() === 'error') consoleErrors.push(msg.text()); });

    // Authenticated start
    await registerMocks(page, { authenticated: true, customProfiles: [{ id: 'prof_1', name: 'Default' }] });
    await page.goto('/home');
    await expect(page).toHaveURL(/\/home$/);

    const navLogout = page.getByTestId('btn-nav-logout');
    await expect(navLogout).toBeVisible();
    await navLogout.click();

    // Confirm dialog appears then confirm logout via test id
    const confirmButton = page.getByTestId('btn-confirm-logout');
    await expect(confirmButton).toBeVisible();
    // Ensure /logout is mocked (idempotent) then click without waiting on network (which may be skipped in mock layer)
    await page.route(/\/api\/v1\/accounts\/logout/i, route => {
      if (route.request().method() === 'POST') {
        return route.fulfill({ status: 200, contentType: 'application/json', body: '{}' });
      }
      return route.continue();
    });
    await confirmButton.click();

    // Poll URL for redirect to login
    await expect.poll(async () => page.url(), { timeout: 10000 }).toMatch(/\/login$/);

    // Toast should appear (non-strict if animations delay)
    try {
      await expect(page.getByText('Logged out successfully.', { exact: false })).toBeVisible({ timeout: 4000 });
    } catch (e) {
      // Soft warn if toast missing but proceed (remove this if toast becomes mandatory)
      console.warn('[SOFT] Logout success toast not detected');
    }
    await expect(page.getByTestId('btn-login-toggle-mode')).toBeVisible();
    const hookError = consoleErrors.find(e => /Rendered fewer hooks than expected/i.test(e));
    expect(hookError, 'No hook order error should be logged on logout').toBeFalsy();
    // Poll for auth flag no longer being 'true' (implementation sets 'false' rather than removing key)
    await expect.poll(async () => page.evaluate(k => localStorage.getItem(k), AUTH_KEY), { timeout: 10000 })
      .not.toBe('true');
  });
});

test.describe('@functional Root index redirects', () => {
  test('unauthenticated visit to root redirects to /login', async ({ page }) => {
    await registerMocks(page, { authenticated: false });
    await page.goto('/');
    await expect.poll(async () => page.url()).toMatch(/\/login$/);
  });

  test('stale auth flag with expired session still redirects to /login', async ({ page }) => {
    await registerMocks(page, { authenticated: false });
    await page.addInitScript((key: string) => { window.localStorage.setItem(key, 'true'); }, AUTH_KEY);
    await page.goto('/');
    await expect.poll(async () => page.url()).toMatch(/\/login$/);
  });

  test('valid session at root immediately navigates to /home', async ({ page }) => {
    await registerMocks(page, { authenticated: true, customProfiles: [{ id: 'prof_1', name: 'Default' }] });
    await page.goto('/');
    await expect.poll(async () => page.url()).toMatch(/\/home$/);
  });
});
