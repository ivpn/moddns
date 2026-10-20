import { test, expect } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';
import { installWebAuthnSuccessStub } from '../utils/webauthn';

const mockAccount = { account_id: 'abc', email: 'user@example.com', email_verified: true, auth_methods: ['password'], mfa: { totp: { enabled: false } } };

/** Base mocks for the account page plus a PATCH /accounts interceptor. */
async function setupRoutes(page: import('@playwright/test').Page, onPatch: (body: string | null) => void) {
  await registerMocks(page, { authenticated: true, accountOverride: mockAccount });
  // Registered after registerMocks so these win over its catch-all.
  await page.route('**/api/v1/accounts', async route => {
    if (route.request().method() === 'PATCH') {
      onPatch(route.request().postData());
      return route.fulfill({ status: 200, body: '' });
    }
    return route.continue();
  });
  await page.route('**/api/v1/sub', route =>
    route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ status: 'active', plan: 'plus', active_until: '2027-01-01T00:00:00Z' }) }));
  await page.route('**/api/v1/webauthn/passkeys', route =>
    route.fulfill({ status: 200, contentType: 'application/json', body: '[]' }));
}

async function openUpdatePassword(page: import('@playwright/test').Page) {
  await page.goto('/account-preferences');
  const updateBtn = page.getByRole('button', { name: /Update password/i });
  await expect(updateBtn).toBeVisible({ timeout: 10_000 });
  await updateBtn.click();
}

// Intercepts account patch + current get to assert JSON Patch sequence
test('password update sends test+replace operations', async ({ page }) => {
  let patchPayload: { updates: { operation: string; path: string; value?: string }[] } | null = null;
  await setupRoutes(page, body => { patchPayload = body ? JSON.parse(body) : null; });
  await openUpdatePassword(page);

  // Fill fields
  await page.getByLabel('Old password').fill('OldPassword123!');
  await page.getByLabel('New password').fill('NewPassword123!');
  await page.getByLabel('Confirm password').fill('NewPassword123!');

  // Submit
  await page.getByRole('button', { name: /Save change/i }).click();

  // Wait for network interception
  await expect.poll(() => patchPayload).not.toBeNull();
  expect(Array.isArray(patchPayload.updates)).toBeTruthy();
  expect(patchPayload.updates).toHaveLength(2);
  expect(patchPayload.updates[0].operation).toBe('test');
  expect(patchPayload.updates[0].path).toBe('/password');
  expect(patchPayload.updates[1].operation).toBe('replace');
  expect(patchPayload.updates[1].path).toBe('/password');
});

// Negative flow: missing old password should not send patch
test('password update blocked without old password', async ({ page }) => {
  let patchCalled = false;
  await setupRoutes(page, () => { patchCalled = true; });
  await openUpdatePassword(page);
  await page.getByLabel('New password').fill('NewPassword123!');
  await page.getByLabel('Confirm password').fill('NewPassword123!');
  await page.getByRole('button', { name: /Save change/i }).click();

  // Short wait to allow potential request
  await page.waitForTimeout(500);
  expect(patchCalled).toBeFalsy();
});

// An account without a password confirms with a passkey before setting one (spec B3a).
// The navigator.credentials stub only takes effect in Chromium, hence @desktop like the login specs.
test('passkey-only account sets a first password after passkey verification', { tag: '@desktop' }, async ({ page }) => {
  let patchPayload: { updates: { operation: string; path: string; value?: unknown }[] } | null = null;
  let reauthPurpose: string | null = null;
  const passkeyOnlyAccount = { ...mockAccount, auth_methods: ['passkey'], mfa: { totp: { enabled: true } } };
  await installWebAuthnSuccessStub(page);
  await registerMocks(page, { authenticated: true });
  // Registered after registerMocks: its /api/v1/ catch-all would otherwise answer with an empty account.
  await page.route(/\/api\/v1\/accounts\/current(\/?|\?.*)$/i, route =>
    route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(passkeyOnlyAccount) }));
  await page.route('**/api/v1/accounts', async route => {
    if (route.request().method() === 'PATCH') {
      patchPayload = JSON.parse(route.request().postData() ?? 'null');
      return route.fulfill({ status: 200, body: '' });
    }
    return route.continue();
  });
  await page.route('**/api/v1/sub', route =>
    route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ status: 'active', plan: 'plus', active_until: '2027-01-01T00:00:00Z' }) }));
  await page.route('**/api/v1/webauthn/passkeys', route =>
    route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([{ id: 'p1', created_at: '2026-01-01T00:00:00Z' }]) }));
  await page.route('**/api/v1/webauthn/passkey/reauth/begin', route => {
    reauthPurpose = JSON.parse(route.request().postData() ?? '{}').purpose;
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ publicKey: { challenge: 'abc', rpId: 'localhost', timeout: 60000, userVerification: 'preferred', allowCredentials: [] } }) });
  });
  await page.route('**/api/v1/webauthn/passkey/reauth/finish', route =>
    route.fulfill({ status: 201, contentType: 'application/json', body: JSON.stringify({ reauth_token: 'reauth-token', expires_at: '2027-01-01T00:00:00Z' }) }));

  await openUpdatePassword(page);
  await expect(page.getByLabel('Old password')).toHaveCount(0);
  await page.getByRole('button', { name: 'Verify with passkey' }).click();
  await expect(page.getByRole('button', { name: 'Passkey verified' })).toBeVisible();
  await expect(page.getByLabel('2FA code')).toHaveCount(0);
  await page.getByLabel('New password').fill('NewPassword123!');
  await page.getByLabel('Confirm password').fill('NewPassword123!');
  await page.getByRole('button', { name: /Save change/i }).click();

  await expect.poll(() => patchPayload).not.toBeNull();
  expect(reauthPurpose).toBe('password_set');
  expect(patchPayload!.updates).toEqual([
    { operation: 'test', path: '/password', value: { reauth_token: 'reauth-token' } },
    { operation: 'replace', path: '/password', value: 'NewPassword123!' },
  ]);
});
