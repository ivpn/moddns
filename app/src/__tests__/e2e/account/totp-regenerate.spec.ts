import { test, expect, type Page } from '@playwright/test';
import { mockPasswordReauth, PASSWORD_REAUTH_TOKEN, registerMocks, type PasswordReauthMock } from '../../mocks/registerMocks';
import { formatBackupCode } from '../../../lib/backupCodes';
import { installWebAuthnSuccessStub } from '../utils/webauthn';

const NEW_CODES = [
  'GFUKSZ7KZBVPNDNTMWMSHAKR5', 'M5JUDDGRRKJYDYKZUXMQA9XND', 'WMWMWMWMWMWMWMWMWMWMWMWMW', 'Q8PTTJFA62Y5ATFUATQJE2WMU',
  'AUGN5NLMRNP4FJJPE7GSTJHXS', 'SEZKMSVCAYXSTBHNPJBJZKXJD', 'K2YW79H8UKFNC2CPK9MV8GPRL', 'HJKLMNPQRSTUVWXYZ23456789',
];
const LEGACY_CODE = 'LegacyCodeAAAAA1';

const PASSWORD = 'Curr3nt-Passw0rd!';

type Totp = { enabled: boolean; backup_codes_remaining?: number; backup_codes_legacy?: boolean };
type RegenerateBody = { otp: string; reauth_token: string };

/** The account page for an account with 2FA on; /accounts/current answers with the latest `totp`. */
async function openAccount(page: Page, initial: Totp, regenerate?: { status: number; body: object }, passkeys: object[] = []) {
  const state = { totp: initial, sentCode: undefined as string | undefined, sent: undefined as RegenerateBody | undefined, reauthPurpose: undefined as string | undefined, passwordReauth: undefined as unknown as PasswordReauthMock };
  const account = () => ({ id: 'abc', email: 'user@example.com', email_verified: true, auth_methods: ['password'], mfa: { totp: state.totp } });
  await registerMocks(page, { authenticated: true, accountOverride: account() });
  // registerMocks' catch-all answers /accounts/current itself; register the account after it.
  await page.route(/\/api\/v1\/accounts\/current(\/?|\?.*)$/i, route =>
    route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(account()) }));
  await page.route('**/api/v1/sub', route =>
    route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ status: 'active', plan: 'plus', active_until: '2027-01-01T00:00:00Z' }) }));
  await page.route('**/api/v1/webauthn/passkeys', route =>
    route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(passkeys) }));
  await page.route('**/api/v1/webauthn/passkey/reauth/begin', route => {
    state.reauthPurpose = JSON.parse(route.request().postData() ?? '{}').purpose;
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ publicKey: { challenge: 'abc', rpId: 'localhost', timeout: 60000, userVerification: 'preferred', allowCredentials: [] } }) });
  });
  await page.route('**/api/v1/webauthn/passkey/reauth/finish', route =>
    route.fulfill({ status: 201, contentType: 'application/json', body: JSON.stringify({ reauth_token: 'reauth-token', expires_at: '2027-01-01T00:00:00Z' }) }));
  state.passwordReauth = await mockPasswordReauth(page);
  await page.route('**/api/v1/accounts/mfa/totp/backup-codes/regenerate', route => {
    state.sent = route.request().postDataJSON() as RegenerateBody;
    state.sentCode = state.sent.otp;
    const res = regenerate ?? { status: 200, body: { backup_codes: NEW_CODES } };
    if (res.status === 200) state.totp = { enabled: true, backup_codes_remaining: 8, backup_codes_legacy: false };
    return route.fulfill({ status: res.status, contentType: 'application/json', body: JSON.stringify(res.body) });
  });
  await page.goto('/account-preferences');
  return state;
}

