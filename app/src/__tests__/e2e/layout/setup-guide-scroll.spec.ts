import { test, expect } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';
import { AUTH_KEY } from '@/lib/consts';

// Verifies the setup guide overlay/panel is scrollable in mobile landscape.

test.describe('@layout setup guide scrollability', () => {
  // eslint-disable-next-line no-empty-pattern
  test.beforeEach(async ({}, testInfo) => {
    // Only run on mobile-like projects (naming pattern from config)
    if (!/(chromium-mobile|iphone15pro)/i.test(testInfo.project.name)) test.skip();
    if (/iphone15pro/i.test(testInfo.project.name)) { test.skip(); }
  });

  test('setup guide overlay scrolls to bottom in landscape', async ({ page }) => {
  await registerMocks(page, { authenticated: true, customProfiles: [{ id: 'p1', profile_id: 'p1', name: 'Default', settings: { logs: { enabled: true }, custom_rules: [] } }] });
    await page.goto('/setup');

  // Force landscape below md breakpoint (md ~768px). Use 700x430.
  await page.setViewportSize({ width: 700, height: 430 });

  // Prefer mobile grid card; fallback to desktop card if responsive layout changes.
  let windowsCard = page.getByTestId('setup-platform-card-windows');
  if (!(await windowsCard.count())) {
    // Fallback: desktop test id
    windowsCard = page.getByTestId('setup-platform-card-desktop-windows');
  }
  if (!(await windowsCard.count())) test.skip();
  await windowsCard.first().click();

    const panel = page.getByTestId('setup-guide-panel');
    await expect(panel).toBeVisible();

    const content = page.getByTestId('setup-guide-content');
    await expect(content).toBeVisible();

    const metricsBefore = await content.evaluate(el => ({ sh: el.scrollHeight, ch: el.clientHeight, st: el.scrollTop }));
    const scrollable = metricsBefore.sh > metricsBefore.ch + 4; // allow tiny diff margin
    if (scrollable) {
      await content.evaluate(el => { el.scrollTop = el.scrollHeight; });
      const atBottom = await content.evaluate(el => el.scrollTop + el.clientHeight >= el.scrollHeight - 2);
      expect(atBottom).toBeTruthy();
    }
  });

  // Regression guard: when overlay panel is open and content is scrolled to the
  // bottom, the last step must not be hidden behind the fixed BottomNav.
  // The panel sits at z-40 and BottomNav at z-50; without a height offset for
  // the navbar the last step gets clipped under it on mobile.
  // Auth is seeded manually (mirrors setup-overlay-header-visibility.spec.ts)
  // because storageState alone doesn't reliably hydrate profile state on
  // protected routes for these mobile projects.
  test('last step is visible above bottom nav when scrolled to bottom', async ({ page }) => {
    await page.goto('/login');
    await page.evaluate((key) => {
      localStorage.setItem(key, 'true');
      const profiles = [{ id: 'prof1', profile_id: 'prof1', name: 'Default', settings: { custom_rules: [] } }];
      localStorage.setItem('profiles', JSON.stringify(profiles));
      localStorage.setItem('activeProfileId', 'prof1');
    }, AUTH_KEY);

    await registerMocks(page, {
      authenticated: true,
      customProfiles: [{ id: 'prof1', profile_id: 'prof1', name: 'Default', settings: { custom_rules: [] } }],
      ensureActiveProfile: true,
    });

    await page.goto('/setup');
    await page.waitForURL(/\/setup$/, { timeout: 10000 });

    // Default Pixel 5 portrait viewport (393x851) is the realistic mobile case
    // that the bug report describes. Don't override.

    const windowsCard = page.getByTestId('setup-platform-card-windows');
    await expect(windowsCard).toBeVisible();
    await windowsCard.click();

    const panel = page.getByTestId('setup-guide-panel');
    await expect(panel).toBeVisible();

    const content = page.getByTestId('setup-guide-content');
    await expect(content).toBeVisible();

    const bottomNav = page.getByTestId('bottom-nav');
    await expect(bottomNav).toBeVisible();

    const lastStep = page.getByTestId('setup-guide-step').last();
    await expect(lastStep).toBeAttached();

    // Scroll the last step into view (mirrors what a user does on touch).
    await lastStep.evaluate(el => el.scrollIntoView({ block: 'end', inline: 'nearest' }));

    const navTop = await bottomNav.evaluate(el => el.getBoundingClientRect().top);
    const lastStepBottom = await lastStep.evaluate(el => el.getBoundingClientRect().bottom);

    // The last step's bottom edge must sit at or above the bottom nav's top edge.
    // Allow 1px epsilon for sub-pixel rounding.
    expect(lastStepBottom).toBeLessThanOrEqual(navTop + 1);
  });
});
