import { test, expect, type Page } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';

/*
  responsive-navigation.spec
  Verifies capability-based navDesktop vs mobile/overlay navigation behavior.
  Scenarios:
    1. Landscape tablet width (e.g. 1100) should NOT show persistent sidebar (navDesktop false) but should show mobile header & open overlay nav.
    2. True desktop width (>=1280) should show persistent sidebar and desktop header; overlay nav elements absent.
    3. Threshold transition 1279 -> 1280 triggers sidebar appearance; 1280 -> 1279 removes it.
*/

async function ensureAuthed(page: Page) {
  await registerMocks(page, { authenticated: true, customProfiles: [{ id: 'prof1', name: 'Default' }] });
}

// Utility: presence checks
// Sidebar (persistent desktop) wrapper test id added in App.tsx; overlay has its own id
const getPersistentSidebar = (page: Page) => page.getByTestId('persistent-sidebar');
const getOverlayNav = (page: Page) => page.getByTestId('overlay-navigation');
const getHeaderBar = (page: Page) => page.getByTestId('app-header-bar');

// NOTE: We rely on data-testid="app-header-bar" set in mobile header only.

// Only run on chromium-desktop project to leverage viewport resizing; skip mobile projects with fixed device descriptors
// (project name check similar to existing responsive.spec)

test.describe('@layout responsive navigation (navDesktop)', () => {
  test.beforeEach(async ({ page }) => {
    if (test.info().project.name !== 'chromium-desktop') test.skip();
    await ensureAuthed(page);
  });

  test('landscape tablet width uses mobile header + overlay nav', async ({ page }) => {
    await page.setViewportSize({ width: 1100, height: 900 }); // between 1024 and 1280
    await page.goto('/setup');
  // Persistent sidebar should NOT exist
  await expect(getPersistentSidebar(page)).toHaveCount(0);
    // Mobile header bar should be visible
    await expect(getHeaderBar(page)).toBeVisible();

    // Open overlay navigation via bottom nav "More" button
    const moreButton = page.getByTestId('bottom-nav').getByRole('button', { name: /more/i });
    await moreButton.click();

    // After opening, expect navigation in overlay mode
    await expect(getOverlayNav(page)).toBeVisible();

  // Close via explicit close button (backdrop may be partially covered on some layouts)
  await page.getByTestId('nav-close').click();
    // Nav panel is always-mounted (CSS transitions); check that it's translated off-screen
    await expect(page.getByTestId('nav-overlay-panel')).toHaveClass(/-translate-x-full/);
  });

  test('desktop width shows persistent sidebar and desktop header', async ({ page }) => {
    await page.setViewportSize({ width: 1400, height: 900 });
    await page.goto('/setup');
  // Persistent sidebar wrapper should exist
  await expect(getPersistentSidebar(page)).toHaveCount(1);
    // Mobile header should not be rendered
    await expect(getHeaderBar(page)).toHaveCount(0);
  });

  test('threshold transition 1279 -> 1280 toggles navDesktop', async ({ page }) => {
    await page.setViewportSize({ width: 1279, height: 900 });
    await page.goto('/setup');
  await expect(getPersistentSidebar(page)).toHaveCount(0);
    await expect(getHeaderBar(page)).toBeVisible();

    // Increase to 1280 (should trigger navDesktop)
    await page.setViewportSize({ width: 1280, height: 900 });
    // Wait for layout effect & hook listeners to fire
    await page.waitForTimeout(100);
  await expect(getPersistentSidebar(page)).toHaveCount(1);
    await expect(getHeaderBar(page)).toHaveCount(0);

    // Decrease back to 1279
    await page.setViewportSize({ width: 1279, height: 900 });
    await page.waitForTimeout(100);
  await expect(getPersistentSidebar(page)).toHaveCount(0);
    await expect(getHeaderBar(page)).toBeVisible();
  });
});