// specRef: api-endpoint-behaviour E13, E14
test.describe('legacy backup codes', () => {
  test('the banner leads to a new set, after which it is gone', async ({ page }) => {
    const state = await openAccount(page, { enabled: true, backup_codes_remaining: 6, backup_codes_legacy: true });

    const banner = page.getByTestId('legacy-backup-codes-banner');
    await expect(banner).toBeVisible({ timeout: 10_000 });
    await expect(banner).toContainText('We strongly advise regenerating your 2FA backup codes.');
    await expect(page.getByText('Your backup codes are of an older format. We strongly advise regenerating them.')).toBeVisible();

    await page.getByTestId('legacy-backup-codes-banner-regenerate').click();
    const dialog = page.getByTestId('regenerate-backup-codes-dialog');
    await expect(dialog).toBeVisible();
    await expect(dialog.getByTestId('regenerate-backup-codes-submit')).toBeDisabled();

    // Lost authenticator: the password and a legacy backup code authorise the new set. The token
    // is issued for the password alone; the code goes only to the regenerate request.
    await dialog.getByLabel('Current password').fill(PASSWORD);
    await dialog.getByTestId('regenerate-backup-codes-code-mode-toggle').click();
    await dialog.getByLabel('Backup code').fill(LEGACY_CODE);
    await dialog.getByTestId('regenerate-backup-codes-submit').click();
    await expect.poll(() => state.sent).toEqual({ otp: LEGACY_CODE, reauth_token: PASSWORD_REAUTH_TOKEN });
    expect(state.passwordReauth.sent).toEqual([{ purpose: 'backup_codes_regenerate', current_password: PASSWORD }]);

    await expect(dialog.getByTestId('backup-code')).toHaveText(NEW_CODES.map(formatBackupCode));
    // The codes are shown only once: Escape does not close the dialog before Done.
    await page.keyboard.press('Escape');
    await expect(dialog).toBeVisible();
    await dialog.getByRole('button', { name: 'Done' }).click();
    await expect(dialog).toBeHidden();

    await expect(banner).toBeHidden();
    await expect(page.getByText('8 of 8 backup codes left.', { exact: false })).toBeVisible();
  });

  test('closing the banner keeps the reminder in the backup codes row', async ({ page }) => {
    await openAccount(page, { enabled: true, backup_codes_remaining: 8, backup_codes_legacy: true });
    await page.getByTestId('legacy-backup-codes-banner-close').click();
    await expect(page.getByTestId('legacy-backup-codes-banner')).toBeHidden();
    await expect(page.getByText('Your backup codes are of an older format. We strongly advise regenerating them.')).toBeVisible();
    await page.reload();
    await expect(page.getByTestId('btn-regenerate-backup-codes')).toBeVisible({ timeout: 10_000 });
    await expect(page.getByTestId('legacy-backup-codes-banner')).toBeHidden();
  });
});

