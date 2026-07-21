import { test, expect } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';

// #121: on mobile the header wrapper is position:fixed with constant top/left/right,
// so nothing should be animated. A `transition-all` here makes Android Chrome
// animate URL-bar-collapse reflows over 500ms while the content's padding-top
// (a px CSS variable) does not follow, leaving a transient gap below the header.
// Playwright cannot simulate the URL-bar collapse itself, so this is a regression
// guard on the mechanism: the wrapper must not transition `all` or any geometry
// property on mobile viewports.
test.describe('@layout mobile header wrapper transition', () => {
  test.beforeEach(async ({ page, isMobile }) => {
    test.skip(!isMobile, 'mobile-only regression guard');
    await registerMocks(page, { authenticated: true });
  });

  test('fixed header wrapper does not animate geometry on mobile', async ({ page }) => {
    await page.goto('/home');
    const wrapper = page.getByTestId('app-header-wrapper');
    await expect(wrapper).toBeVisible();

    const transition = await wrapper.evaluate((el) => {
      const cs = getComputedStyle(el);
      return { property: cs.transitionProperty, duration: cs.transitionDuration };
    });

    const props = transition.property.split(',').map(p => p.trim());
    const durations = transition.duration.split(',').map(d => parseFloat(d));
    const animated = props.filter((p, i) => (durations[i] ?? durations[0] ?? 0) > 0);

    for (const forbidden of ['all', 'top', 'left', 'right', 'bottom', 'width', 'height', 'transform']) {
      expect(animated, `mobile header must not animate "${forbidden}"`).not.toContain(forbidden);
    }
  });
});
