import { test, expect } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';
import { AUTH_KEY } from '@/lib/consts';

// Ensures app header remains visible and clickable when setup overlay is open in mobile landscape.

test.describe('@layout setup overlay header visibility', () => {
  // eslint-disable-next-line no-empty-pattern
  test.beforeEach(async ({}, testInfo) => {
    if (!/(chromium-mobile|iphone15pro)/i.test(testInfo.project.name)) test.skip();
  });

  test('header visible above overlay and overlay scrolls', async ({ page }) => {
    if (/iphone15pro/i.test(test.info().project.name)) test.skip();
    // Strategy: land on public route first so no protected loader / redirect runs before we seed auth + mocks.
    await page.goto('/login');

    // Seed auth + minimal profile data directly on the page (synchronous, ensures initial state) before hitting protected route.
    await page.evaluate((key) => {
      localStorage.setItem(key, 'true');
  const profiles = [{ id: 'prof1', profile_id: 'prof1', name: 'Default', settings: { custom_rules: [] } }];
      localStorage.setItem('profiles', JSON.stringify(profiles));
      localStorage.setItem('activeProfileId', 'prof1');
    }, AUTH_KEY);

    // Register network mocks (adds route handlers BEFORE protected loader fires)
  await registerMocks(page, { authenticated: true, customProfiles: [{ id: 'prof1', profile_id: 'prof1', name: 'Default', settings: { custom_rules: [] } }], ensureActiveProfile: true });

    // Now navigate to setup (protected). Loader should see auth + mocks.
    await page.goto('/setup');
    await page.waitForURL(/\/setup$/, { timeout: 10000 });

    // Force a landscape-like viewport only for chromium mobile (iphone preset keeps default to avoid mismatch)
    if (/chromium-mobile/i.test(test.info().project.name)) {
      try { await page.setViewportSize({ width: 700, height: 430 }); } catch { /* ignore if not supported */ }
    }

    // Open Windows guide
    const windowsCard = page.getByTestId('setup-platform-card-windows');
    await expect(windowsCard).toBeVisible();
    await windowsCard.click();

    const panel = page.getByTestId('setup-guide-panel');
    await panel.waitFor({ state: 'visible', timeout: 15000 });

    // Header bar should still be visible above the overlay
    const headerBar = page.getByTestId('app-header-bar');
    await expect(headerBar).toBeVisible();

    // Open nav via bottom nav "More" button; if overlay blocked pointer events test would fail
    const moreBtn = page.getByTestId('bottom-nav').getByRole('button', { name: /more/i });
    await moreBtn.click();
    // Expect mobile nav overlay to appear (navigation menu role or close button X inside nav)
  const nav = page.getByTestId('overlay-navigation');
  await expect(nav).toBeVisible();
  // Close control should be present in overlay header
  await expect(page.getByTestId('nav-close')).toBeVisible();

    // Close nav by clicking button again if close present
    const closeLogout = page.getByTestId('btn-nav-logout');
    await expect(closeLogout).toBeVisible();

    // Scroll overlay content
    const content = page.getByTestId('setup-guide-content');
    const canScroll = await content.evaluate(el => el.scrollHeight > el.clientHeight);
    if (canScroll) {
      await content.evaluate(el => { el.scrollTop = el.scrollHeight; });
      const atBottom = await content.evaluate(el => {
        // allow a small epsilon to account for fractional pixel differences on iOS
        return el.scrollTop + el.clientHeight >= el.scrollHeight - Math.max(8, el.clientHeight * 0.02);
      });
      expect(atBottom).toBeTruthy();
    }
  });
});