// specRef: api-endpoint-behaviour E13, E14
test.describe('backup codes row', () => {
  test('suggests a new set when few codes are left and regenerates with an authenticator code', async ({ page }) => {
    const state = await openAccount(page, { enabled: true, backup_codes_remaining: 2, backup_codes_legacy: false });
    await expect(page.getByText('Only 2 of 8 backup codes left. Regenerate them to get a fresh set.')).toBeVisible({ timeout: 10_000 });
    await expect(page.getByTestId('legacy-backup-codes-banner')).toHaveCount(0);

    await page.getByTestId('btn-regenerate-backup-codes').click();
    const dialog = page.getByTestId('regenerate-backup-codes-dialog');
    await dialog.getByLabel('Current password').fill(PASSWORD);
    const input = dialog.getByLabel('2FA code');
    await expect(input).toHaveAttribute('inputmode', 'numeric');
    await input.fill('123 456');
    await input.press('Enter');
    await expect.poll(() => state.sentCode).toBe('123 456');
    await expect(dialog.getByTestId('backup-code')).toHaveCount(8);
  });

  test('is absent while 2FA is off', async ({ page }) => {
    await openAccount(page, { enabled: false });
    await expect(page.getByRole('radio', { name: /^Enable$/ }).first()).toBeVisible({ timeout: 10_000 });
    await expect(page.getByTestId('btn-regenerate-backup-codes')).toHaveCount(0);
  });

  test('a refused code keeps the user signed in and the old set in place', async ({ page }) => {
    await openAccount(page, { enabled: true, backup_codes_remaining: 5 }, { status: 400, body: { error: 'invalid 2FA code' } });
    await page.getByTestId('btn-regenerate-backup-codes').click({ timeout: 10_000 });
    const dialog = page.getByTestId('regenerate-backup-codes-dialog');
    await dialog.getByLabel('Current password').fill(PASSWORD);
    await dialog.getByLabel('2FA code').fill('123456');
    await dialog.getByTestId('regenerate-backup-codes-submit').click();

    await expect(page.getByText('invalid 2FA code')).toBeVisible();
    await expect(dialog.getByLabel('2FA code')).toBeVisible();
    await expect(page).toHaveURL(/\/account-preferences/);
  });

  test('asks for the password as well as the code', async ({ page }) => {
    const state = await openAccount(page, { enabled: true, backup_codes_remaining: 5 });
    await page.getByTestId('btn-regenerate-backup-codes').click({ timeout: 10_000 });
    const dialog = page.getByTestId('regenerate-backup-codes-dialog');
    const submit = dialog.getByTestId('regenerate-backup-codes-submit');
    await dialog.getByLabel('2FA code').fill('123456');
    await expect(submit).toBeDisabled();
    await dialog.getByLabel('2FA code').press('Enter');
    expect(state.sent).toBeUndefined();
    await dialog.getByLabel('Current password').fill(PASSWORD);
    await expect(submit).toBeEnabled();
    await submit.click();
    await expect.poll(() => state.sent).toEqual({ otp: '123456', reauth_token: PASSWORD_REAUTH_TOKEN });
    expect(state.passwordReauth.sent).toEqual([{ purpose: 'backup_codes_regenerate', current_password: PASSWORD }]);
  });

  // The navigator.credentials stub only takes effect in Chromium.
  test('a passkey replaces the password, the code is still asked for', { tag: '@desktop' }, async ({ page }) => {
    await installWebAuthnSuccessStub(page);
    const state = await openAccount(page, { enabled: true, backup_codes_remaining: 5 }, undefined, [{ id: 'p1', created_at: '2026-01-01T00:00:00Z' }]);
    await page.getByTestId('btn-regenerate-backup-codes').click({ timeout: 10_000 });
    const dialog = page.getByTestId('regenerate-backup-codes-dialog');
    await dialog.getByRole('button', { name: 'Verify with passkey' }).click();
    await expect(dialog.getByRole('button', { name: 'Passkey verified' })).toBeVisible();
    await expect(dialog.getByTestId('regenerate-backup-codes-submit')).toBeDisabled();
    await dialog.getByLabel('2FA code').fill('123456');
    await dialog.getByTestId('regenerate-backup-codes-submit').click();
    await expect.poll(() => state.sent).toEqual({ otp: '123456', reauth_token: 'reauth-token' });
    expect(state.reauthPurpose).toBe('backup_codes_regenerate');
    await expect(dialog.getByTestId('backup-code')).toHaveCount(8);
  });

  // The server uses the passkey token up with the first try, right or wrong code.
  test('after a refused code, the passkey is asked for again and the code is kept', { tag: '@desktop' }, async ({ page }) => {
    await installWebAuthnSuccessStub(page);
    await openAccount(page, { enabled: true, backup_codes_remaining: 5 }, undefined, [{ id: 'p1', created_at: '2026-01-01T00:00:00Z' }]);
    let issued = 0;
    const sent: RegenerateBody[] = [];
    await page.route('**/api/v1/webauthn/passkey/reauth/finish', route =>
      route.fulfill({ status: 201, contentType: 'application/json', body: JSON.stringify({ reauth_token: `reauth-token-${++issued}`, expires_at: '2027-01-01T00:00:00Z' }) }));
    await page.route('**/api/v1/accounts/mfa/totp/backup-codes/regenerate', route => {
      sent.push(route.request().postDataJSON() as RegenerateBody);
      return sent.length === 1
        ? route.fulfill({ status: 400, contentType: 'application/json', body: JSON.stringify({ error: 'invalid 2FA code' }) })
        : route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ backup_codes: NEW_CODES }) });
    });

    await page.getByTestId('btn-regenerate-backup-codes').click({ timeout: 10_000 });
    const dialog = page.getByTestId('regenerate-backup-codes-dialog');
    await dialog.getByRole('button', { name: 'Verify with passkey' }).click();
    await dialog.getByLabel('2FA code').fill('111111');
    await dialog.getByTestId('regenerate-backup-codes-submit').click();
    await expect(page.getByText('invalid 2FA code')).toBeVisible();

    await expect(dialog.getByRole('button', { name: 'Verify with passkey' })).toBeEnabled();
    await expect(dialog.getByTestId('regenerate-backup-codes-submit')).toBeDisabled();
    await expect(dialog.getByLabel('2FA code')).toHaveValue('111111');
    await dialog.getByLabel('2FA code').fill('123456');
    await dialog.getByRole('button', { name: 'Verify with passkey' }).click();
    await dialog.getByTestId('regenerate-backup-codes-submit').click();
    await expect(dialog.getByTestId('backup-code')).toHaveCount(8);
    expect(sent).toEqual([{ otp: '111111', reauth_token: 'reauth-token-1' }, { otp: '123456', reauth_token: 'reauth-token-2' }]);
  });

  test('a malformed code is caught before it is sent', async ({ page }) => {
    const state = await openAccount(page, { enabled: true, backup_codes_remaining: 5 });
    await page.getByTestId('btn-regenerate-backup-codes').click({ timeout: 10_000 });
    const dialog = page.getByTestId('regenerate-backup-codes-dialog');
    await dialog.getByTestId('regenerate-backup-codes-code-mode-toggle').click();
    const input = dialog.getByLabel('Backup code');
    await input.fill('GFUKS-Z7KZB-VPNDN');
    await input.blur();
    await expect(dialog.getByText("This doesn't look like a backup code. Check it and try again.")).toBeVisible();
    await expect(dialog.getByTestId('regenerate-backup-codes-submit')).toBeDisabled();
    expect(state.sentCode).toBeUndefined();
  });
});

