import { test, expect } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';

// Regression guard: /announcements must be reachable by logged-out users.
// (It lives under PublicLayout and is allow-listed in App.tsx's isPublicPath;
// without that allow-list the unauthenticated safeguard bounces to /login.)
test.describe('@functional Announcements public access', () => {
  test('logged-out user can open /announcements without being redirected to /login', async ({ page }) => {
    await registerMocks(page, { authenticated: false });
    // Stub the feed so the page reaches a deterministic state regardless of API.
    await page.route('**/api/v1/announcements', (route) =>
      route.fulfill({ status: 200, contentType: 'application/json', body: '[]' })
    );

    await page.goto('/announcements');

    // Renders inside the public layout, shows its heading, and stays on the route.
    await page.waitForSelector('[data-testid="public-layout"]', { state: 'attached', timeout: 5000 });
    await expect(page.getByRole('heading', { name: 'Announcements' })).toBeVisible();
    expect(new URL(page.url()).pathname).toBe('/announcements');
  });
});
