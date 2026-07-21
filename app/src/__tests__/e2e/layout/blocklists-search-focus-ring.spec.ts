import { test, expect } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';

// #120: the desktop blocklists search sits inside an overflow-x-auto scroll
// container. The shared Input paints its focus ring as a 3px box-shadow outside
// its border box, so the input needs >=3px of room inside the scroller's clip
// box (its padding box) or the ring gets clipped. Geometric proxy assertion:
// the input must be inset >=3px from the clipping ancestor on top/left/bottom.
const VIEWPORTS = [
  { width: 800, height: 600, label: 'md' },
  { width: 1280, height: 800, label: 'lg' },
];

for (const vp of VIEWPORTS) {
  test.describe(`@layout blocklists search focus ring (${vp.label})`, () => {
    test.beforeEach(async ({ page }) => {
      await registerMocks(page, { authenticated: true });
      await page.setViewportSize({ width: vp.width, height: vp.height });
    });

    test('search input has room for its focus ring inside the scroll container', async ({ page }) => {
      await page.goto('/blocklists');
      const search = page.locator('input[aria-label="Search blocklists"]:visible');
      await expect(search).toBeVisible();
      await search.focus();

      const insets = await search.evaluate((el) => {
        let node = el.parentElement;
        while (node) {
          const cs = getComputedStyle(node);
          if (['auto', 'scroll', 'hidden', 'clip'].includes(cs.overflowX)) {
            const r = el.getBoundingClientRect();
            const c = node.getBoundingClientRect();
            return {
              left: r.left - (c.left + parseFloat(cs.borderLeftWidth)),
              top: r.top - (c.top + parseFloat(cs.borderTopWidth)),
              bottom: (c.bottom - parseFloat(cs.borderBottomWidth)) - r.bottom,
            };
          }
          node = node.parentElement;
        }
        return null;
      });

      expect(insets, 'search input should be inside an overflow container').not.toBeNull();
      expect(insets!.left).toBeGreaterThanOrEqual(3);
      expect(insets!.top).toBeGreaterThanOrEqual(3);
      expect(insets!.bottom).toBeGreaterThanOrEqual(3);
    });
  });
}
