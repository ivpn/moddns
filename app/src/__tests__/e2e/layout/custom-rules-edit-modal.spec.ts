import { test, expect } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';

// Regression: the "Edit rule" modal must not break out when a rule's group name
// is very long (QA issue #634 — "The Edit modal breaks when the group name is too
// long"). The group <Select> value should truncate inside the dialog rather than
// expand it or spill past its right edge.
(test.describe as typeof test.describe)('@layout Custom Rules edit modal long group name', () => {
  test('group select stays within the modal', async ({ page }) => {
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
    await page.getByRole('button', { name: 'Edit rule' }).first().click();

    const dialog = page.getByRole('dialog');
    await expect(dialog).toBeVisible();

    // The group trigger must not extend past the dialog's right edge.
    const dialogBox = await dialog.boundingBox();
    const triggerBox = await page.locator('#rule-edit-group').boundingBox();
    expect(dialogBox).not.toBeNull();
    expect(triggerBox).not.toBeNull();
    expect(triggerBox!.x + triggerBox!.width).toBeLessThanOrEqual(dialogBox!.x + dialogBox!.width);
  });
});
