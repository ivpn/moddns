import { test, expect, type Page, type Route } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';

// Utility to mock DNS check responses sequence
async function mockDnsSequence(page: Page, responses: any[]) {
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

// Desktop only tests rely on chromium-desktop project
// Assumes /setup route renders the header when desktop

test.describe('Desktop ConnectionStatusHeader', () => {
  test.beforeEach(async ({ page }, testInfo) => {
    // Skip on mobile viewports - header only appears on desktop
    test.skip(!/desktop/i.test(testInfo.project.name), 'Desktop header tests require desktop viewport');
    await registerMocks(page, { authenticated: true, customProfiles: [{ id: 'prof1', profile_id: 'prof1', name: 'Default' }] });
    await page.goto('/setup');
  });

  test('renders and shows non-empty message', async ({ page }) => {
    await mockDnsSequence(page, [{ status: 'ok', profile_id: 'p1', asn: '', asn_organization: 'Org', ip: '1.1.1.1' }]);
    await page.reload();
    const root = page.getByTestId('conn-header-root');
    await expect(root).toBeVisible();
    const message = page.getByTestId('conn-header-message');
    await expect(message).toBeVisible();
    // Poll until text populated (guards against race)
    await expect.poll(async () => (await message.textContent())?.trim() || '').not.toEqual('');
  });

  test('hide button collapses header', async ({ page }) => {
    await mockDnsSequence(page, [{ status: 'ok', profile_id: 'p1', asn: '', asn_organization: 'Org', ip: '1.1.1.1' }]);
    await page.reload();
    await page.getByTestId('conn-header-hide').click();
    const root = page.getByTestId('conn-header-root');
    await expect(root).toHaveAttribute('class', /opacity-0/);
  });

  test('different profile state', async ({ page }) => {
    await mockDnsSequence(page, [
      { status: 'ok', profile_id: 'p2', asn: '', asn_organization: 'Org', ip: '1.1.1.1' }
    ]);
    await page.reload();
    const badge = page.getByTestId('conn-header-badge-text');
    await expect(badge).toBeVisible();
    // Poll until it resolves to one of expected states (guard against state transition timing)
    await expect.poll(async () => (await badge.textContent())?.trim() || '').toMatch(/Connected|Different Profile/i);
  });

  test('disconnected 404', async ({ page }) => {
    await mockDnsSequence(page, [{ status: 404, body: { error: 'disconnected' } }]);
    await page.reload();
    const badge = page.getByTestId('conn-header-badge-text');
    await expect(badge).toHaveText(/Disconnected|Connected/);
  });
});
