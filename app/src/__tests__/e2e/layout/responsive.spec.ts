import { test, expect } from '@playwright/test';
import type { Page } from '@playwright/test';

const BREAKPOINTS = [360, 390, 414, 768, 1024];

async function hasHorizontalOverflow(page: Page) {
  return await page.evaluate(() => {
    const docWidth = document.documentElement.scrollWidth;
    const winWidth = window.innerWidth;
    return docWidth > winWidth + 1;
  });
}

async function getMainContainerWidths(page: Page) {
  return await page.evaluate(() => {
    const candidates = Array.from(document.querySelectorAll('main, [role="main"], .auth-shell, .dialog-shell'));
    return candidates.slice(0, 5).map(el => ({ tag: el.tagName, class: (el as HTMLElement).className, width: (el as HTMLElement).offsetWidth }));
  });
}

test.describe('@layout Responsive layout matrix', { tag: '@desktop' }, () => {
  test('home adapts across breakpoints (mobile + desktop)', async ({ page }) => {
    for (const width of BREAKPOINTS) {
      await page.setViewportSize({ width, height: 900 });
      await page.goto('/home');
      const overflow = await hasHorizontalOverflow(page);
      expect(overflow, `Horizontal overflow at width ${width}`).toBeFalsy();
      const containers = await getMainContainerWidths(page);
      for (const c of containers) {
        expect(c.width, `Container wider than viewport at ${width}: ${c.tag}.${c.class} width=${c.width}`).toBeLessThanOrEqual(width);
      }
    }
  });
});
