import { test, expect } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';

// #127: the shared Tooltip was hover-only, so the (i) info buttons did nothing
// on touch devices. Taps must now toggle the tooltip, and tapping elsewhere
// must dismiss it.
test.describe('Settings retention tooltip on touch', () => {
  test.beforeEach(async ({ page, isMobile }) => {
    test.skip(!isMobile, 'touch interaction is mobile-only');
    await registerMocks(page, { authenticated: true });
  });

  test('tapping the (i) icon shows the tooltip, tapping outside hides it', async ({ page }) => {
    await page.goto('/settings', { waitUntil: 'domcontentloaded' });

    const trigger = page.getByTestId('retention-info-trigger');
    await trigger.scrollIntoViewIfNeeded();
    await expect(trigger).toBeVisible();

    await trigger.tap();
    const tooltip = page.getByRole('tooltip');
    await expect(tooltip).toBeVisible();
    await expect(tooltip).toContainText(/retention/i);

    // Tap far from the trigger to dismiss
    await page.getByTestId('mobile-header-page-title').tap();
    await expect(tooltip).not.toBeVisible();
  });

  test('second tap on the (i) icon hides the tooltip', async ({ page }) => {
    await page.goto('/settings', { waitUntil: 'domcontentloaded' });

    const trigger = page.getByTestId('retention-info-trigger');
    await trigger.scrollIntoViewIfNeeded();
    await trigger.tap();
    await expect(page.getByRole('tooltip')).toBeVisible();
    await trigger.tap();
    await expect(page.getByRole('tooltip')).not.toBeVisible();
  });
});
