import { test, expect } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';
import { installWebAuthnCreateExcludedStub, installWebAuthnCreateNotAllowedStub, installWebAuthnCreateStub, installWebAuthnSuccessStub } from '../utils/webauthn';

// The navigator.credentials stubs only take effect in Chromium, hence @desktop like the login specs.
test.describe('Passkey management', { tag: '@desktop' }, () => {
  const baseAccount = { account_id: 'abc', email: 'user@example.com', email_verified: true, mfa: { totp: { enabled: false } } };
  const creationOptions = { publicKey: { challenge: 'abc', rp: { id: 'localhost', name: 'modDNS' }, user: { id: 'dXNlcg', name: 'user@example.com', displayName: 'user@example.com' }, pubKeyCredParams: [{ type: 'public-key', alg: -7 }], timeout: 60000 } };
  const requestOptions = { publicKey: { challenge: 'abc', rpId: 'localhost', timeout: 60000, userVerification: 'preferred', allowCredentials: [] } };

  type Captured = { addBegin: { body: unknown; mfaCode?: string } | null; reauthPurpose: string | null; addFinished: boolean; deleted: { id: string; body: unknown; mfaCode?: string } | null };

  type Page = import('@playwright/test').Page;

  const onePasskey = [{ id: 'p1', created_at: '2026-01-01T00:00:00Z', user_verified: true }];

  async function setup(page: Page, account: Record<string, unknown>, installCreateStub: (page: Page) => Promise<void> = installWebAuthnCreateStub, passkeys: Record<string, unknown>[] = onePasskey): Promise<Captured> {
    const captured: Captured = { addBegin: null, reauthPurpose: null, addFinished: false, deleted: null };
    await installWebAuthnSuccessStub(page);
    await installCreateStub(page);
    await registerMocks(page, { authenticated: true });
    // Registered after registerMocks: its /api/v1/ catch-all would otherwise answer first.
    await page.route(/\/api\/v1\/accounts\/current(\/?|\?.*)$/i, route =>
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(account) }));
    await page.route('**/api/v1/sub', route =>
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ status: 'active', plan: 'plus', active_until: '2027-01-01T00:00:00Z' }) }));
    await page.route('**/api/v1/webauthn/passkeys', route =>
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(passkeys) }));
    await page.route('**/api/v1/webauthn/passkey/reauth/begin', route => {
      captured.reauthPurpose = JSON.parse(route.request().postData() ?? '{}').purpose;
      return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(requestOptions) });
    });
    await page.route('**/api/v1/webauthn/passkey/reauth/finish', route =>
      route.fulfill({ status: 201, contentType: 'application/json', body: JSON.stringify({ reauth_token: 'reauth-token', expires_at: '2027-01-01T00:00:00Z' }) }));
    await page.route('**/api/v1/webauthn/passkey/add/begin', route => {
      captured.addBegin = { body: JSON.parse(route.request().postData() ?? 'null'), mfaCode: route.request().headers()['x-mfa-code'] };
      return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(creationOptions) });
    });
    await page.route('**/api/v1/webauthn/passkey/add/finish', route => {
      captured.addFinished = true;
      return route.fulfill({ status: 201, body: '' });
    });
    await page.route('**/api/v1/webauthn/passkey/p*', route => {
      if (route.request().method() !== 'DELETE') return route.continue();
      captured.deleted = { id: route.request().url().split('/').pop() ?? '', body: JSON.parse(route.request().postData() ?? 'null'), mfaCode: route.request().headers()['x-mfa-code'] };
      return route.fulfill({ status: 204, body: '' });
    });
    await page.goto('/account-preferences');
    return captured;
  }

  // specRef: F3
  test('adding a passkey asks for the current password first', async ({ page }) => {
    const captured = await setup(page, { ...baseAccount, auth_methods: ['password', 'passkey'] });

    await page.getByRole('button', { name: /Add Passkey/i }).click();
    const dialog = page.getByRole('dialog', { name: 'Add passkey' });
    await expect(dialog).toBeVisible();
    await expect(dialog.getByRole('button', { name: 'Continue' })).toBeDisabled();
    await dialog.getByRole('button', { name: 'Password', exact: true }).click();
    await dialog.getByLabel('Current password').fill('CurrentPass123!');
    await dialog.getByRole('button', { name: 'Continue' }).click();

    await expect.poll(() => captured.addFinished).toBe(true);
    expect(captured.addBegin?.body).toEqual({ current_password: 'CurrentPass123!' });
  });

  // specRef: F3
  test('adding a passkey to a passkey-only account verifies with an existing passkey', async ({ page }) => {
    const captured = await setup(page, { ...baseAccount, auth_methods: ['passkey'], mfa: { totp: { enabled: true } } });

    await page.getByRole('button', { name: /Add Passkey/i }).click();
    const dialog = page.getByRole('dialog', { name: 'Add passkey' });
    await dialog.getByRole('button', { name: 'Verify with passkey' }).click();
    await expect(dialog.getByRole('button', { name: 'Passkey verified' })).toBeVisible();
    await dialog.getByRole('button', { name: 'Continue' }).click();

    await expect.poll(() => captured.addFinished).toBe(true);
    expect(captured.reauthPurpose).toBe('passkey_add');
    expect(captured.addBegin?.body).toEqual({ reauth_token: 'reauth-token' });
    expect(captured.addBegin?.mfaCode).toBeUndefined();
  });

  // specRef: F8
  test('deleting a passkey asks for the current password and 2FA code first', async ({ page }) => {
    const captured = await setup(page, { ...baseAccount, auth_methods: ['password', 'passkey'], mfa: { totp: { enabled: true } } });

    await page.getByRole('button', { name: 'Delete', exact: true }).click();
    const dialog = page.getByRole('dialog', { name: 'Delete passkey' });
    await expect(dialog.getByRole('button', { name: 'Delete passkey' })).toBeDisabled();
    await dialog.getByRole('button', { name: 'Password', exact: true }).click();
    await dialog.getByLabel('Current password').fill('CurrentPass123!');
    await dialog.getByLabel('2FA code').fill('123456');
    await dialog.getByRole('button', { name: 'Delete passkey' }).click();

    await expect.poll(() => captured.deleted).not.toBeNull();
    expect(captured.deleted).toEqual({ id: 'p1', body: { current_password: 'CurrentPass123!' }, mfaCode: '123456' });
  });

  // specRef: F8
  test('deleting a passkey can be confirmed with a passkey', async ({ page }) => {
    const captured = await setup(page, { ...baseAccount, auth_methods: ['password', 'passkey'] });

    await page.getByRole('button', { name: 'Delete', exact: true }).click();
    const dialog = page.getByRole('dialog', { name: 'Delete passkey' });
    await dialog.getByRole('button', { name: 'Passkey', exact: true }).click();
    await dialog.getByRole('button', { name: 'Verify with passkey' }).click();
    await expect(dialog.getByRole('button', { name: 'Passkey verified' })).toBeVisible();
    await dialog.getByRole('button', { name: 'Delete passkey' }).click();

    await expect.poll(() => captured.deleted).not.toBeNull();
    expect(captured.reauthPurpose).toBe('passkey_delete');
    expect(captured.deleted?.body).toEqual({ reauth_token: 'reauth-token' });
  });

  // specRef: F8a
  test('the only passkey of an account without a password cannot be deleted', async ({ page }) => {
    await setup(page, { ...baseAccount, auth_methods: ['passkey'] });

    await expect(page.getByRole('button', { name: 'Delete', exact: true })).toBeDisabled();
    await expect(page.getByText('Set a password or add another passkey before deleting this one.')).toBeVisible();
  });

  // specRef: F3b
  test('adding a passkey from an authenticator that already holds one explains why it was refused', async ({ page }) => {
    const captured = await setup(page, { ...baseAccount, auth_methods: ['password', 'passkey'] }, installWebAuthnCreateExcludedStub);

    await page.getByRole('button', { name: /Add Passkey/i }).click();
    const dialog = page.getByRole('dialog', { name: 'Add passkey' });
    await dialog.getByRole('button', { name: 'Password', exact: true }).click();
    await dialog.getByLabel('Current password').fill('CurrentPass123!');
    await dialog.getByRole('button', { name: 'Continue' }).click();

    await expect(page.getByText('This device or password manager already has a passkey for your account')).toBeVisible();
    expect(captured.addFinished).toBe(false);
  });

  // specRef: F7, F15
  test('a passkey whose last sign-in did not verify the user is marked', async ({ page }) => {
    await setup(page, { ...baseAccount, auth_methods: ['password', 'passkey'] }, installWebAuthnCreateStub, [
      { id: 'p1', created_at: '2026-01-01T00:00:00Z', user_verified: true },
      { id: 'p2', created_at: '2026-02-01T00:00:00Z', user_verified: false },
    ]);

    const badges = page.getByTestId('passkey-needs-verification');
    await expect(badges).toHaveCount(1);
    await expect(badges).toHaveText('Needs PIN or biometric');
    const badge = page.getByTestId('passkey-item-p2').getByTestId('passkey-needs-verification');
    await expect(badge).toBeVisible();
    await expect(badge.locator('svg')).toBeVisible();
    // Next to the passkey ID, on the same row.
    const idBox = (await page.getByTestId('passkey-item-p2').getByText('p2', { exact: true }).boundingBox())!;
    const badgeBox = (await badge.boundingBox())!;
    expect(Math.abs((idBox.y + idBox.height / 2) - (badgeBox.y + badgeBox.height / 2))).toBeLessThan(8);
    expect(badgeBox.x).toBeGreaterThan(idBox.x + idBox.width);
  });

  // specRef: F3, F14
  test('a refused passkey creation mentions that security keys need a PIN', async ({ page }) => {
    const captured = await setup(page, { ...baseAccount, auth_methods: ['password', 'passkey'] }, installWebAuthnCreateNotAllowedStub);

    await page.getByRole('button', { name: /Add Passkey/i }).click();
    const dialog = page.getByRole('dialog', { name: 'Add passkey' });
    await dialog.getByRole('button', { name: 'Password', exact: true }).click();
    await dialog.getByLabel('Current password').fill('CurrentPass123!');
    await dialog.getByRole('button', { name: 'Continue' }).click();

    await expect(page.getByText('If you use a security key, it needs a PIN')).toBeVisible();
    expect(captured.addFinished).toBe(false);
  });

  const unverifiedPasskeys = [
    { id: 'p1', created_at: '2026-01-01T00:00:00Z', user_verified: true },
    { id: 'p2', created_at: '2026-02-01T00:00:00Z', user_verified: false },
  ];

  // specRef: F7, F16
  test('an account with a passkey that did not verify the user sees the deadline on every page', async ({ page }) => {
    await setup(page, { ...baseAccount, auth_methods: ['password', 'passkey'] }, installWebAuthnCreateStub, unverifiedPasskeys);

    const banner = page.getByTestId('passkey-verification-banner');
    await expect(banner).toBeVisible();
    await expect(banner).toContainText('One of your passkeys needs an important update.');
    await expect(banner).toContainText("From December 31, 2026, signing in with a passkey will ask for your fingerprint, face, device password or security key PIN, and a passkey that can't will stop working.");

    await page.goto('/home');
    await expect(page.getByTestId('passkey-verification-banner')).toBeVisible();

    await page.getByTestId('passkey-verification-banner').getByRole('link', { name: 'See what to do' }).click();
    await expect(page).toHaveURL(/\/faq#passkey-pin$/);
    const entry = page.locator('#passkey-pin');
    await expect(entry).toBeInViewport();
    await expect(entry).toContainText('Nothing to do for passkeys on Apple devices');
    await expect(entry).toContainText('The PIN is stored on the key itself.');
    await expect(entry.getByRole('link', { name: "Google's guide" })).toHaveCount(0);
    await expect(entry.getByRole('link', { name: "Microsoft's guide" })).toHaveAttribute('href', 'https://support.microsoft.com/en-us/account-billing/sign-in-to-your-account-with-a-security-key-b23a2a45-6ab8-4c86-9f22-bcadf60235aa');
    await expect(entry.getByRole('link', { name: "Yubico's guide" })).toHaveAttribute('href', 'https://docs.yubico.com/software/yubikey/tools/authenticator/auth-guide/fido2.html');
  });

  // specRef: F16
  test('the deadline banner is a visible warning below the page header on a fresh load of any page', async ({ page }) => {
    await setup(page, { ...baseAccount, auth_methods: ['password', 'passkey'] }, installWebAuthnCreateStub, unverifiedPasskeys);

    for (const path of ['/blocklists', '/home']) {
      await page.goto(path);
      const banner = page.getByTestId('passkey-verification-banner');
      await expect(banner).toBeVisible();
      // Static page content, shown on every page: no live region to re-announce it.
      await expect(banner).not.toHaveAttribute('role');
      const top = (await banner.boundingBox())!.y;
      const headerBottom = await page.getByTestId('app-header-wrapper').evaluate(el => el.getBoundingClientRect().bottom);
      expect(top, path).toBeGreaterThanOrEqual(headerBottom);
      // Same width as the page content it sits in (both measured in one layout pass).
      const edges = await banner.evaluate(el => {
        const own = el.getBoundingClientRect();
        const parent = el.parentElement!.getBoundingClientRect();
        const style = getComputedStyle(el.parentElement!);
        return {
          left: Math.round(own.left), right: Math.round(own.right),
          contentLeft: Math.round(parent.left + parseFloat(style.paddingLeft)),
          contentRight: Math.round(parent.right - parseFloat(style.paddingRight)),
        };
      });
      expect(edges.left, path).toBe(edges.contentLeft);
      expect(edges.right, path).toBe(edges.contentRight);
    }

    const link = page.getByTestId('passkey-verification-banner').getByRole('link', { name: 'See what to do' });
    const rdns600 = await page.evaluate(() => {
      const probe = document.createElement('span');
      probe.style.color = 'var(--tailwind-colors-rdns-600)';
      document.body.appendChild(probe);
      const color = getComputedStyle(probe).color;
      probe.remove();
      return color;
    });
    await expect(link).toHaveCSS('color', rdns600);
  });

  // specRef: F7, F16
  test('no deadline banner when every passkey verified the user', async ({ page }) => {
    await setup(page, { ...baseAccount, auth_methods: ['password', 'passkey'] });

    await expect(page.getByTestId('passkey-item-p1')).toBeVisible();
    await expect(page.getByTestId('passkey-verification-banner')).toHaveCount(0);
  });
});
