import { test, expect, type Page } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';

const storedDismissal = (page: Page) => page.evaluate(() =>
  JSON.parse(localStorage.getItem('moddns-storage') ?? '{}')?.state?.passkeyBannerDismissedKey ?? null);

test.describe('Passkey verification banner', () => {
  const baseAccount = { account_id: 'abc', email: 'user@example.com', email_verified: true, mfa: { totp: { enabled: false } } };

  async function setup(page: Page, authMethods: string[], passkeys: Record<string, unknown>[]) {
    await registerMocks(page, { authenticated: true });
    // Registered after registerMocks: its /api/v1/ catch-all would otherwise answer first.
    await page.route(/\/api\/v1\/accounts\/current(\/?|\?.*)$/i, route =>
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ ...baseAccount, auth_methods: authMethods }) }));
    await page.route('**/api/v1/sub', route =>
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ status: 'active', plan: 'plus', active_until: '2027-01-01T00:00:00Z' }) }));
    await page.route('**/api/v1/webauthn/passkeys', route =>
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(passkeys) }));
    await page.goto('/account-preferences');
  }

  // specRef: api-endpoint-behaviour F17
  test('an account with a password closes the banner for good in this browser', async ({ page }) => {
    await setup(page, ['password', 'passkey'], [{ id: 'p1', created_at: '2026-01-01T00:00:00Z', user_verified: false }]);

    const banner = page.getByTestId('passkey-verification-banner');
    await expect(banner).toContainText('You can still sign in with your password.');
    await banner.getByRole('button', { name: 'Close' }).click();
    await expect(banner).toBeHidden();

    // Navigate only once the page has rendered: WebKit logs an aborted lazy-chunk import otherwise.
    await expect(page.getByText('Account info')).toBeVisible();
    await page.goto('/settings');
    const settingsLoaded = page.getByText('Settings on this page are applied only to this profile.');
    await expect(settingsLoaded).toBeVisible();
    await expect(page.getByTestId('passkey-verification-banner')).toBeHidden();
    await page.reload();
    await expect(settingsLoaded).toBeVisible();
    await expect(page.getByTestId('passkey-verification-banner')).toBeHidden();

    // /blocklists loads profiles only, not the account.
    const passkeysLoaded = page.waitForResponse('**/api/v1/webauthn/passkeys');
    await page.goto('/blocklists');
    await passkeysLoaded;
    // The tabs render in the same component as the banner, after the lazy page chunk loads.
    await expect(page.getByRole('tab', { name: 'Lists' })).toBeVisible();
    await expect(page.getByTestId('passkey-verification-banner')).toBeHidden();
  });

  // specRef: api-endpoint-behaviour F17
  test('closing the banner in one tab closes it in another open tab', async ({ page, context }) => {
    const passkeys = [{ id: 'p1', created_at: '2026-01-01T00:00:00Z', user_verified: false }];
    const other = await context.newPage();
    await setup(other, ['password', 'passkey'], passkeys);
    await setup(page, ['password', 'passkey'], passkeys);
    await expect(other.getByTestId('passkey-verification-banner')).toBeVisible();

    await page.getByTestId('passkey-verification-banner').getByRole('button', { name: 'Close' }).click();

    await expect(other.getByTestId('passkey-verification-banner')).toBeHidden();
    await expect(other.getByText('Account info')).toBeVisible();
    await other.reload();
    await expect(other.getByText('Account info')).toBeVisible();
    await expect(other.getByTestId('passkey-verification-banner')).toBeHidden();
    expect(await storedDismissal(page)).toBe('p1');
  });

  // specRef: api-endpoint-behaviour F17
  test('logging out forgets the dismissal', { tag: '@desktop' }, async ({ page }) => {
    await setup(page, ['password', 'passkey'], [{ id: 'p1', created_at: '2026-01-01T00:00:00Z', user_verified: false }]);
    await page.getByTestId('passkey-verification-banner').getByRole('button', { name: 'Close' }).click();
    expect(await storedDismissal(page)).toBe('p1');

    await page.route(/\/api\/v1\/accounts\/logout/i, route =>
      route.request().method() === 'POST' ? route.fulfill({ status: 200, contentType: 'application/json', body: '{}' }) : route.continue());
    await page.getByTestId('btn-nav-logout').click();
    await page.getByTestId('btn-confirm-logout').click();
    await expect(page).toHaveURL(/\/login$/);

    expect(await storedDismissal(page)).toBeNull();
  });

  // specRef: api-endpoint-behaviour F16, F17
  test('a passkey-only account cannot close the banner', async ({ page }) => {
    await setup(page, ['passkey'], [{ id: 'p1', created_at: '2026-01-01T00:00:00Z', user_verified: false }]);

    const banner = page.getByTestId('passkey-verification-banner');
    await expect(banner).toContainText('One of your passkeys needs an important update.');
    await expect(banner).not.toContainText('You can still sign in');
    await expect(banner.getByRole('button', { name: 'Close' })).toHaveCount(0);
  });
});
