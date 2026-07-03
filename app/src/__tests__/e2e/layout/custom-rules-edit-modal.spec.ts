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

  // Regression: the open group dropdown option must truncate, not spill under the
  // check icon / off the viewport edge (QA issue #634 follow-up — "Cutoff issue in
  // the group dropdown inside the Edit rule modal when the group name is too long").
  test('open group dropdown option stays on screen', async ({ page }) => {
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
    await expect(page.getByRole('dialog')).toBeVisible();

    // Open the group <Select> and inspect the long-named option.
    await page.locator('#rule-edit-group').click();
    const option = page.getByRole('option', { name: longGroup });
    await expect(option).toBeVisible();

    // The label must genuinely truncate with an ellipsis: a block box whose content
    // overflows its clipped width. An inline `truncate` span (the old bug) ignores
    // overflow, so its content is never clipped (scrollWidth === clientWidth).
    const truncation = await option.evaluate((el) => {
      // The leaf span holding the label text (Radix wraps it in an ItemText span).
      const label = Array.from(el.querySelectorAll('span')).find(
        (s) => s.textContent === el.textContent && s.querySelectorAll('span').length === 0,
      ) as HTMLElement | undefined;
      if (!label) return null;
      const cs = getComputedStyle(label);
      return {
        textOverflow: cs.textOverflow,
        overflowX: cs.overflowX,
        clipped: label.scrollWidth > label.clientWidth,
      };
    });
    expect(truncation).not.toBeNull();
    expect(truncation!.textOverflow).toBe('ellipsis');
    expect(truncation!.overflowX).toBe('hidden');
    expect(truncation!.clipped).toBe(true);

    // And it must not spill off the viewport edge.
    const optionBox = await option.boundingBox();
    const viewportWidth = page.viewportSize()?.width ?? 0;
    expect(optionBox).not.toBeNull();
    expect(optionBox!.x + optionBox!.width).toBeLessThanOrEqual(viewportWidth);
  });
});
