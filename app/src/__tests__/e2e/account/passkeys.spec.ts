import { test, expect } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';
import { installWebAuthnCreateStub, installWebAuthnSuccessStub } from '../utils/webauthn';

// The navigator.credentials stubs only take effect in Chromium, hence @desktop like the login specs.
test.describe('Passkey management', { tag: '@desktop' }, () => {
  const baseAccount = { account_id: 'abc', email: 'user@example.com', email_verified: true, mfa: { totp: { enabled: false } } };
  const creationOptions = { publicKey: { challenge: 'abc', rp: { id: 'localhost', name: 'modDNS' }, user: { id: 'dXNlcg', name: 'user@example.com', displayName: 'user@example.com' }, pubKeyCredParams: [{ type: 'public-key', alg: -7 }], timeout: 60000 } };
  const requestOptions = { publicKey: { challenge: 'abc', rpId: 'localhost', timeout: 60000, userVerification: 'preferred', allowCredentials: [] } };

  type Captured = { addBegin: { body: unknown; mfaCode?: string } | null; reauthPurpose: string | null; addFinished: boolean };

  async function setup(page: import('@playwright/test').Page, account: Record<string, unknown>): Promise<Captured> {
    const captured: Captured = { addBegin: null, reauthPurpose: null, addFinished: false };
    await installWebAuthnSuccessStub(page);
    await installWebAuthnCreateStub(page);
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
    await dialog.getByRole('button', { name: 'Password' }).click();
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
});
