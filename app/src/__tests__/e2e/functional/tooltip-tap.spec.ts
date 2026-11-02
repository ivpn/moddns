import { test, expect } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';

// #127: the shared Tooltip was hover-only, so the (i) info buttons did nothing
// on touch devices. Taps must now toggle the tooltip, and tapping elsewhere
// must dismiss it. The /setup IPv4 (i) icon stands in for every (i) tooltip.
test.describe('Info tooltip on touch', { tag: '@mobile' }, () => {
  test.beforeEach(async ({ page }) => {
    await registerMocks(page, { authenticated: true });
  });

  test('tapping the (i) icon shows the tooltip, tapping outside hides it', async ({ page }) => {
    await page.goto('/setup', { waitUntil: 'domcontentloaded' });

    const trigger = page.getByRole('button', { name: 'IPv4 usage information' });
    await trigger.scrollIntoViewIfNeeded();
    await expect(trigger).toBeVisible();

    await trigger.tap();
    const tooltip = page.getByRole('tooltip');
    await expect(tooltip).toBeVisible();

    // Tap far from the trigger to dismiss
    await page.getByRole('heading', { level: 1 }).first().tap();
    await expect(tooltip).not.toBeVisible();
  });

  test('second tap on the (i) icon hides the tooltip', async ({ page }) => {
    await page.goto('/setup', { waitUntil: 'domcontentloaded' });

    const trigger = page.getByRole('button', { name: 'IPv4 usage information' });
    await trigger.scrollIntoViewIfNeeded();
    await trigger.tap();
    await expect(page.getByRole('tooltip')).toBeVisible();
    await trigger.tap();
    await expect(page.getByRole('tooltip')).not.toBeVisible();
  });
});
