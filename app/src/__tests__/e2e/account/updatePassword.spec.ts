import { test, expect } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';

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
