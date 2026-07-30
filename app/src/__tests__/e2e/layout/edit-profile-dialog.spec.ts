import { test, expect } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';

// #122: in the Edit Profile dialog the "Delete profile" danger card used a
// non-wrapping flex row, so on narrow (mobile) viewports the button overflowed
// the card and overlapped the description text, and the dialog itself
// (max-w-3xl, overriding the primitive's mobile margins) spanned the full
// viewport on phones. The dialog is now capped like the account-preferences
// modals (calc(100vw-2rem) / 500px) and the danger card always stacks the
// button below the description.
test.describe('@layout edit profile dialog', () => {
  test.beforeEach(async ({ page }) => {
    await registerMocks(page, { authenticated: true });
  });

  test('dialog fits the viewport and delete button sits below the description', async ({ page }) => {
    // The profile dropdown is hidden on /home; use the Rules page like the
    // issue's repro steps (Rules tab -> select profile -> edit icon).
    await page.goto('/custom-rules');

    // Open the profile dropdown and its edit (settings) action
    await page.getByRole('combobox').first().click();
    await page.getByTestId('edit-profile-settings').click();

    const dialog = page.locator('[data-slot="dialog-content"]');
    await expect(dialog).toBeVisible();

    // Dialog leaves horizontal breathing room (1rem each side on mobile,
    // 500px cap on larger screens) like the account-preferences modals.
    const viewport = page.viewportSize();
    const dialogBox = await dialog.boundingBox();
    expect(dialogBox).not.toBeNull();
    expect(dialogBox!.width).toBeLessThanOrEqual(Math.min(viewport!.width - 24, 500));
    expect(dialogBox!.x).toBeGreaterThanOrEqual(8);

    const description = page.getByText(/You can delete your profile immediately/);
    await expect(description).toBeVisible();
    const deleteButton = page.getByRole('button', { name: 'Delete profile' });
    await expect(deleteButton).toBeVisible();

    const textBox = await description.boundingBox();
    const buttonBox = await deleteButton.boundingBox();
    expect(textBox).not.toBeNull();
    expect(buttonBox).not.toBeNull();

    // Stacked layout: the button starts below the description at every size.
    expect(buttonBox!.y).toBeGreaterThanOrEqual(textBox!.y + textBox!.height - 1);
  });
});
