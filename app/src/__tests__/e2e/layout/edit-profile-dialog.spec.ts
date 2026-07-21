import { test, expect } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';

// #122: in the Edit Profile dialog the "Delete profile" danger card used a
// non-wrapping flex row, so on narrow (mobile) viewports the button overflowed
// the card and overlapped the description text. The card must lay the button
// out below the text on mobile and never intersect the description.
test.describe('@layout edit profile dialog danger card', () => {
  test.beforeEach(async ({ page }) => {
    await registerMocks(page, { authenticated: true });
  });

  test('delete button does not overlap the danger card description', async ({ page }) => {
    // The profile dropdown is hidden on /home; use the Rules page like the
    // issue's repro steps (Rules tab -> select profile -> edit icon).
    await page.goto('/custom-rules');

    // Open the profile dropdown and its edit (settings) action
    await page.getByRole('combobox').first().click();
    await page.getByTestId('edit-profile-settings').click();

    const description = page.getByText(/You can delete your profile immediately/);
    await expect(description).toBeVisible();
    const deleteButton = page.getByRole('button', { name: 'Delete profile' });
    await expect(deleteButton).toBeVisible();

    const textBox = await description.boundingBox();
    const buttonBox = await deleteButton.boundingBox();
    expect(textBox).not.toBeNull();
    expect(buttonBox).not.toBeNull();

    const intersects =
      buttonBox!.x < textBox!.x + textBox!.width &&
      buttonBox!.x + buttonBox!.width > textBox!.x &&
      buttonBox!.y < textBox!.y + textBox!.height &&
      buttonBox!.y + buttonBox!.height > textBox!.y;
    expect(intersects, 'delete button must not overlap the description text').toBe(false);
  });
});
