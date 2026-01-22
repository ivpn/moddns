import { test, expect } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';

const SERVICES_ENDPOINT = /\/api\/v1\/services(\/?|\?.*)$/i;
const PROFILE_GET_ENDPOINT = /\/api\/v1\/profiles\/([^/]+)(\/?|\?.*)$/i;
const PROFILE_SERVICES_ENDPOINT = /\/api\/v1\/profiles\/([^/]+)\/services(\/?|\?.*)$/i;

test.describe('@functional services tab', () => {
  test('shows services catalog and allows toggling', async ({ page }) => {
    // Provide a single profile so active profile resolution is straightforward.
    await registerMocks(page, {
      authenticated: true,
      profilesCount: 1,
    });

    const profileId = 'p1';
    let blocked: string[] = [];

    await page.route(SERVICES_ENDPOINT, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          services: [
            { id: 'google', name: 'Google', logo_key: 'google', asns: [15169] },
          ],
        }),
      });
    });

    // Capture enable/disable operations.
    await page.route(PROFILE_SERVICES_ENDPOINT, async (route) => {
      const method = route.request().method();
      const body = route.request().postDataJSON() as { service_ids?: string[] };
      const ids = body?.service_ids ?? [];

      if (method === 'POST') {
        blocked = Array.from(new Set([...blocked, ...ids]));
        return route.fulfill({ status: 200, body: '' });
      }
      if (method === 'DELETE') {
        blocked = blocked.filter((id) => !ids.includes(id));
        return route.fulfill({ status: 200, body: '' });
      }
      return route.continue();
    });

    // Profile GET used after toggling.
    await page.route(PROFILE_GET_ENDPOINT, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          account_id: 'mock-account-id',
          id: profileId,
          name: 'Profile 1',
          profile_id: profileId,
          settings: {
            profile_id: profileId,
            privacy: {
              default_rule: 'allow',
              subdomains_rule: 'allow',
              blocklists: ['bl-basic'],
              services: { blocked },
            },
            logs: {
              enabled: false,
              log_clients_ips: false,
              log_domains: false,
              retention: 0,
            },
            advanced: {},
            security: {},
            statistics: { enabled: false },
            custom_rules: [],
          },
        }),
      });
    });

    await page.goto('/blocklists');

    await page.getByRole('tab', { name: 'Services' }).click();

    const cards = page.getByTestId('service-card');
    await expect(cards.first()).toBeVisible();
    await expect(cards.first()).toContainText('Google');

    // Toggle on; should stay on after profile refresh.
    await cards.first().locator('[data-slot="switch"]').click();

    // We can't reliably assert toast text across environments; assert the switch is checked.
    await expect(cards.first().locator('[data-slot="switch"]')).toHaveAttribute('data-state', 'checked');
  });
});
