import { test, expect, type Page } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';

// The persisted app store may hold only the selected profile's internal id and
// device-level UI flags; profile data must never reach localStorage, and the
// account-scoped keys are cleared on logout.

const STORAGE_KEY = 'moddns-storage';
const PROFILE = { id: 'mongo_prof_1', profile_id: 'dnscred01', name: 'Default' };

const readStore = (page: Page) => page.evaluate((k) => localStorage.getItem(k) ?? '', STORAGE_KEY);

async function logoutViaNav(page: Page) {
  await page.route(/\/api\/v1\/accounts\/logout/i, (route) =>
    route.request().method() === 'POST'
      ? route.fulfill({ status: 200, contentType: 'application/json', body: '{}' })
      : route.continue());
  await page.getByTestId('btn-nav-logout').click();
  await page.getByTestId('btn-confirm-logout').click();
  await expect(page).toHaveURL(/\/login$/);
}

test.describe('@functional Browser storage', { tag: '@desktop' }, () => {
  test.beforeEach(async ({ page }) => {
    await registerMocks(page, { authenticated: true, customProfiles: [PROFILE] });
  });

  test('keeps only the selected profile id while signed in and clears it on logout', async ({ page }) => {
    await page.goto('/home');
    await expect(page).toHaveURL(/\/home$/);

    await expect.poll(() => readStore(page)).toContain(PROFILE.id);
    const signedIn = await readStore(page);
    expect(signedIn).not.toContain(PROFILE.profile_id);
    expect(signedIn).not.toContain('"activeProfile"');

    await logoutViaNav(page);

    const afterLogout = await readStore(page);
    expect(afterLogout).not.toContain(PROFILE.id);
    expect(afterLogout).not.toContain(PROFILE.profile_id);
  });

  test('logging out in one tab logs out the other open tab', async ({ page, context }) => {
    await page.goto('/home');
    await expect(page).toHaveURL(/\/home$/);

    const other = await context.newPage();
    await registerMocks(other, { authenticated: true, customProfiles: [PROFILE] });
    await other.goto('/settings');
    await expect(other).toHaveURL(/\/settings$/);

    await logoutViaNav(page);

    await expect(other).toHaveURL(/\/login$/);
    expect(await readStore(other)).not.toContain(PROFILE.id);
  });
});
