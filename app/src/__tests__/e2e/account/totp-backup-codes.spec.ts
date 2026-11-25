import { test, expect, type Page } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';
import { formatBackupCode } from '../../../lib/backupCodes';
import { TOTP_TOO_MANY_ATTEMPTS_MESSAGE } from '../../../lib/authToasts';

// Backup codes are 25 characters over the 32-symbol uppercase alphabet (125 bits).
const BACKUP_CODES = [
  'GFUKSZ7KZBVPNDNTMWMSHAKR5', 'M5JUDDGRRKJYDYKZUXMQA9XND', 'WMWMWMWMWMWMWMWMWMWMWMWMW', 'Q8PTTJFA62Y5ATFUATQJE2WMU',
  'AUGN5NLMRNP4FJJPE7GSTJHXS', 'SEZKMSVCAYXSTBHNPJBJZKXJD', 'K2YW79H8UKFNC2CPK9MV8GPRL', 'HJKLMNPQRSTUVWXYZ23456789',
];

async function ensurePasswordMode(page: Page) {
  await page.getByTestId('login-page').waitFor();
  if (await page.getByTestId('login-passkey-form').count()) {
    await page.getByTestId('btn-login-toggle-mode').click();
  }
}

async function submitPasswordLogin(page: Page) {
  await page.goto('/login');
  await ensurePasswordMode(page);
  await page.getByTestId('input-email').fill('user@example.com');
  await page.getByTestId('input-password').fill('secret');
  await page.getByTestId('btn-login-password-submit').click();
  await expect(page.getByTestId('input-otp')).toBeVisible();
}

test('enable 2FA shows every backup code grouped and without clipping', async ({ page }) => {
  await registerMocks(page, {
    authenticated: true,
    accountOverride: { id: 'abc', email: 'user@example.com', email_verified: true, auth_methods: ['password'], mfa: { totp: { enabled: false } } },
  });
  await page.route('**/api/v1/sub', route =>
    route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ status: 'active', plan: 'plus', active_until: '2027-01-01T00:00:00Z' }) }));
  await page.route('**/api/v1/webauthn/passkeys', route =>
    route.fulfill({ status: 200, contentType: 'application/json', body: '[]' }));
  await page.route('**/api/v1/accounts/mfa/totp/enable', route =>
    route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ secret: 'JBSWY3DPEHPK3PXPJBSWY3DPEHPK3PXP', uri: 'otpauth://totp/modDNS:user@example.com?secret=JBSWY3DPEHPK3PXPJBSWY3DPEHPK3PXP&issuer=modDNS', account: 'user@example.com' }) }));
  await page.route('**/api/v1/accounts/mfa/totp/enable/confirm', route =>
    route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ backup_codes: BACKUP_CODES }) }));

  await page.goto('/account-preferences');
  const toggle = page.getByRole('radio', { name: /^Enable$/ }).first();
  await expect(toggle).toBeVisible({ timeout: 10_000 });
  await toggle.click();
  await page.locator('#totp-code').fill('123456');
  await page.getByRole('button', { name: /Enable 2FA/i }).click();

  const cells = page.getByTestId('backup-code');
  await expect(cells).toHaveText(BACKUP_CODES.map(formatBackupCode));
  for (const cell of await cells.all()) {
    const fits = await cell.evaluate(el => el.scrollWidth <= el.clientWidth);
    expect(fits, `backup code ${await cell.textContent()} is clipped`).toBe(true);
  }
});

test.describe('2FA login', { tag: '@desktop' }, () => {
  test('a grouped backup code is sent whole', async ({ page }) => {
    let sentCode: string | undefined;
    let attempt = 0;
    await page.route(/\/api\/v1\/login(\/?|\?.*)$/i, async route => {
      if (route.request().method() !== 'POST') return route.continue();
      attempt++;
      if (attempt === 1) {
        return route.fulfill({ status: 401, contentType: 'application/json', body: JSON.stringify({ error: 'TOTP is required' }) });
      }
      sentCode = route.request().headers()['x-mfa-code'];
      return route.fulfill({ status: 401, contentType: 'application/json', body: JSON.stringify({ error: 'invalid 2FA code' }) });
    });

    await submitPasswordLogin(page);
    await page.getByTestId('input-otp-mode-toggle').click();
    // Phone keyboards must not autocorrect or capitalise a typed code.
    const otpInput = page.getByTestId('input-otp');
    await expect(otpInput).toHaveAttribute('inputmode', 'text');
    await expect(otpInput).toHaveAttribute('autocorrect', 'off');
    await expect(otpInput).toHaveAttribute('autocapitalize', 'none');
    await expect(otpInput).toHaveAttribute('spellcheck', 'false');

    const grouped = formatBackupCode(BACKUP_CODES[0]);
    await page.getByTestId('input-otp').fill(grouped);
    await expect(page.getByTestId('input-otp')).toHaveValue(grouped);
    await page.getByTestId('btn-login-password-submit').click();
    await expect.poll(() => sentCode).toBe(grouped);
  });
});

