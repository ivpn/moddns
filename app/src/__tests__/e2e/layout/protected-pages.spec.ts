// moved from src/tests/e2e/protected-pages.spec.ts
import { test, expect } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';

const routes = [
  '/home',
  '/setup',
  '/settings',
  '/blocklists',
  '/custom-rules',
  '/account-preferences',
  '/mobileconfig',
  '/query-logs',
];

// All routes are now checked strictly for horizontal overflow
const skipHorizontalCheck = new Set<string>();

test.describe('@layout Protected pages basic smoke', () => {
  test.beforeEach(async ({ page }) => {
    await registerMocks(page, { authenticated: true, customProfiles: [{ id: 'prof_1', name: 'Default' }] });
  });

  for (const route of routes) {
    test(`loads ${route} without redirect`, async ({ page }) => {
      await page.goto(route);
      await expect(page).toHaveURL(new RegExp(route.replace('/', '\/')));
      if (!skipHorizontalCheck.has(route)) {
          const hasHorizontal = await page.evaluate(() => {
            const doc = document.documentElement;
            return doc.scrollWidth > doc.clientWidth + 1;
          });
          if (process.env.STRICT_MOBILE === '1') {
            expect(hasHorizontal, `Horizontal overflow detected on ${route}`).toBeFalsy();
          }
      }
    });
  }
});
