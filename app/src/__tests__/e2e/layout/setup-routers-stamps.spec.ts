import { test, expect, type Route } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';

// Fixture stamps — these are real sdns:// strings produced by the api stamp service
// (decoded values verified in api/service/dnsstamp/service_test.go). Hard-coding
// the strings means the E2E test exercises the UI without re-running the encoder.
const STAMPS_NO_DEVICE = {
  doh: 'sdns://AgcAAAAAAAAAAA0xLjEuMS4xAA5kbnMubW9kZG5zLm5ldBYvZG5zLXF1ZXJ5L2FiYzEyM2RlZjQ',
  dot: 'sdns://AwcAAAAAAAAAABEyMDQuMTEuMTQuMjU6ODUzABRhYmMxMjNkZWY0LmRucy5tb2RkbnMubmV0',
  doq: 'sdns://BAcAAAAAAAAAABEyMDQuMTEuMTQuMjU6ODUzABRhYmMxMjNkZWY0LmRucy5tb2RkbnMubmV0',
};
const STAMPS_WITH_DEVICE = {
  doh: 'sdns://AgcAAAAAAAAAAA0xLjEuMS4xAA5kbnMubW9kZG5zLm5ldCYvZG5zLXF1ZXJ5L2FiYzEyM2RlZjQvTGl2aW5nJTIwUm9vbQ',
  dot: 'sdns://AwcAAAAAAAAAABEyMDQuMTEuMTQuMjU6ODUzAB5MaXZpbmctLVJvb20tYWJjMTIzZGVmNC5kbnMubW9kZG5zLm5ldA',
  doq: 'sdns://BAcAAAAAAAAAABEyMDQuMTEuMTQuMjU6ODUzAB5MaXZpbmctLVJvb20tYWJjMTIzZGVmNC5kbnMubW9kZG5zLm5ldA',
};

