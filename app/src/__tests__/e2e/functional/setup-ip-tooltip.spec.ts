import { test, expect } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';

// #127 follow-up: on /setup the IPv4/IPv6 rows are tap-to-copy on mobile and
// the (i) info icon sits inside that tap zone. Tapping the (i) must show the
// tooltip and must NOT trigger the copy action; tapping the address value must
// still copy. Copy always raises a toast (success or failure), so "no toast"
// proves copy was not triggered.
test.describe('Setup IP info tooltip on touch', () => {
  test.beforeEach(async ({ page, isMobile }) => {
    test.skip(!isMobile, 'touch interaction is mobile-only');
    await registerMocks(page, { authenticated: true });
  });

  test('tapping the IPv4 (i) shows the tooltip without copying', async ({ page }) => {
    await page.goto('/setup');

    const trigger = page.getByRole('button', { name: 'IPv4 usage information' });
    await trigger.scrollIntoViewIfNeeded();
    await expect(trigger).toBeVisible();

    await trigger.tap();
    await expect(page.getByRole('tooltip')).toBeVisible();
    await expect(page.getByRole('tooltip')).toContainText(/Plain DNS is not supported/i);

    // No copy toast of any kind — the tap must not reach the copy handler
    await expect(page.locator('[data-sonner-toast]')).toHaveCount(0);
  });

  test('IPv4 and IPv6 copy tap zones are identical and fill the row', async ({ page }) => {
    await page.goto('/setup');

    const v4 = page.getByRole('button', { name: 'Copy IPv4' }).first();
    const v6 = page.getByRole('button', { name: 'Copy IPv6' }).first();
    await v4.scrollIntoViewIfNeeded();
    await expect(v4).toBeVisible();
    await expect(v6).toBeVisible();

    const v4Box = await v4.boundingBox();
    const v6Box = await v6.boundingBox();
    expect(v4Box).not.toBeNull();
    expect(v6Box).not.toBeNull();

    // Same tap area regardless of the address length…
    expect(Math.abs(v4Box!.width - v6Box!.width)).toBeLessThanOrEqual(2);
    expect(Math.abs(v4Box!.x - v6Box!.x)).toBeLessThanOrEqual(2);

    // …and it fills the row up to the label/info area (not sized to content)
    const rowBox = await v4.evaluate((el) => {
      const r = el.parentElement!.getBoundingClientRect();
      return { x: r.x, width: r.width };
    });
    expect(v4Box!.width).toBeGreaterThanOrEqual(rowBox.width * 0.6);
  });

  test('tapping the IPv4 value still copies (toast appears)', async ({ page }) => {
    await page.goto('/setup');

    const copyTarget = page.getByRole('button', { name: 'Copy IPv4' }).first();
    await copyTarget.scrollIntoViewIfNeeded();
    await copyTarget.tap();

    // Success or failure toast both prove the copy handler ran
    await expect(page.locator('[data-sonner-toast]').first()).toBeVisible();
  });
});
