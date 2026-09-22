import { test, expect } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';

test.describe('@layout Setup side panel desktop', { tag: '@desktop' }, () => {
  test('opens platform guide in side panel on desktop', async ({ page }) => {
  await registerMocks(page, { authenticated: true, customProfiles: [{ id: 'prof1', profile_id: 'prof1', name: 'Default', settings: { custom_rules: [] } }] });

    await page.goto('/setup');

    const windowsCard = page.getByTestId('setup-platform-card-desktop-windows');
    await expect(windowsCard).toBeVisible();

    await windowsCard.click();

    const panel = page.getByTestId('setup-guide-panel');
    await expect(panel).toHaveAttribute('data-mode', 'sidepanel');

    const width = await panel.evaluate(el => (el as HTMLElement).getBoundingClientRect().width);
    expect(width).toBeGreaterThan(550);
    expect(width).toBeLessThan(640);

    await expect(page.getByTestId('setup-guide-title')).toHaveText(/Windows setup/i);
  });
});
