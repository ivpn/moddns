import { test, expect, type Route } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';

// Fixture DoH stamp — a real sdns:// string (proto 0x02) produced by the api
// stamp service. The Linux dnscrypt-proxy tab only ever fetches with an empty
// device id, so a single fixture is enough.
const STAMP_DOH = 'sdns://AgcAAAAAAAAAAA0xLjEuMS4xAA5kbnMubW9kZG5zLm5ldBYvZG5zLXF1ZXJ5L2FiYzEyM2RlZjQ';

(test.describe as typeof test.describe)('@layout @desktop Setup → Linux → dnscrypt-proxy tab', () => {
  test('fetches the DoH stamp and renders a ready-to-paste dnscrypt-proxy.toml block', async ({ page }) => {
    test.skip(!/-desktop$/i.test(test.info().project.name), 'Run only on *-desktop project');

    const calls: Array<{ profile_id?: string; device_id?: string }> = [];

    await registerMocks(page, {
      authenticated: true,
      customProfiles: [{ id: 'abc123def4', profile_id: 'abc123def4', name: 'Default', settings: { custom_rules: [] } }],
    });

    // Register AFTER registerMocks so this handler wins over the catch-all
    // `/api/v1/` route. Cross-origin POST + JSON triggers a CORS preflight, so
    // handle OPTIONS too (see setup-routers-stamps.spec.ts for the full rationale).
    await page.route(/\/api\/v1\/dnsstamp(\/?|\?.*)$/i, async (r: Route) => {
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
      return r.fulfill({
        status: 200,
        headers: corsHeaders,
        contentType: 'application/json',
        body: JSON.stringify({ doh: STAMP_DOH }),
      });
    });

    await page.goto('/setup');

    // Navigate Setup → Linux
    const linuxCard = page.getByTestId('setup-platform-card-desktop-linux');
    await expect(linuxCard).toBeVisible();
    await linuxCard.click();

    // Switch to the dnscrypt-proxy tab.
    const dnscryptTabButton = page.getByRole('button', { name: /^dnscrypt-proxy$/ });
    await expect(dnscryptTabButton).toBeVisible();
    await dnscryptTabButton.click();

    // The tab fetches the DoH stamp itself.
    await expect.poll(() => calls.length, { timeout: 5000 }).toBeGreaterThanOrEqual(1);
    expect(calls[0]?.profile_id).toBe('abc123def4');

    // The config block renders a ready-to-paste TOML snippet built from the stamp.
    const dnscryptConfig = page.getByTestId('dnscrypt-proxy-config');
    await expect(dnscryptConfig).toBeVisible();
    await expect(dnscryptConfig.getByText(/server_names = \['modDNS-abc123def4'\]/)).toBeVisible();
    await expect(dnscryptConfig.getByText(new RegExp(`stamp = '${STAMP_DOH}'`))).toBeVisible();
  });
});
