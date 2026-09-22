import { test, expect } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';

const routes = ['/home','/settings','/account-preferences','/mobileconfig'];

for (const route of routes) {
  test.describe(`@layout Desktop layout ${route}`, { tag: '@desktop' }, () => {
    test.beforeEach(async ({ page }) => {
  await registerMocks(page, { authenticated: true, customProfiles: [{ id: 'prof_1', name: 'Default' }] });
      await page.goto(route);
    });

    test('no horizontal overflow & right edge aligned', async ({ page }) => {
      await page.waitForSelector('[data-testid="app-content"]', { timeout: 5000 });
      const result = await page.evaluate(() => {
        const el = document.querySelector('[data-testid="app-content"]');
        const doc = document.documentElement;
        if (!el) return null;
        const rect = (el as HTMLElement).getBoundingClientRect();
        const hasOverflow = doc.scrollWidth > doc.clientWidth + 1;
        return { right: rect.right, left: rect.left, viewport: doc.clientWidth, hasOverflow };
      });
      expect(result, 'Content container missing').not.toBeNull();
      if (!result) return;
      expect(result.hasOverflow, 'No horizontal overflow expected').toBeFalsy();
      expect(Math.abs(result.viewport - result.right)).toBeLessThanOrEqual(1);
      expect(result.left).toBeGreaterThanOrEqual(0);
    });
  });
}
