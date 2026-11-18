import { test, expect, type Page } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';
import { formatBackupCode } from '../../../lib/backupCodes';

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
    // Phone keyboards must not autocorrect or capitalise a typed code.
    const otpInput = page.getByTestId('input-otp');
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
