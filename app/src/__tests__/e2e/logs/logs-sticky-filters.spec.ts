import { test, expect, type Page } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';

// Sticky filter bar (issue item 5): the Filters row pins below the app header while
// the list scrolls, opaque, without introducing horizontal overflow (the enabling
// App.tsx change swaps app-content's overflow-x-hidden for overflow-x-clip — `hidden`
// creates a scroll container that silently breaks position:sticky).

const profile = { id: 'prof1', profile_id: 'prof1', name: 'Default', settings: { logs: { enabled: true } } };

const manyRows = Array.from({ length: 60 }).map((_, i) => ({
    profile_id: 'prof1',
    timestamp: `2026-08-13T10:${Math.floor(i / 60).toString().padStart(2, '0')}:${(59 - (i % 60)).toString().padStart(2, '0')}Z`,
    status: 'processed',
    protocol: 'dns',
    device_id: `device-${i}`,
    client_ip: `10.0.0.${i}`,
    dns_request: { domain: `row-${i}.example.test`, query_type: 'A' },
}));

// Register the logs route AFTER registerMocks so it is tested BEFORE the catch-all
// route (Playwright matches routes in reverse registration order).
async function setupLogsPage(page: Page) {
    await registerMocks(page, { authenticated: true, customProfiles: [profile] });
    await page.route(/\/api\/v1\/profiles\/prof1\/logs(\?|$)/i, route => {
        route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(manyRows) });
    });
    await page.goto('/query-logs');
    await page.getByTestId('logs-scroll-container').waitFor({ state: 'attached', timeout: 10000 });
}

test.describe('Logs sticky filter bar', () => {
    test('filters pin below the header while the list scrolls, opaque, no horizontal overflow', async ({ page }) => {
        await setupLogsPage(page);
        const sticky = page.getByTestId('logs-sticky-filters');

        expect(await sticky.evaluate(el => getComputedStyle(el).position)).toBe('sticky');

        const before = (await sticky.boundingBox())!;
        // scrollTo instead of mouse.wheel — the wheel API is unsupported on the
        // mobile-WebKit project.
        await page.evaluate(() => window.scrollTo(0, 1500));
        await page.waitForFunction(() => window.scrollY > 800);

        const after = (await sticky.boundingBox())!;
        // The bar must NOT have scrolled away with the content...
        expect(after.y).toBeGreaterThan(-1);
        // ...and must sit exactly at its sticky offset: the full header-stack height.
        const expectedTop = await page.evaluate(() =>
            parseFloat(getComputedStyle(document.documentElement).getPropertyValue('--app-header-stack-full')) || 64
        );
        expect(Math.abs(after.y - expectedTop)).toBeLessThanOrEqual(2);
        // Sanity: the page actually scrolled under it.
        expect(before.y).toBeGreaterThanOrEqual(after.y);

        // Opaque surface — scrolled content cannot show through.
        const bg = await sticky.evaluate(el => getComputedStyle(el).backgroundColor);
        expect(bg).not.toBe('rgba(0, 0, 0, 0)');

        // The overflow-x swap must not reintroduce horizontal scrolling.
        const docOverflow = await page.evaluate(() =>
            Math.max(document.body.scrollWidth, document.documentElement.scrollWidth) - window.innerWidth
        );
        expect(docOverflow).toBeLessThanOrEqual(1);
    });
});