// Someone who lost the authenticator turns 2FA off with a backup code, then enrols again.
test.describe('backup code recovery', { tag: '@desktop' }, () => {
  test('disable 2FA accepts a grouped backup code', async ({ page }) => {
    let sentCode: string | undefined;
    const account = { id: 'abc', email: 'user@example.com', email_verified: true, auth_methods: ['password'], mfa: { totp: { enabled: true } } };
    await registerMocks(page, { authenticated: true, accountOverride: account });
    // registerMocks' catch-all answers /accounts/current itself; register the account after it.
    await page.route(/\/api\/v1\/accounts\/current(\/?|\?.*)$/i, route =>
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(account) }));
    await page.route('**/api/v1/sub', route =>
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ status: 'active', plan: 'plus', active_until: '2027-01-01T00:00:00Z' }) }));
    await page.route('**/api/v1/webauthn/passkeys', route =>
      route.fulfill({ status: 200, contentType: 'application/json', body: '[]' }));
    await page.route('**/api/v1/accounts/mfa/totp/disable', route => {
      sentCode = (route.request().postDataJSON() as { otp: string }).otp;
      return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ id: 'abc', mfa: { totp: { enabled: false } } }) });
    });

    await page.goto('/account-preferences');
    const toggle = page.getByRole('radio', { name: /^Disable$/ }).first();
    await expect(toggle).toBeVisible({ timeout: 10_000 });
    await toggle.click();

    const input = page.locator('#disable-totp-code');
    await expect(input).toHaveAttribute('inputmode', 'numeric');
    await page.getByTestId('disable-2fa-code-mode-toggle').click();
    await expect(input).toHaveAttribute('inputmode', 'text');
    await expect(input).toHaveAttribute('autocorrect', 'off');
    await expect(input).toHaveAttribute('autocapitalize', 'none');
    await expect(input).toHaveAttribute('spellcheck', 'false');

    const grouped = formatBackupCode(BACKUP_CODES[0]);
    await input.fill(grouped);
    await expect(input).toHaveValue(grouped);
    await page.getByRole('button', { name: /^Disable 2FA$/ }).click();
    await expect.poll(() => sentCode).toBe(grouped);
  });

  test('a wrong code keeps the user signed in and shows why', async ({ page }) => {
    const account = { id: 'abc', email: 'user@example.com', email_verified: true, auth_methods: ['password'], mfa: { totp: { enabled: true } } };
    await registerMocks(page, { authenticated: true, accountOverride: account });
    await page.route(/\/api\/v1\/accounts\/current(\/?|\?.*)$/i, route =>
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(account) }));
    await page.route('**/api/v1/accounts/mfa/totp/disable', route =>
      route.fulfill({ status: 400, contentType: 'application/json', body: JSON.stringify({ error: 'invalid 2FA code' }) }));

    await page.goto('/account-preferences');
    await page.getByRole('radio', { name: /^Disable$/ }).first().click();
    await page.getByTestId('disable-2fa-code-mode-toggle').click();
    await page.locator('#disable-totp-code').fill(formatBackupCode(BACKUP_CODES[2]));
    await page.getByRole('button', { name: /^Disable 2FA$/ }).click();

    await expect(page.getByText('invalid 2FA code')).toBeVisible();
    await expect(page.locator('#disable-totp-code')).toBeVisible();
    await expect(page).toHaveURL(/\/account-preferences/);
  });

  test('password reset accepts a grouped backup code', async ({ page }) => {
    let sentCode: string | undefined;
    await page.route('**/api/v1/verify/reset-password', route => {
      const code = route.request().headers()['x-mfa-code'];
      if (!code) {
        return route.fulfill({ status: 401, contentType: 'application/json', body: JSON.stringify({ error: 'TOTP is required' }) });
      }
      sentCode = code;
      return route.fulfill({ status: 204, body: '' });
    });

    await page.goto('/reset-password/reset-token');
    await page.getByPlaceholder('New password').fill('N3w-Passw0rd-Long!');
    await page.getByPlaceholder('Confirm password').fill('N3w-Passw0rd-Long!');
    await page.getByRole('button', { name: /^Reset password$/ }).click();

    await page.getByTestId('reset-password-code-mode-toggle').click();
    const input = page.getByPlaceholder('Backup code');
    await expect(input).toBeVisible();
    await expect(input).toHaveAttribute('inputmode', 'text');
    await expect(input).toHaveAttribute('autocorrect', 'off');
    await expect(input).toHaveAttribute('autocapitalize', 'none');

    const grouped = formatBackupCode(BACKUP_CODES[1]);
    await input.fill(grouped);
    await expect(input).toHaveValue(grouped);
    await page.getByRole('button', { name: /^Reset password$/ }).click();
    await expect.poll(() => sentCode).toBe(grouped);
    await expect(page).toHaveURL(/\/login/);
  });

  test('password reset reports a wrong 2FA code', async ({ page }) => {
    await page.route('**/api/v1/verify/reset-password', route => {
      const code = route.request().headers()['x-mfa-code'];
      const error = code ? 'invalid 2FA code' : 'TOTP is required';
      return route.fulfill({ status: 401, contentType: 'application/json', body: JSON.stringify({ error }) });
    });

    await page.goto('/reset-password/reset-token');
    await page.getByPlaceholder('New password').fill('N3w-Passw0rd-Long!');
    await page.getByPlaceholder('Confirm password').fill('N3w-Passw0rd-Long!');
    await page.getByRole('button', { name: /^Reset password$/ }).click();
    await page.getByPlaceholder('2FA code').fill('123456');
    await page.getByRole('button', { name: /^Reset password$/ }).click();
    await expect(page.getByText(/invalid 2FA code/i)).toBeVisible();
  });
});

