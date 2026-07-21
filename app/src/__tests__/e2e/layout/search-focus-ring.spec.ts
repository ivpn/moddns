import { test, expect } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';

// #120: desktop search inputs on the Blocklists and Logs pages sit inside an
// overflow-x-auto scroll container. The shared Input paints its focus ring as a
// 3px box-shadow outside its border box, so the input needs >=3px of room inside
// the scroller's clip box (its padding box) or the ring gets clipped. Geometric
// proxy assertion: the input must be inset >=3px from the clipping ancestor.
//
// The Logs filters row is only "desktop" at lg (1024px); Blocklists at md (768px).
const CASES = [
  { path: '/blocklists', label: 'Search blocklists', viewports: [
    { width: 800, height: 600, tag: 'md' },
    { width: 1280, height: 800, tag: 'lg' },
  ]},
  { path: '/query-logs', label: 'Search domain or its part', viewports: [
    { width: 1280, height: 800, tag: 'lg' },
  ]},
];

for (const c of CASES) {
  for (const vp of c.viewports) {
    test.describe(`@layout search focus ring ${c.path} (${vp.tag})`, () => {
      test.beforeEach(async ({ page }) => {
        await registerMocks(page, {
          authenticated: true,
          customProfiles: [{ id: 'prof1', profile_id: 'prof1', name: 'Default', settings: { logs: { enabled: true } } }]
        });
        // Register AFTER registerMocks so it wins over the /profiles catch-all
        await page.route(/\/api\/v1\/profiles\/prof1\/logs/i, route => {
          route.fulfill({ status: 200, contentType: 'application/json', body: '[]' });
        });
        await page.setViewportSize({ width: vp.width, height: vp.height });
      });

      test('search input has room for its focus ring inside the scroll container', async ({ page }) => {
        await page.goto(c.path);
        const search = page.locator(`input[aria-label="${c.label}"]:visible`);
        await expect(search).toBeVisible();
        await search.focus();

        const insets = await search.evaluate((el) => {
          let node = el.parentElement;
          while (node) {
            const cs = getComputedStyle(node);
            if (['auto', 'scroll', 'hidden', 'clip'].includes(cs.overflowX)) {
              const r = el.getBoundingClientRect();
              const c2 = node.getBoundingClientRect();
              return {
                left: r.left - (c2.left + parseFloat(cs.borderLeftWidth)),
                top: r.top - (c2.top + parseFloat(cs.borderTopWidth)),
                bottom: (c2.bottom - parseFloat(cs.borderBottomWidth)) - r.bottom,
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
}
