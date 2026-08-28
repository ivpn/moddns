import { test, expect } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';

// Mechanism guard for the mobile report "the modDNS header disappears at the bottom
// of the logs": the app header has no hide-on-scroll behaviour, so it must stay
// pinned at y=0 at any scroll depth — including parked at the bottom of a long list
// and through a refresh-from-bottom (whose document-height collapse used to clamp
// scrollY and chain pagination fetches). WebKit-only: the report is iOS-specific and
// sticky handling differs per engine. Playwright cannot emulate the iOS URL bar or
// rubber-band overscroll; a device retest covers those.

const profile = { id: 'prof1', profile_id: 'prof1', name: 'Default', settings: { logs: { enabled: true } } };

const row = (i: number) => ({
    profile_id: 'prof1',
    timestamp: `2026-08-12T10:${Math.floor(i / 60).toString().padStart(2, '0')}:${(i % 60).toString().padStart(2, '0')}Z`,
    status: 'processed',
    protocol: 'dns',
    device_id: `device-${i}`,
    client_ip: `10.0.0.${i % 250}`,
    dns_request: { domain: `row-${i}.example.test`, query_type: 'A' },
});

test.describe('Logs mobile bottom header', () => {
    test('app header stays pinned at the bottom of a long list and through a refresh', async ({ page }) => {
        test.skip(!/iphone15pro/i.test(test.info().project.name), 'Only run on iPhone project');

        await registerMocks(page, { authenticated: true, customProfiles: [profile] });
        await page.route(/\/api\/v1\/profiles\/prof1\/logs(\?|$)/i, route => {
            const url = new URL(route.request().url());
            const pageNum = parseInt(url.searchParams.get('page') || '1', 10);
            const count = pageNum === 1 ? 100 : 25;
            const offset = pageNum === 1 ? 0 : 100 + (pageNum - 2) * 25;
            route.fulfill({
                status: 200,
                contentType: 'application/json',
                body: JSON.stringify(Array.from({ length: count }).map((_, i) => row(offset + i))),
            });
        });
        await page.goto('/query-logs');
        await expect(page.getByTestId('querylog-card-toggle')).toHaveCount(100);

        const header = page.getByTestId('app-header-wrapper');
        await expect(header).toBeVisible();

        await page.evaluate(() => window.scrollTo(0, document.body.scrollHeight));
        await expect.poll(() => page.evaluate(() => window.scrollY)).toBeGreaterThan(500);
        await expect(header).toBeVisible();
        expect(Math.abs((await header.boundingBox())!.y)).toBeLessThanOrEqual(1);

        // Refresh from the bottom: the header must remain pinned through the replace.
        await page.locator('[data-testid="logs-refresh-button"]:visible').click();
        await expect.poll(() => page.evaluate(() => window.scrollY)).toBe(0);
        await expect(header).toBeVisible();
        expect(Math.abs((await header.boundingBox())!.y)).toBeLessThanOrEqual(1);
    });
});
