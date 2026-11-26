import { test, expect, type Page } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';
import { installWebAuthnSuccessStub } from '../utils/webauthn';

const PASSWORD = 'Curr3nt-Passw0rd!';

type Sent = { body?: object; mfaCode?: string; reauthPurpose?: string };

/** The account page with the sessions request recorded and answered with `answer`. */
async function openDialog(page: Page, { totp = false, passkeys = [] as object[], answer = { status: 204, body: undefined as object | undefined } } = {}) {
  const sent: Sent = {};
  const account = { id: 'abc', email: 'user@example.com', email_verified: true, auth_methods: ['password'], mfa: { totp: { enabled: totp } } };
  await registerMocks(page, { authenticated: true, accountOverride: account });
  // registerMocks' catch-all answers /accounts/current itself; register the account after it.
  await page.route(/\/api\/v1\/accounts\/current(\/?|\?.*)$/i, route =>
    route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(account) }));
  await page.route('**/api/v1/sub', route =>
    route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ status: 'active', plan: 'plus', active_until: '2027-01-01T00:00:00Z' }) }));
  await page.route('**/api/v1/webauthn/passkeys', route =>
    route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(passkeys) }));
  await page.route('**/api/v1/webauthn/passkey/reauth/begin', route => {
    sent.reauthPurpose = JSON.parse(route.request().postData() ?? '{}').purpose;
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ publicKey: { challenge: 'abc', rpId: 'localhost', timeout: 60000, userVerification: 'preferred', allowCredentials: [] } }) });
  });
  await page.route('**/api/v1/webauthn/passkey/reauth/finish', route =>
    route.fulfill({ status: 201, contentType: 'application/json', body: JSON.stringify({ reauth_token: 'reauth-token', expires_at: '2027-01-01T00:00:00Z' }) }));
  await page.route(/\/api\/v1\/sessions(\/?|\?.*)$/i, route => {
    if (route.request().method() !== 'DELETE') return route.fallback();
    sent.body = route.request().postDataJSON();
    sent.mfaCode = route.request().headers()['x-mfa-code'];
    return route.fulfill({ status: answer.status, contentType: 'application/json', body: answer.body ? JSON.stringify(answer.body) : '' });
  });

  await page.goto('/account-preferences');
  await page.getByRole('button', { name: /Log out web sessions/i }).click({ timeout: 10_000 });
  return { sent, dialog: page.getByTestId('delete-sessions-dialog') };
}

// Signing out the other sessions needs the password (plus the 2FA code when it is on) or a passkey.
// specRef: api-endpoint-behaviour A3, F5
test.describe('log out other sessions', { tag: '@desktop' }, () => {
  test('asks for the password first', async ({ page }) => {
    const { sent, dialog } = await openDialog(page);
    const submit = dialog.getByRole('button', { name: 'Log out sessions' });
    await expect(submit).toBeDisabled();
    await dialog.getByLabel('Current password').fill(PASSWORD);
    await submit.click();
    await expect(page.getByText('All other sessions have been logged out.')).toBeVisible();
    expect(sent.body).toEqual({ current_password: PASSWORD });
    expect(sent.mfaCode).toBeUndefined();
  });

  test('with 2FA on, the code goes with the password', async ({ page }) => {
    const { sent, dialog } = await openDialog(page, { totp: true });
    await dialog.getByLabel('Current password').fill(PASSWORD);
    await expect(dialog.getByRole('button', { name: 'Log out sessions' })).toBeDisabled();
    await dialog.getByLabel('2FA code').fill('123456');
    await dialog.getByRole('button', { name: 'Log out sessions' }).click();
    await expect(page.getByText('All other sessions have been logged out.')).toBeVisible();
    expect(sent.body).toEqual({ current_password: PASSWORD, otp: '123456' });
    expect(sent.mfaCode).toBeUndefined();
  });

  test('a passkey replaces the password', async ({ page }) => {
    await installWebAuthnSuccessStub(page);
    const { sent, dialog } = await openDialog(page, { passkeys: [{ id: 'p1', created_at: '2026-01-01T00:00:00Z' }] });
    await dialog.getByRole('button', { name: 'Verify with passkey' }).click();
    await expect(dialog.getByRole('button', { name: 'Passkey verified' })).toBeVisible();
    await dialog.getByRole('button', { name: 'Log out sessions' }).click();
    await expect(page.getByText('All other sessions have been logged out.')).toBeVisible();
    expect(sent.body).toEqual({ reauth_token: 'reauth-token' });
    expect(sent.reauthPurpose).toBe('sessions_delete');
  });

  test('a refused password keeps the dialog and the session', async ({ page }) => {
    const { dialog } = await openDialog(page, { answer: { status: 400, body: { error: 'invalid current password' } } });
    await dialog.getByLabel('Current password').fill('wrong');
    await dialog.getByRole('button', { name: 'Log out sessions' }).click();
    await expect(page.getByText('invalid current password')).toBeVisible();
    await expect(dialog).toBeVisible();
    await expect(page).toHaveURL(/\/account-preferences/);
  });
});