const LOCKOUT = { error: 'too many failed 2FA attempts, try again later' };

async function openEnable2FA(page: Page, confirm: { status: number; body: object }) {
  await registerMocks(page, {
    authenticated: true,
    accountOverride: { id: 'abc', email: 'user@example.com', email_verified: true, auth_methods: ['password'], mfa: { totp: { enabled: false } } },
  });
  await page.route('**/api/v1/sub', route =>
    route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ status: 'active', plan: 'plus', active_until: '2027-01-01T00:00:00Z' }) }));
  await page.route('**/api/v1/webauthn/passkeys', route =>
    route.fulfill({ status: 200, contentType: 'application/json', body: '[]' }));
  await page.route('**/api/v1/accounts/mfa/totp/enable', route =>
    route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ secret: 'JBSWY3DPEHPK3PXPJBSWY3DPEHPK3PXP', uri: 'otpauth://totp/modDNS:user@example.com?secret=JBSWY3DPEHPK3PXPJBSWY3DPEHPK3PXP&issuer=modDNS', account: 'user@example.com' }) }));
  await page.route('**/api/v1/accounts/mfa/totp/enable/confirm', route =>
    route.fulfill({ status: confirm.status, contentType: 'application/json', body: JSON.stringify(confirm.body) }));

  await page.goto('/account-preferences');
  const toggle = page.getByRole('radio', { name: /^Enable$/ }).first();
  await expect(toggle).toBeVisible({ timeout: 10_000 });
  await toggle.click();
  await page.locator('#totp-code').fill('123456');
  await page.getByRole('button', { name: /Enable 2FA/i }).click();
}

test.describe('refused 2FA codes', { tag: '@desktop' }, () => {
  test('set-up: a wrong code keeps the user signed in and shows why', async ({ page }) => {
    await openEnable2FA(page, { status: 400, body: { error: 'incorrect OTP' } });
    await expect(page.getByText('incorrect OTP')).toBeVisible();
    await expect(page.locator('#totp-code')).toBeVisible();
    await expect(page).toHaveURL(/\/account-preferences/);
  });

  // The lockout reads the same at set-up, disable, password reset and login.
  test('set-up: lockout', async ({ page }) => {
    await openEnable2FA(page, { status: 429, body: LOCKOUT });
    await expect(page.getByText(TOTP_TOO_MANY_ATTEMPTS_MESSAGE)).toBeVisible();
  });

  test('disable: lockout', async ({ page }) => {
    const account = { id: 'abc', email: 'user@example.com', email_verified: true, auth_methods: ['password'], mfa: { totp: { enabled: true } } };
    await registerMocks(page, { authenticated: true, accountOverride: account });
    await page.route(/\/api\/v1\/accounts\/current(\/?|\?.*)$/i, route =>
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(account) }));
    await page.route('**/api/v1/accounts/mfa/totp/disable', route =>
      route.fulfill({ status: 429, contentType: 'application/json', body: JSON.stringify(LOCKOUT) }));

    await page.goto('/account-preferences');
    await page.getByRole('radio', { name: /^Disable$/ }).first().click();
    await page.locator('#disable-totp-code').fill('123456');
    await page.getByRole('button', { name: /^Disable 2FA$/ }).click();
    await expect(page.getByText(TOTP_TOO_MANY_ATTEMPTS_MESSAGE)).toBeVisible();
  });

  test('password reset: lockout', async ({ page }) => {
    await page.route('**/api/v1/verify/reset-password', route => {
      if (!route.request().headers()['x-mfa-code']) {
        return route.fulfill({ status: 401, contentType: 'application/json', body: JSON.stringify({ error: 'TOTP is required' }) });
      }
      return route.fulfill({ status: 429, contentType: 'application/json', body: JSON.stringify(LOCKOUT) });
    });

    await page.goto('/reset-password/reset-token');
    await page.getByPlaceholder('New password').fill('N3w-Passw0rd-Long!');
    await page.getByPlaceholder('Confirm password').fill('N3w-Passw0rd-Long!');
    await page.getByRole('button', { name: /^Reset password$/ }).click();
    await page.getByPlaceholder('2FA code').fill('123456');
    await page.getByRole('button', { name: /^Reset password$/ }).click();
    await expect(page.getByText(TOTP_TOO_MANY_ATTEMPTS_MESSAGE)).toBeVisible();
  });
});
