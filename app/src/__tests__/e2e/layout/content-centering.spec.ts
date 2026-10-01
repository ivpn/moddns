import { test, expect } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';

/**
 * Content centering tests to prevent layout regression where content
 * appears shifted to one side on mobile/tablet devices.
 *
 * Root cause of original bug: body element had `display: flex` and
 * `place-items: center` from Vite template, causing #root to be
 * horizontally centered when it didn't fill full viewport width.
 */

const VIEWPORTS = [
  { name: 'iPhone SE', width: 375, height: 667 },
  { name: 'iPhone 14', width: 390, height: 844 },
  { name: 'iPhone 14 Pro Max', width: 430, height: 932 },
  { name: 'iPad Portrait', width: 768, height: 1024 },
  { name: 'iPad Landscape', width: 1024, height: 768 },
  { name: 'iPad Pro Landscape', width: 1194, height: 834 },
];

const PROTECTED_ROUTES = ['/setup', '/blocklists', '/home', '/settings', '/custom-rules', '/query-logs'];

// These tests set their own viewport, so the project's device descriptor is irrelevant;
// run once per engine (Chromium desktop + WebKit iPhone) instead of on every project.
test.describe('@layout Content centering - body styles', { tag: ['@desktop', '@ios'] }, () => {
  test('body element should not have centering flex styles', async ({ page }) => {
    await registerMocks(page, { authenticated: true });
    await page.goto('/setup');

    const bodyStyles = await page.evaluate(() => {
      const body = document.body;
      const computed = getComputedStyle(body);
      return {
        display: computed.display,
        placeItems: computed.placeItems,
        justifyContent: computed.justifyContent,
        alignItems: computed.alignItems,
        justifyItems: computed.justifyItems,
      };
    });

    // Body should NOT be a flex container that centers children
    // This was the root cause of the left-shift bug
    if (bodyStyles.display === 'flex' || bodyStyles.display === 'inline-flex') {
      expect(bodyStyles.placeItems).not.toBe('center');
      expect(bodyStyles.justifyContent).not.toBe('center');
      expect(bodyStyles.justifyItems).not.toBe('center');
      // align-items: center is OK for vertical centering, but combined with
      // justify-content: center would cause horizontal shift
      if (bodyStyles.alignItems === 'center') {
        expect(bodyStyles.justifyContent).not.toBe('center');
      }
    }
  });

  test('html and body should span full viewport width', async ({ page }) => {
    await registerMocks(page, { authenticated: true });
    await page.goto('/setup');

    const dimensions = await page.evaluate(() => {
      const viewport = window.innerWidth;
      const htmlWidth = document.documentElement.offsetWidth;
      const bodyWidth = document.body.offsetWidth;
      const rootEl = document.getElementById('root');
      const rootWidth = rootEl ? rootEl.offsetWidth : 0;
      return { viewport, htmlWidth, bodyWidth, rootWidth };
    });

    // All should be equal to viewport width (within 1px tolerance for rounding)
    expect(dimensions.htmlWidth).toBeGreaterThanOrEqual(dimensions.viewport - 1);
    expect(dimensions.bodyWidth).toBeGreaterThanOrEqual(dimensions.viewport - 1);
    expect(dimensions.rootWidth).toBeGreaterThanOrEqual(dimensions.viewport - 1);
  });
});

test.describe('@layout Content centering - viewport matrix', { tag: ['@desktop', '@ios'] }, () => {
  test.beforeEach(async ({ page }) => {
    await registerMocks(page, { authenticated: true });
  });

  // One test per viewport keeps a fresh page for each; WebKit is unreliable across
  // repeated navigations in a single page.
  for (const vp of VIEWPORTS) {
    test(`app-content starts at the left edge and setup-container is centered on ${vp.name}`, async ({ page }) => {
      await page.setViewportSize({ width: vp.width, height: vp.height });
      await page.goto('/setup');

      const appContent = page.getByTestId('app-content');
      await expect(appContent).toBeVisible();
      const contentBox = (await appContent.boundingBox())!;
      // Content should start at x=0, not offset to the right, and span the viewport.
      expect(contentBox.x, `app-content x offset on ${vp.name}`).toBe(0);
      expect(contentBox.width, `app-content width on ${vp.name}`).toBeGreaterThanOrEqual(vp.width - 1);

      // Below the desktop breakpoint the setup container is centered with symmetric margins.
      if (vp.width < 1280) {
        const container = page.getByTestId('setup-container');
        await expect(container, `setup-container on ${vp.name}`).toBeVisible();
        const box = (await container.boundingBox())!;
        const leftMargin = box.x;
        const rightMargin = vp.width - (box.x + box.width);
        // 30px tolerance accounts for px-4 (16px) padding rounding differently per side.
        const marginDiff = Math.abs(leftMargin - rightMargin);
        expect(
          marginDiff,
          `Asymmetric margins on ${vp.name}: left=${leftMargin.toFixed(0)}px, right=${rightMargin.toFixed(0)}px, diff=${marginDiff.toFixed(0)}px`
        ).toBeLessThan(30);
      }
    });
  }

  test('content is visually centered across multiple pages', async ({ page }) => {
    await page.setViewportSize({ width: 768, height: 1024 }); // iPad portrait

    for (const route of PROTECTED_ROUTES) {
      await page.goto(route);

      // Find the main content container (different pages may use different containers)
      const appContent = page.getByTestId('app-content');
      const box = await appContent.boundingBox();

      // Verify content starts at left edge
      expect(box!.x, `${route}: app-content not at left edge`).toBe(0);

      // Verify no horizontal overflow
      const hasOverflow = await page.evaluate(() => {
        return document.documentElement.scrollWidth > window.innerWidth + 1;
      });
      expect(hasOverflow, `${route}: has horizontal overflow`).toBe(false);
    }
  });
});
