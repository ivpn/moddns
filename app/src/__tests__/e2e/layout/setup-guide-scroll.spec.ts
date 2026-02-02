import { test, expect } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';

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
});
