import { test, expect } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';

// Regression: the "Delete group" confirmation modal must not break out when the
// group name is very long (QA issue #634 — "Overflow issue in the Delete group
// modal when group name is too long"). The interpolated name must wrap/break
// inside the dialog rather than spill past its right edge.
(test.describe as typeof test.describe)('@layout Custom Rules delete group modal long name', () => {
  test('delete confirmation stays within the modal', async ({ page }) => {
    test.skip(!/chromium-desktop/i.test(test.info().project.name), 'desktop layout regression');

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

    // Open the group's actions menu, then the "Delete group" item.
    await page.getByRole('button', { name: 'Group actions' }).first().click();
    await page.getByRole('menuitem', { name: /Delete group/i }).click();

    const dialog = page.getByRole('dialog');
    await expect(dialog).toBeVisible();

    // The description (which interpolates the long name) must stay within the dialog.
    const dialogBox = await dialog.boundingBox();
    const descBox = await dialog.getByText(longGroup, { exact: false }).boundingBox();
    expect(dialogBox).not.toBeNull();
    expect(descBox).not.toBeNull();
    expect(descBox!.x + descBox!.width).toBeLessThanOrEqual(dialogBox!.x + dialogBox!.width + 1);
  });
});
