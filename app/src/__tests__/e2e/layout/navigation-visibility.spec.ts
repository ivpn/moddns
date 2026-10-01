import { test, expect } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';
const NAV_SELECTOR = '[data-testid="main-navigation"]';

test.describe('@layout Navigation visibility responsive behaviour', () => {
  test('is hidden on mobile viewport', { tag: '@mobile' }, async ({ page }) => {
  await registerMocks(page, { authenticated: true, customProfiles: [{ id: 'prof1', name: 'Default', uuid: 'prof1' }] });
    await page.goto('/setup');
    await expect(page.locator(NAV_SELECTOR)).toHaveCount(0);
  });

  test('is visible on desktop viewport', { tag: '@desktop' }, async ({ page }) => {
  await registerMocks(page, { authenticated: true, customProfiles: [{ id: 'prof1', name: 'Default', uuid: 'prof1' }] });
    await page.goto('/setup');
    const nav = page.locator(NAV_SELECTOR);
    await expect(nav).toHaveCount(1);
    await expect(nav).toBeVisible();
  });
});
