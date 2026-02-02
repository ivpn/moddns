import { test, expect, type Page, type Route } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';

async function mockDnsSequence(page: Page, responses: Record<string, unknown>[]) {
  let call = 0;
  await page.route(/https:\/\/.*\..*\/$/i, async (route: Route) => {
    const r = responses[Math.min(call, responses.length - 1)];
    call++;
    if (r.status === 404) {
      return route.fulfill({ status: 404, contentType: 'application/json', body: JSON.stringify(r.body) });
    }
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(r) });
  });
}

test.describe('Mobile ConnectionStatusBar', () => {
  test.use({ viewport: { width: 375, height: 800 } });

  test.beforeEach(async ({ page }, testInfo) => {
    // Skip on desktop - mobile bar only appears on mobile viewports
    test.skip(/desktop/i.test(testInfo.project.name), 'Mobile bar tests require mobile viewport');
    await registerMocks(page, { authenticated: true, customProfiles: [{ id: 'prof1', profile_id: 'prof1', name: 'Default' }] });
    await page.goto('/setup');
  });

  test('renders mobile bar and hides desktop header', async ({ page }) => {
    await mockDnsSequence(page, [{ status: 'ok', profile_id: 'p1', asn: '', asn_organization: 'Org', ip: '1.1.1.1' }]);
    await page.reload();
    await expect(page.getByTestId('conn-mobile-root')).toBeVisible();
    await expect(page.getByTestId('conn-header-root')).toHaveCount(0);
  });

  test('message persists across poll transition', async ({ page }) => {
    await mockDnsSequence(page, [
      { status: 'ok', profile_id: 'p1', asn: '', asn_organization: 'Org', ip: '1.1.1.1' },
      { status: 'ok', profile_id: 'p1', asn: '', asn_organization: 'Org2', ip: '1.1.1.1' }
    ]);
    await page.reload();
    const message = page.getByTestId('conn-mobile-message');
    await expect(message).not.toHaveText('');
  });

  test('disconnected 404 mobile', async ({ page }) => {
    await mockDnsSequence(page, [{ status: 404, body: { error: 'disconnected' } }]);
    await page.reload();
    const badge = page.getByTestId('conn-mobile-badge');
    await expect(badge).toBeVisible();
  });
});