(test.describe as typeof test.describe)('@layout @desktop Setup → Routers → DNS Stamps tab', () => {
  test('renders three stamps, tooltip toggles, advanced disclosure triggers refetch with device id', async ({ page }) => {
    test.skip(!/-desktop$/i.test(test.info().project.name), 'Run only on *-desktop project');

    // Track how many times the API was called and with which payload so we can
    // assert the debounced refetch actually happened with the device id.
    const calls: Array<{ profile_id?: string; device_id?: string }> = [];

    await registerMocks(page, {
      authenticated: true,
      customProfiles: [{ id: 'abc123def4', profile_id: 'abc123def4', name: 'Default', settings: { custom_rules: [] } }],
    });

    // Register AFTER registerMocks so this handler takes precedence over the
    // catch-all `/api/v1/` route registered inside registerMocks (which would
    // otherwise call r.continue() on POSTs and send them to a dead backend).
    // Also handle the CORS preflight — cross-origin POST + JSON triggers OPTIONS.
    // CORS: api.ts sets `withCredentials: true`, so the response must include
    // `Access-Control-Allow-Credentials: true` AND `Access-Control-Allow-Origin`
    // set to the exact request origin (NOT '*' — that combination is rejected
    // by browsers per the CORS spec).
    await page.route(/\/api\/v1\/dnsstamp(\/?|\?.*)$/i, async (r: Route) => {
      // headerValue() is async — must be awaited. The page is served at
      // http://localhost:5173 (vite dev server), api calls go to http://localhost:3000.
      const origin = (await r.request().headerValue('origin')) ?? 'http://localhost:5173';
      const corsHeaders = {
        'Access-Control-Allow-Origin': origin,
        'Access-Control-Allow-Credentials': 'true',
        'Vary': 'Origin',
      };

      const method = r.request().method();
      if (method === 'OPTIONS') {
        return r.fulfill({
          status: 200,
          headers: {
            ...corsHeaders,
            'Access-Control-Allow-Methods': 'POST, OPTIONS',
            'Access-Control-Allow-Headers': 'content-type, cookie',
          },
          body: '',
        });
      }
      if (method !== 'POST') {
        return r.continue();
      }
      const body = r.request().postDataJSON() as { profile_id?: string; device_id?: string };
      calls.push(body);
      const payload = body.device_id ? STAMPS_WITH_DEVICE : STAMPS_NO_DEVICE;
      return r.fulfill({
        status: 200,
        headers: corsHeaders,
        contentType: 'application/json',
        body: JSON.stringify(payload),
      });
    });

    await page.goto('/setup');

    // Navigate Setup → Routers
    const routersCard = page.getByTestId('setup-platform-card-desktop-routers');
    await expect(routersCard).toBeVisible();
    await routersCard.click();

    // The "DNS Stamps" tab button lives in the Routers guide tab list.
    const stampsTabButton = page.getByRole('button', { name: /^DNS Stamps$/ });
    await expect(stampsTabButton).toBeVisible();
    await stampsTabButton.click();

    // Tab body becomes visible.
    const tab = page.getByTestId('stamps-tab');
    await expect(tab).toBeVisible();

    // Three stamps render — wait for initial fetch.
    await expect.poll(() => calls.length, { timeout: 5000 }).toBeGreaterThanOrEqual(1);
    await expect(tab.getByText(STAMPS_NO_DEVICE.doh, { exact: false })).toBeVisible();
    await expect(tab.getByText(STAMPS_NO_DEVICE.dot, { exact: false })).toBeVisible();
    await expect(tab.getByText(STAMPS_NO_DEVICE.doq, { exact: false })).toBeVisible();

    // The explainer text is persistent — no click required.
    await expect(tab.getByText(/DNS Stamps bundle a resolver's address/)).toBeVisible();
    // The trust pills row is persistent too.
    await expect(tab.getByText(/Resolver advertises:/)).toBeVisible();
    await expect(tab.getByText(/^DNSSEC$/)).toBeVisible();
    await expect(tab.getByText(/^No logs$/)).toBeVisible();
    // Per-protocol compatibility hints are rendered alongside each stamp.
    await expect(tab.getByText(/Works with: UniFi Network/)).toBeVisible();
    await expect(tab.getByText(/Works with: Android Private DNS/)).toBeVisible();
    await expect(tab.getByText(/Works with: AdGuard Home, recent dnscrypt-proxy/)).toBeVisible();

    // dnscrypt.info reference is a real external link that opens in a new tab.
    const specLink = tab.getByRole('link', { name: /dnscrypt\.info\/stamps-specifications/i });
    await expect(specLink).toHaveAttribute('href', /dnscrypt\.info\/stamps-specifications/);
    await expect(specLink).toHaveAttribute('target', '_blank');

    // Privacy-policy link clarifies what "No logs" means in practice.
    const privacyLink = tab.getByRole('link', { name: /How modDNS handles logs/i });
    await expect(privacyLink).toHaveAttribute('href', '/privacy');
    await expect(privacyLink).toHaveAttribute('target', '_blank');

    // Advanced disclosure starts collapsed.
    const advanced = page.getByTestId('stamps-advanced');
    const deviceInput = page.getByTestId('stamps-device-input');
    await expect(advanced).not.toHaveAttribute('open', '');
    await expect(deviceInput).toBeHidden();

    // Expand the disclosure.
    await advanced.locator('summary').click();
    await expect(deviceInput).toBeVisible();

    // Type a device label — debounced refetch fires ~300ms later with the device id.
    const callsBefore = calls.length;
    await deviceInput.fill('Living Room');

    // Wait for at least one new call carrying the device id.
    await expect.poll(
      () => calls.find((c, i) => i >= callsBefore && c.device_id === 'Living Room') ?? null,
      { timeout: 5000 }
    ).not.toBeNull();

    // Stamps update to the device-scoped variants.
    await expect(tab.getByText(STAMPS_WITH_DEVICE.doh, { exact: false })).toBeVisible();
    await expect(tab.getByText(STAMPS_WITH_DEVICE.dot, { exact: false })).toBeVisible();
    await expect(tab.getByText(STAMPS_WITH_DEVICE.doq, { exact: false })).toBeVisible();
  });
});
