import { test, expect } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';

test.describe('Mobile Header Page Title', { tag: '@mobile' }, () => {
  test.use({ viewport: { width: 430, height: 900 } });

  test.beforeEach(async ({ page }) => {
    await registerMocks(page, {
      authenticated: true,
      customProfiles: [{ id: 'prof1', profile_id: 'prof1', name: 'Default' }]
    });
  });

  test('shows page title inside mobile header area on Blocklists', async ({ page }) => {
    await page.goto('/blocklists', { waitUntil: 'domcontentloaded' });

    // Validate final URL (allow possible trailing slash variations)
    await expect.poll(async () => /\/blocklists$/.test(page.url())).toBeTruthy();

    const titleLocator = page.getByTestId('mobile-header-page-title');
    await titleLocator.waitFor({ state: 'visible', timeout: 5000 });
    await expect(titleLocator).toHaveText(/Blocklists/i);
  });

  test('shows page title inside mobile header area on Settings', async ({ page }) => {
    await page.goto('/settings', { waitUntil: 'domcontentloaded' });

    await expect.poll(async () => /\/settings$/.test(page.url())).toBeTruthy();

    const titleLocator = page.getByTestId('mobile-header-page-title');
    await titleLocator.waitFor({ state: 'visible', timeout: 5000 });
    await expect(titleLocator).toHaveText(/Settings/i);
  });

  test('shows page title inside mobile header area on Custom Rules', async ({ page }) => {
    await page.goto('/custom-rules', { waitUntil: 'domcontentloaded' });

    await expect.poll(async () => /\/custom-rules$/.test(page.url())).toBeTruthy();

    const titleLocator = page.getByTestId('mobile-header-page-title');
    await titleLocator.waitFor({ state: 'visible', timeout: 5000 });
    await expect(titleLocator).toHaveText(/Custom rules/i);
  });

  test('shows page title inside mobile header area on Query Logs', async ({ page }) => {
    await page.goto('/query-logs', { waitUntil: 'domcontentloaded' });

    await expect.poll(async () => /\/query-logs$/.test(page.url())).toBeTruthy();

    const titleLocator = page.getByTestId('mobile-header-page-title');
    await titleLocator.waitFor({ state: 'visible', timeout: 5000 });
    await expect(titleLocator).toHaveText(/Logs/i);
  });
});
