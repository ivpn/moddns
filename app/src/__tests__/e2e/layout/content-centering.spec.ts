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

test.describe('@layout Content centering - body styles', () => {
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

test.describe('@layout Content centering - app content area', () => {
  test.beforeEach(async ({ page }) => {
    await registerMocks(page, { authenticated: true });
  });

  test('app-content fills full viewport width on mobile', async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto('/setup');

    const appContent = page.getByTestId('app-content');
    await expect(appContent).toBeVisible();

    const box = await appContent.boundingBox();
    const viewport = page.viewportSize()!;

    // app-content should start at x=0 (no left offset)
    expect(box!.x).toBe(0);
    // app-content should span full viewport width
    expect(box!.width).toBeGreaterThanOrEqual(viewport.width - 1);
  });

  for (const vp of VIEWPORTS) {
    test(`content area starts at left edge on ${vp.name}`, async ({ page }) => {
      await page.setViewportSize({ width: vp.width, height: vp.height });
      await page.goto('/setup');

      const appContent = page.getByTestId('app-content');
      const box = await appContent.boundingBox();

      // Content should start at x=0, not offset to the right
      expect(box!.x, `app-content x offset on ${vp.name}`).toBe(0);
    });
  }
});

test.describe('@layout Content centering - symmetric margins', () => {
  test.beforeEach(async ({ page }) => {
    await registerMocks(page, { authenticated: true });
  });

  for (const vp of VIEWPORTS.filter(v => v.width < 1280)) {
    test(`setup-container has symmetric margins on ${vp.name}`, async ({ page }) => {
      await page.setViewportSize({ width: vp.width, height: vp.height });
      await page.goto('/setup');

      const container = page.getByTestId('setup-container');
      await expect(container).toBeVisible();

      const box = await container.boundingBox();
      const viewport = page.viewportSize()!;

      const leftMargin = box!.x;
      const rightMargin = viewport.width - (box!.x + box!.width);

      // Left and right margins should be roughly equal (within 30px tolerance)
      // This accounts for px-4 (16px) padding which may round differently
      const marginDiff = Math.abs(leftMargin - rightMargin);
      expect(
        marginDiff,
        `Asymmetric margins on ${vp.name}: left=${leftMargin.toFixed(0)}px, right=${rightMargin.toFixed(0)}px, diff=${marginDiff.toFixed(0)}px`
      ).toBeLessThan(30);
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