// Every screen that asks for a 2FA code opens on the numeric keypad and offers the backup-code field.
// specRef: api-endpoint-behaviour A3, E6
test.describe('2FA code fields on the account page', () => {
  const screens: { name: string; open: (page: Page) => Promise<void>; testId: string }[] = [
    {
      name: 'disable 2FA',
      testId: 'disable-2fa-code',
      open: async page => { await page.getByRole('radio', { name: /^Disable$/ }).first().click(); },
    },
    {
      name: 'regenerate backup codes',
      testId: 'regenerate-backup-codes-code',
      open: async page => { await page.getByTestId('btn-regenerate-backup-codes').click(); },
    },
    {
      name: 'update password',
      testId: 'update-password-code',
      open: async page => { await page.getByRole('button', { name: /Update password/i }).click(); },
    },
    {
      name: 'change email',
      testId: 'change-email-code',
      open: async page => { await page.getByRole('button', { name: /Change email/i }).click(); },
    },
    {
      name: 'log out other sessions',
      testId: 'delete-sessions-code',
      open: async page => { await page.getByRole('button', { name: /Log out web sessions/i }).click(); },
    },
    {
      name: 'export profiles',
      testId: 'export-code',
      open: async page => { await page.getByTestId('btn-export-profiles').click(); },
    },
    {
      name: 'delete account',
      testId: 'delete-account-code',
      open: async page => { await page.getByRole('button', { name: /^Delete account$/ }).click(); },
    },
  ];

  for (const screen of screens) {
    test(screen.name, async ({ page }) => {
      await openAccount(page, { enabled: true, backup_codes_remaining: 8 });
      await expect(page.getByTestId('btn-regenerate-backup-codes')).toBeVisible({ timeout: 10_000 });
      await screen.open(page);

      const input = page.getByTestId(screen.testId);
      await expect(input).toHaveAttribute('inputmode', 'numeric');
      await page.getByTestId(`${screen.testId}-mode-toggle`).click();
      await expect(input).toHaveAttribute('inputmode', 'text');
      await expect(input).toHaveAttribute('autocapitalize', 'none');
      await expect(input).toBeFocused();
      await input.fill(formatBackupCode(NEW_CODES[0]));
      await input.blur();
      await expect(page.getByTestId(`${screen.testId}-hint`)).toHaveText('One of your saved backup codes. Each code works once.');
    });
  }
});
