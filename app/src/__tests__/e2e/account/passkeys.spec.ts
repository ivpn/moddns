import { test, expect } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';
import { installWebAuthnCreateExcludedStub, installWebAuthnCreateStub, installWebAuthnSuccessStub } from '../utils/webauthn';

// The navigator.credentials stubs only take effect in Chromium, hence @desktop like the login specs.
test.describe('Passkey management', { tag: '@desktop' }, () => {
  const baseAccount = { account_id: 'abc', email: 'user@example.com', email_verified: true, mfa: { totp: { enabled: false } } };
  const creationOptions = { publicKey: { challenge: 'abc', rp: { id: 'localhost', name: 'modDNS' }, user: { id: 'dXNlcg', name: 'user@example.com', displayName: 'user@example.com' }, pubKeyCredParams: [{ type: 'public-key', alg: -7 }], timeout: 60000 } };
  const requestOptions = { publicKey: { challenge: 'abc', rpId: 'localhost', timeout: 60000, userVerification: 'preferred', allowCredentials: [] } };

  type Captured = { addBegin: { body: unknown; mfaCode?: string } | null; reauthPurpose: string | null; addFinished: boolean; deleted: { id: string; body: unknown; mfaCode?: string } | null };

  type Page = import('@playwright/test').Page;

  async function setup(page: Page, account: Record<string, unknown>, installCreateStub: (page: Page) => Promise<void> = installWebAuthnCreateStub): Promise<Captured> {
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
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([{ id: 'p1', created_at: '2026-01-01T00:00:00Z' }]) }));
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
});
