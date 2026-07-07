import { test, expect } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';

(test.describe as typeof test.describe)('@layout @ios Custom Rules iOS visibility', () => {
  test('content renders on iOS', async ({ page }) => {
    test.skip(!/iphone15pro/i.test(test.info().project.name), 'Only relevant for iPhone viewport projects');

  await registerMocks(page, { authenticated: true, customProfiles: [{ id: 'prof1', profile_id: 'prof1', name: 'Default', settings: { custom_rules: [] } }] });

    await page.goto('/custom-rules');

    await expect(page.getByText('Manually add domains', { exact: false })).toBeVisible();
    await expect(page.getByRole('tab', { name: /denylist/i })).toBeVisible();
  });

  // Regression: a long group name must truncate, not push the group header off-screen
  // (QA issue #634 — "Group names are cut off on mobile viewports when they are too long").
  test('long group name does not overflow on mobile', async ({ page }) => {
    test.skip(!/iphone15pro/i.test(test.info().project.name), 'Only relevant for iPhone viewport projects');

    const longGroup = 'StuffsadsadadasdasdasdasdasdasdasdasdasdasdaszdasdsddsdsdsFddgd';
    await registerMocks(page, {
      authenticated: true,
      customProfiles: [{
        id: 'prof1', profile_id: 'prof1', name: 'Default',
        settings: {
          custom_rules: [
            { id: 'r1', action: 'block', value: '*.ebay.com', group: longGroup, order: 0 },
          ],
        },
      }],
    });

    await page.goto('/custom-rules');

    const nameEl = page.getByText(longGroup, { exact: false }).first();
    await expect(nameEl).toBeVisible();

    // The name must truncate within the viewport, not spill past its right edge.
    // (The app clips overflow-x globally, so a document-level overflow check would
    // miss this — assert the element's own layout box stays on-screen instead.)
    const box = await nameEl.boundingBox();
    const viewportWidth = page.viewportSize()?.width ?? 0;
    expect(box).not.toBeNull();
    expect(box!.x + box!.width).toBeLessThanOrEqual(viewportWidth);
  });
});
