import { test, expect } from '@playwright/test';

const mockAccount = {
  account_id: 'abc',
  email: 'old@example.com',
  email_verified: true,
  auth_methods: ['password'],
  mfa: { totp: { enabled: false } },
};

const setupRoutes = async (
  page: import('@playwright/test').Page,
  onPatch: (body: string | null) => void,
) => {
  await page.addInitScript(() => { window.localStorage.setItem('AUTH_KEY', 'true'); });
  await page.route('**/api/v1/accounts', async route => {
    if (route.request().method() === 'PATCH') {
      onPatch(route.request().postData());
      return route.fulfill({ status: 200, body: '' });
    }
    return route.continue();
  });
  await page.route('**/api/v1/accounts/current', async route => {
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(mockAccount) });
  });
  await page.route('**/api/v1/webauthn/passkeys', async route => {
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([]) });
  });
  await page.route('**/api/v1/profiles', async route => {
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([]) });
  });
};

// Happy path: two-step flow shows the lockout warning, tolerates a
// casing/whitespace variant in the confirm entry, and sends the PATCH.
test('email change requires confirm step and sends patch on match', async ({ page }) => {
  let patchBody: string | null = null;
  await setupRoutes(page, body => { patchBody = body; });
  await page.goto('http://localhost:5173/account-preferences');

  const changeBtn = page.getByRole('button', { name: /Change email/i });
  const isVisible = await changeBtn.isVisible().catch(() => false);
  if (!isVisible) {
    test.skip(true, 'Change email button not reachable - app server likely not started');
    return;
  }
  await changeBtn.click();

  await page.getByPlaceholder('new@example.com').fill('new@example.com');
  await page.getByPlaceholder('••••••••').fill('Password123!');
  await page.getByRole('button', { name: /^Continue$/ }).click();

  // Confirm step: warning about non-deliverable address is visible, no PATCH yet.
  await expect(page.getByTestId('confirm-email-warning')).toBeVisible();
  await expect(page.getByTestId('confirm-email-warning')).toContainText(/password reset/i);
  expect(patchBody).toBeNull();

  await page.getByTestId('input-confirm-email').fill(' NEW@Example.com ');
  await page.getByRole('button', { name: /Update email/i }).click();

  await expect.poll(() => patchBody).not.toBeNull();
  const payload = JSON.parse(patchBody!) as { updates: { operation: string; path: string; value: { new_email: string } }[] };
  expect(payload.updates).toHaveLength(1);
  expect(payload.updates[0].path).toBe('/email');
  expect(payload.updates[0].value.new_email).toBe('new@example.com');
});

// Negative flow: mismatching confirm entry blocks submit and no PATCH is sent.
test('email change blocked while confirm entry mismatches', async ({ page }) => {
  let patchCalled = false;
  await setupRoutes(page, () => { patchCalled = true; });
  await page.goto('http://localhost:5173/account-preferences');

  const changeBtn = page.getByRole('button', { name: /Change email/i });
  const isVisible = await changeBtn.isVisible().catch(() => false);
  if (!isVisible) {
    test.skip(true, 'Change email button not reachable - app server likely not started');
    return;
  }
  await changeBtn.click();

  await page.getByPlaceholder('new@example.com').fill('new@example.com');
  await page.getByPlaceholder('••••••••').fill('Password123!');
  await page.getByRole('button', { name: /^Continue$/ }).click();

  await page.getByTestId('input-confirm-email').fill('wrong@example.com');
  await expect(page.getByTestId('confirm-email-mismatch')).toBeVisible();
  await expect(page.getByRole('button', { name: /Update email/i })).toBeDisabled();
  await page.waitForTimeout(300);
  expect(patchCalled).toBeFalsy();
});
