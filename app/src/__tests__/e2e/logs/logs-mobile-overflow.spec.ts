import { test, expect } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';

// Assumes mobile project config sets viewport (e.g., iPhone) and baseURL.
// Verifies logs page has no horizontal overflow and key containers fit within viewport width.

test.describe('Logs mobile layout', () => {
  test('no horizontal overflow and containers fit', async ({ page }) => {
    await registerMocks(page, {
      authenticated: true,
      customProfiles: [{ id: 'prof1', profile_id: 'prof1', name: 'Default', settings: { logs: { enabled: true } } }],
      extraRoutes: async (p) => {
        // Provide a deterministic set of logs with long domain to challenge layout
        await p.route(/\/api\/v1\/profiles\/prof1\/logs/i, route => {
          const now = new Date().toISOString();
            const items = Array.from({ length: 3 }).map((_, i) => ({
              profile_id: 'prof1',
              timestamp: now,
              status: i === 1 ? 'blocked' : 'processed',
              protocol: 'dns',
              device_id: 'device-mobile-long-id',
              client_ip: '10.0.0.' + i,
              dns_request: { domain: `very-very-long-test-subdomain-${i}.example-reallylongdomainforlayout-validation.test` }
            }));
            route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(items) });
        });
      }
    });

  await page.goto('/query-logs');

    // Wait for filters to render
    // Prefer scroll container if logs present, else use empty state container
    const scrollContainer = page.getByTestId('logs-scroll-container');
    const emptyState = page.getByTestId('logs-empty-state');
    // Wait up to 10s for either container to appear
    const appeared = await Promise.race([
      scrollContainer.first().waitFor({ state: 'attached', timeout: 10000 }).then(() => 'scroll').catch(() => null),
      emptyState.first().waitFor({ state: 'attached', timeout: 10000 }).then(() => 'empty').catch(() => null)
    ]);
    expect(appeared, 'Expected logs scroll container or empty state to appear').not.toBeNull();

    // Inject a long domain entry scenario by ensuring at least one log card or fallback appears
    // If no logs present, the empty state should also not overflow horizontally.

    // Evaluate layout metrics in browser context
  const result = await page.evaluate(() => {
      const docEl = document.documentElement;
      const body = document.body;
      const vw = window.innerWidth;
      const sc = document.querySelector('[data-testid="logs-scroll-container"]') as HTMLElement | null;
      const scrollWidthDoc = Math.max(
        body.scrollWidth,
        docEl.scrollWidth,
        body.offsetWidth,
        docEl.offsetWidth
      );
      const scOverflow = sc ? sc.scrollWidth - sc.clientWidth : 0;
      return { vw, scrollWidthDoc, scOverflow, scClient: sc?.clientWidth, scScroll: sc?.scrollWidth };
    });

    // Assertions: document width should not exceed viewport significantly (allow 1px tolerance)
    expect(result.scrollWidthDoc).toBeLessThanOrEqual(result.vw + 1);
    // Scroll container should not have horizontal overflow
    expect(result.scOverflow).toBeLessThanOrEqual(1);

    // Additionally confirm no body horizontal scrollbar via CSS overflow values
    const hasHorizontalScrollbar = await page.evaluate(() => {
      return window.innerHeight < document.documentElement.clientHeight || document.documentElement.scrollWidth > window.innerWidth;
    });
    expect(hasHorizontalScrollbar).toBeFalsy();
  });

  test('whole-card expansion: every row expands, quick-rule is excluded, no overflow with long labels', async ({ page }) => {
    await registerMocks(page, {
      authenticated: true,
      customProfiles: [{ id: 'prof1', profile_id: 'prof1', name: 'Default', settings: { logs: { enabled: true } } }]
    });

    // Register the logs route AFTER registerMocks so it is tested BEFORE the catch-all
    // route (Playwright matches routes in reverse registration order). The catch-all in
    // registerMocks matches `/api/v1/profiles` and would otherwise shadow this endpoint,
    // returning the profiles array instead of our logs payload.
    const now = new Date().toISOString();
    const items = [
      // Blocked row WITH reasons — deliberately long ids to challenge layout / overflow
      {
        profile_id: 'prof1',
        timestamp: now,
        status: 'blocked',
        protocol: 'dns',
        device_id: 'device-with-reasons',
        client_ip: '10.0.0.1',
        dns_request: { domain: 'blocked-with-reasons.example-longdomainforlayout-validation.test' },
        reasons: [
          'blocklist: very-long-blocklist-identifier-xxxxxxxxxxxxxxxxxxxx',
          'service: another-long-service-id-yyyyyyyyyyyyyyyyyyyy'
        ]
      },
      // Processed row WITHOUT reasons — now also expandable (detail grid, no reasons block)
      {
        profile_id: 'prof1',
        timestamp: now,
        status: 'processed',
        protocol: 'dns',
        device_id: 'device-no-reasons',
        client_ip: '10.0.0.2',
        dns_request: { domain: 'processed-no-reasons.example.test' }
      }
    ];
    await page.route(/\/api\/v1\/profiles\/prof1\/logs/i, route => {
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(items) });
    });

    await page.goto('/query-logs');

    const scrollContainer = page.getByTestId('logs-scroll-container');
    await scrollContainer.first().waitFor({ state: 'attached', timeout: 10000 });

    // Every row is expandable now → one toggle + one panel per mocked row (2).
    const toggles = page.getByTestId('querylog-card-toggle');
    await expect(toggles).toHaveCount(items.length);
    const panels = page.getByTestId('querylog-expanded-panel');
    await expect(panels).toHaveCount(items.length);

    const firstPanel = panels.nth(0);
    const secondPanel = panels.nth(1);

    // Collapsed initial state
    await expect(toggles.nth(0)).toHaveAttribute('aria-expanded', 'false');
    await expect(firstPanel).toHaveAttribute('data-expanded', 'false');

    // Quick-rule button is excluded from the overlay: clicking it must NOT expand the card.
    await page.getByTestId('logs-quick-rule-button').nth(0).click();
    await expect(firstPanel).toHaveAttribute('data-expanded', 'false');
    // Close any sheet the quick-rule action opened so it doesn't cover the cards below.
    await page.keyboard.press('Escape');

    // Keyboard: focus the first card's toggle and press Enter to expand.
    await toggles.nth(0).focus();
    await page.keyboard.press('Enter');
    await expect(toggles.nth(0)).toHaveAttribute('aria-expanded', 'true');
    await expect(firstPanel).toHaveAttribute('data-expanded', 'true');

    // Expanded panel shows the detail grid (protocol + timestamp always render) and the reasons block.
    await expect(firstPanel.getByTestId('querylog-detail-grid')).toBeVisible();
    await expect(firstPanel.getByTestId('querylog-detail-protocol')).toBeVisible();
    await expect(firstPanel.getByTestId('querylog-detail-timestamp')).toBeVisible();
    await expect(firstPanel.getByTestId('querylog-reasons')).toBeVisible();

    // The processed (no-reasons) row expands too: detail grid visible, but no reasons block.
    await toggles.nth(1).click();
    await expect(secondPanel).toHaveAttribute('data-expanded', 'true');
    await expect(secondPanel.getByTestId('querylog-detail-grid')).toBeVisible();
    await expect(secondPanel.getByTestId('querylog-reasons')).toHaveCount(0);

    // Re-run overflow assertions AFTER expansion, with the long labels rendered
    const result = await page.evaluate(() => {
      const docEl = document.documentElement;
      const body = document.body;
      const vw = window.innerWidth;
      const sc = document.querySelector('[data-testid="logs-scroll-container"]') as HTMLElement | null;
      const scrollWidthDoc = Math.max(
        body.scrollWidth,
        docEl.scrollWidth,
        body.offsetWidth,
        docEl.offsetWidth
      );
      const scrollingElWidth = document.scrollingElement ? document.scrollingElement.scrollWidth : docEl.scrollWidth;
      const scOverflow = sc ? sc.scrollWidth - sc.clientWidth : 0;
      return { vw, scrollWidthDoc, scrollingElWidth, scOverflow };
    });
    expect(result.scrollingElWidth).toBeLessThanOrEqual(result.vw + 1);
    expect(result.scrollWidthDoc).toBeLessThanOrEqual(result.vw + 1);
    expect(result.scOverflow).toBeLessThanOrEqual(1);

    // The expanded panel's bounding box must sit within the viewport horizontally
    const viewport = page.viewportSize();
    const viewportWidth = viewport?.width ?? result.vw;
    const box = await firstPanel.boundingBox();
    expect(box, 'Expected expanded panel to have a bounding box').not.toBeNull();
    if (box) {
      expect(box.x).toBeGreaterThanOrEqual(0);
      expect(box.x + box.width).toBeLessThanOrEqual(viewportWidth + 1);
    }
  });

  test('expanded card collapses when clicking anywhere, including the expanded panel', async ({ page }) => {
    await registerMocks(page, {
      authenticated: true,
      customProfiles: [{ id: 'prof1', profile_id: 'prof1', name: 'Default', settings: { logs: { enabled: true } } }]
    });
    const now = new Date().toISOString();
    const items = [
      {
        profile_id: 'prof1', timestamp: now, status: 'blocked', protocol: 'dns',
        device_id: 'd1', client_ip: '10.0.0.1',
        dns_request: { domain: 'blocked.example.test' },
        reasons: ['blocklist: some-blocklist-id']
      }
    ];
    await page.route(/\/api\/v1\/profiles\/prof1\/logs/i, route => {
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(items) });
    });

    await page.goto('/query-logs');
    await page.getByTestId('logs-scroll-container').first().waitFor({ state: 'attached', timeout: 10000 });

    const toggle = page.getByTestId('querylog-card-toggle').first();
    const panel = page.getByTestId('querylog-expanded-panel').first();

    // Expand.
    await toggle.click();
    await expect(panel).toHaveAttribute('data-expanded', 'true');

    // Click inside the EXPANDED PANEL region (below the header) — must collapse the card.
    const box = await panel.boundingBox();
    expect(box, 'Expected an expanded panel bounding box').not.toBeNull();
    if (box) {
      await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2);
    }
    await expect(panel).toHaveAttribute('data-expanded', 'false');
  });

  test('mobile: one-time expand hint shows, dismisses after first expand, and stays gone', async ({ page }, testInfo) => {
    // The hint is mobile-only (md:hidden); skip on desktop projects.
    test.skip(!/(chromium-mobile|iphone15pro)/i.test(testInfo.project.name), 'mobile-only hint');

    await registerMocks(page, {
      authenticated: true,
      customProfiles: [{ id: 'prof1', profile_id: 'prof1', name: 'Default', settings: { logs: { enabled: true } } }]
    });
    const now = new Date().toISOString();
    const items = [
      { profile_id: 'prof1', timestamp: now, status: 'processed', protocol: 'dns', device_id: 'd1', client_ip: '10.0.0.1', dns_request: { domain: 'a.example.test' } },
      { profile_id: 'prof1', timestamp: now, status: 'blocked', protocol: 'dns', device_id: 'd2', client_ip: '10.0.0.2', dns_request: { domain: 'b.example.test' } }
    ];
    await page.route(/\/api\/v1\/profiles\/prof1\/logs/i, route => {
      route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(items) });
    });

    await page.goto('/query-logs');
    await page.getByTestId('logs-scroll-container').first().waitFor({ state: 'attached', timeout: 10000 });

    // Hint is visible on first visit.
    const hint = page.getByTestId('logs-expand-hint');
    await expect(hint).toBeVisible();

    // Expanding a row dismisses the hint.
    await page.getByTestId('querylog-card-toggle').first().click();
    await expect(hint).toHaveCount(0);

    // Persisted: reload keeps it gone.
    await page.reload();
    await page.getByTestId('logs-scroll-container').first().waitFor({ state: 'attached', timeout: 10000 });
    await expect(page.getByTestId('logs-expand-hint')).toHaveCount(0);
  });
});
