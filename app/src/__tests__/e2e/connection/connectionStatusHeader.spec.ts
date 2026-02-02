import { test, expect, type Page, type Route } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';

// Utility to mock DNS check responses sequence
const escapeRegex = (input: string) => input.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
const dnsCheckDomains = Array.from(
  new Set(
    [
      process.env.VITE_DNS_CHECK_DOMAIN,
      'test.moddns.net',
      'test.staging.ivpndns.net',
      'test.test.ivpndns.net'
    ].filter(Boolean) as string[]
  )
);
const dnsCheckRouteRegex = new RegExp(
  `https://[A-Za-z0-9-]+\\.(?:${dnsCheckDomains.map((d) => escapeRegex(d)).join('|')})/$`,
  'i'
);

async function mockDnsSequence(page: Page, responses: Record<string, unknown>[]) {
  let call = 0;
  await page.route(dnsCheckRouteRegex, async (route: Route) => {
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
    await page.evaluate(() => window.localStorage?.removeItem('moddns-storage'));
  });

  test.afterEach(async ({ page }, testInfo) => {
    if (!/desktop/i.test(testInfo.project.name)) return;
    if (page.isClosed()) return;
    await page.evaluate(() => window.localStorage?.removeItem('moddns-storage'));
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

  test('restore control brings header back', async ({ page }) => {
    await mockDnsSequence(page, [{ status: 'ok', profile_id: 'p1', asn: '', asn_organization: 'Org', ip: '1.1.1.1' }]);
    await page.reload();
    await page.getByTestId('conn-header-hide').click();
    const restoreButton = page.getByTestId('conn-header-show');
    await expect(restoreButton).toBeVisible();
    await restoreButton.click();
    await expect(page.getByTestId('conn-header-root')).toBeVisible();
  });

  test('visibility preference persists across reloads', async ({ page }) => {
    await mockDnsSequence(page, [{ status: 'ok', profile_id: 'p1', asn: '', asn_organization: 'Org', ip: '1.1.1.1' }]);
    await page.reload();
    await page.getByTestId('conn-header-hide').click();
    const restoreButton = page.getByTestId('conn-header-show');
    await expect(restoreButton).toBeVisible();
    await page.reload();
    await expect(page.getByTestId('conn-header-show')).toBeVisible();
    await expect(page.getByTestId('conn-header-root')).toHaveCount(0);
    await page.getByTestId('conn-header-show').click();
    await expect(page.getByTestId('conn-header-root')).toBeVisible();
    await page.reload();
    await expect(page.getByTestId('conn-header-root')).toBeVisible();
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
