import { test, expect } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';

// Viewports representative of iPad portrait/landscape and a small phone
const VIEWPORTS = [
  { width: 390, height: 844, label: 'phone' },
  { width: 834, height: 1112, label: 'ipad-portrait' },
  { width: 1112, height: 834, label: 'ipad-landscape' },
];

// Long strings to stress overflow
const LONG_NAME = 'UltraSuperMegaExtendedBlocklistNameWithoutSpacesToTestOverflowHandling1234567890';
const LONG_DESC = 'This description containsSupercalifragilisticexpialidociousWordsAndALongURLLikeHttps://ExampleVeryLongDomainNameThatShouldWrapProperlyAndNotOverflowEvenOnTabletViewports to verify wrapping and clamping work.';

async function registerLongBlocklists(page: import('@playwright/test').Page) {
  await registerMocks(page, { authenticated: true });
  await page.route(/\/api\/v1\/blocklists(\/?|\?.*)$/i, async (route) => {
    const body = JSON.stringify([
      {
        blocklist_id: 'long-one',
        name: LONG_NAME,
        description: LONG_DESC,
        homepage: 'https://example.com/this/is/a/very/long/path/that/should/wrap/properly/in/the/tooltip',
        entries: '123456',
        last_modified: new Date().toISOString(),
      },
      // Add a few normal lists to create a realistic grid
      ...Array.from({ length: 5 }).map((_, i) => ({
        blocklist_id: `normal-${i}`,
        name: `Normal ${i}`,
        description: 'Short description',
        homepage: 'https://example.com',
        entries: '100',
        last_modified: new Date().toISOString(),
      }))
    ]);
    await route.fulfill({ status: 200, contentType: 'application/json', body });
  });
}

for (const vp of VIEWPORTS) {
  test.describe(`@layout blocklists overflow (${vp.label})`, () => {
    test.beforeEach(async ({ page }) => {
      await registerLongBlocklists(page);
      await page.setViewportSize({ width: vp.width, height: vp.height });
    });

    test('long text does not cause horizontal overflow and Updated is on its own row', async ({ page }) => {
      await page.goto('/blocklists');
      const firstCard = page.getByTestId('blocklist-card').first();
      await expect(firstCard).toBeVisible();
      await page.waitForTimeout(150);

      const hasOverflowX = await page.evaluate(() => {
        const de = document.documentElement;
        const body = document.body;
        const maxScrollWidth = Math.max(de.scrollWidth, body.scrollWidth);
        const clientWidth = de.clientWidth;
        return maxScrollWidth > clientWidth + 1;
      });
      expect(hasOverflowX).toBeFalsy();

      // (Optional) visual regression currently disabled to reduce flakiness; enable when stable
      // if (['phone','ipad-portrait'].includes(vp.label)) {
      //   await expect(firstCard).toHaveScreenshot(`blocklist-card-${vp.label}.png`, { maxDiffPixels: 3000 });
      // }

      const isBelowXl = vp.width < 1280;
      if (isBelowXl) {
        const entriesLocator = firstCard.locator('span', { hasText: /entries$/ }).first();
        // Mobile/tablet Updated is inside the second row which is `.xl:hidden` parent
        const updatedLocator = firstCard.locator('.xl\\:hidden span', { hasText: /^Updated / }).first();
        await expect(entriesLocator).toBeVisible();
        await expect(updatedLocator).toBeVisible();
        const [entriesBox, updatedBox] = await Promise.all([
          entriesLocator.boundingBox(),
          updatedLocator.boundingBox(),
        ]);
        expect(entriesBox).not.toBeNull();
        expect(updatedBox).not.toBeNull();
        if (entriesBox && updatedBox) {
          expect(updatedBox.y).toBeGreaterThan(entriesBox.y + entriesBox.height - 4);
        }
      } else {
        const updatedInline = firstCard.locator('.xl\\:inline', { hasText: /^Updated / }).first();
        await expect(updatedInline).toBeVisible();
      }

      const updatedTextAny = await firstCard.locator('text=/^Updated /').first().innerText();
      expect(updatedTextAny).toMatch(/^Updated (?:~)?/);
    });
  });
}

// Document-level scroll is masked by `overflow-x-hidden` on the app content wrapper,
// so a horizontally-overflowing inner container shows up as clipped content rather than
// a page scrollbar. This guards against that by asserting that no element extends past
// the viewport's right edge, across every blocklists tab.
test.describe('@layout blocklists no inner horizontal overflow', () => {
  test.beforeEach(async ({ page }) => {
    await registerMocks(page, { authenticated: true });
    await page.setViewportSize({ width: 390, height: 844 });
  });

  async function maxRightOverflow(page: import('@playwright/test').Page): Promise<number> {
    return page.evaluate(() => {
      const vw = document.documentElement.clientWidth;
      let worst = 0;
      document.querySelectorAll('main *').forEach((el) => {
        const right = el.getBoundingClientRect().right;
        if (right - vw > worst) worst = right - vw;
      });
      return Math.round(worst);
    });
  }

  for (const tab of ['Lists', 'Categories', 'Services']) {
    test(`${tab} tab content stays within the viewport`, async ({ page }) => {
      await page.goto('/blocklists');
      await page.getByRole('tab', { name: tab }).click();
      await page.waitForTimeout(400);
      const overflow = await maxRightOverflow(page);
      expect(overflow).toBeLessThanOrEqual(1);
    });
  }
});
