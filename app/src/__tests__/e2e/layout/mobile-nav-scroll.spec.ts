import { test, expect } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';

// This test ensures the mobile navigation drawer is vertically scrollable in landscape mode
// and that the last interactive element (support button) can be brought into view.

// Some projects in config have suffixes like -dark; match loosely
function isMobileLike(name?: string) {
  return !!name && /(chromium-mobile|iphone15pro)/i.test(name);
}

test.describe('@layout mobile nav scrollability', () => {
  // eslint-disable-next-line no-empty-pattern
  test.beforeEach(async ({}, testInfo) => {
    if (!isMobileLike(testInfo.project.name)) test.skip();
  });

  test('navigation menu scrolls to bottom in landscape', async ({ page }) => {
  await registerMocks(page, { authenticated: true, customProfiles: [{ id: 'p1', profile_id: 'p1', name: 'Default', settings: { logs: { enabled: true }, custom_rules: [] } }] });
    await page.goto('/home');

    // Force landscape by resizing viewport (Pixel 5 baseline width/height swapped)
    await page.setViewportSize({ width: 900, height: 430 });

    // Open menu via bottom nav "More" button
    const moreBtn = page.getByTestId('bottom-nav').getByRole('button', { name: /more/i });
    await moreBtn.click();

  const nav = page.getByTestId('overlay-navigation');
    await expect(nav).toBeVisible();

    // Ensure nav content overflows (height smaller than scrollHeight) OR at least scroll is possible
    const canScroll = await nav.evaluate(el => {
      return el.scrollHeight > el.clientHeight;
    });

    // Scroll to bottom and ensure support button is in view
    await nav.evaluate(el => { el.scrollTop = el.scrollHeight; });
    const supportBtn = page.getByTestId('nav-support');
    await expect(supportBtn).toBeVisible();
    await supportBtn.scrollIntoViewIfNeeded();

    // Assert it is in viewport (bounding box within window)
    const box = await supportBtn.boundingBox();
    expect(box).not.toBeNull();
    if (box) {
      expect(box.y + box.height).toBeLessThanOrEqual(430); // within viewport height
    }

    // If the menu items fit entirely (edge case), canScroll might be false; that's acceptable.
    // But if items exceed view, canScroll must be true. We approximate by number of buttons.
    const buttonsCount = await nav.locator('button').count();
    if (buttonsCount > 8) { // heuristic threshold for overflow
      expect(canScroll).toBeTruthy();
    }
  });
});
