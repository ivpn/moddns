import { test, expect } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';

// Emulates a tablet/iPad-like viewport (portrait & landscape variants)
const VIEWPORTS = [
  { width: 834, height: 1112, label: 'iPad-portrait' }, // iPad Air portrait
  { width: 1112, height: 834, label: 'iPad-landscape' }, // iPad Air landscape
];

// Utility: ensure enough mock blocklists to require vertical scrolling
function extendBlocklists(blocklists: Record<string, unknown>[], minCount = 30) {
  const clone = [...blocklists];
  let i = 0;
  while (clone.length < minCount) {
    const base = blocklists[i % blocklists.length];
    clone.push({
      ...base,
      blocklist_id: `${base.blocklist_id}-extra-${clone.length}`,
      name: `${base.name} Extra ${clone.length}`,
    });
    i++;
  }
  return clone;
}

// Ensure extended blocklists route takes precedence over default registerMocks route.
// Playwright matches routes in registration order; empirically later route was winning,
// so registerMocks first, then add the overflow route.
async function registerBlocklistsWithOverflow(page: import('@playwright/test').Page) {
  await registerMocks(page, { authenticated: true });
  await page.route(/\/api\/v1\/blocklists(\/?|\?.*)$/i, async (r) => {
    const mod = await import('../../mocks/apiMocks');
    const base = mod.createMockBlocklists();
    const extended = extendBlocklists(base);
    await r.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(extended) });
  });
}

for (const vp of VIEWPORTS) {
  test.describe(`@layout blocklists scroll reach (${vp.label})`, () => {
    test.beforeEach(async ({ page }) => {
      await registerBlocklistsWithOverflow(page);
      await page.setViewportSize({ width: vp.width, height: vp.height });
    });

    test('last blocklist card can be scrolled into view', async ({ page }) => {
  await page.goto('/blocklists');

      // Wait for blocklists request to resolve and cards render (poll up to 8s)
      const start = Date.now();
      const cards = page.getByTestId('blocklist-card');
      while (Date.now() - start < 8000) {
        if (await cards.count() > 0) break;
        await page.waitForTimeout(150);
      }
      expect(await cards.count()).toBeGreaterThan(0);

      // Count cards (should exceed visible area)
      const count = await cards.count();
      expect(count).toBeGreaterThan(20); // sanity threshold

      // Scroll container: page scroll or ScrollArea viewport
      // Try locating ScrollArea viewport first
      const viewport = page.locator('[data-slot="scroll-area-viewport"]').first();
      const hasViewport = await viewport.count();

      if (hasViewport) {
        // Scroll the internal viewport
        await viewport.evaluate(el => { el.scrollTop = el.scrollHeight; });
      } else {
        // Fallback to window scroll
        await page.evaluate(() => { window.scrollTo(0, document.body.scrollHeight); });
      }

      const last = cards.nth(count - 1);
      await last.scrollIntoViewIfNeeded();
      await expect(last).toBeVisible();

      // Assert its bottom is within (or very close to) viewport height (allow small overflow tolerance)
      const box = await last.boundingBox();
      expect(box).not.toBeNull();
      if (box) {
        expect(box.y + box.height).toBeLessThanOrEqual(vp.height + 2);
      }
    });
  });
}
