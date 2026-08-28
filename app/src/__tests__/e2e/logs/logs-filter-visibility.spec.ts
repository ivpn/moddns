import { test, expect, type Page } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';

// Active-filter visibility (issue item 3): accent on non-default triggers, a clear-all
// chip, and a filtered empty state that offers clearing instead of the onboarding CTA.

const profile = { id: 'prof1', profile_id: 'prof1', name: 'Default', settings: { logs: { enabled: true } } };

const rows = [
    { profile_id: 'prof1', timestamp: '2026-08-13T10:00:01Z', status: 'processed', protocol: 'dns', device_id: 'd1', client_ip: '10.0.0.1', dns_request: { domain: 'one.example.test', query_type: 'A' } },
    { profile_id: 'prof1', timestamp: '2026-08-13T10:00:00Z', status: 'processed', protocol: 'dns', device_id: 'd1', client_ip: '10.0.0.1', dns_request: { domain: 'two.example.test', query_type: 'A' } },
];

const ACCENT = /border-\[var\(--tailwind-colors-rdns-600\)\]/;

// Register the logs route AFTER registerMocks so it is tested BEFORE the catch-all
// route (Playwright matches routes in reverse registration order). Blocked-status
// requests return no rows so the filtered empty state renders.
async function setupLogsPage(page: Page) {
    await registerMocks(page, { authenticated: true, customProfiles: [profile] });
    await page.route(/\/api\/v1\/profiles\/prof1\/logs(\?|$)/i, route => {
        const url = new URL(route.request().url());
        const body = url.searchParams.get('status') === 'blocked' ? [] : rows;
        route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(body) });
    });
    await page.goto('/query-logs');
    await page.getByTestId('logs-scroll-container').waitFor({ state: 'attached', timeout: 10000 });
}

const visibleByLabel = (page: Page, label: string) =>
    page.locator(`[aria-label="${label}"]:visible`);

test.describe('Logs filter visibility', () => {
    test('active filter accents its trigger, shows the clear chip, and the empty state clears', async ({ page }) => {
        await setupLogsPage(page);
        await expect(page.getByTestId('querylog-card-toggle')).toHaveCount(2);

        const statusTrigger = visibleByLabel(page, 'Filter by status');
        await expect(statusTrigger).not.toHaveClass(ACCENT);
        await expect(page.getByTestId('logs-clear-filters')).toHaveCount(0);

        await statusTrigger.click();
        const blockedOption = page.getByRole('option', { name: 'Blocked' });
        // Options advertise clickability with a pointer cursor.
        expect(await blockedOption.evaluate(el => getComputedStyle(el).cursor)).toBe('pointer');
        await blockedOption.click();

        // Accent + chip appear; the blocked view is empty, so the filtered empty
        // state renders with a Clear filters action (not the DNS-setup CTA).
        await expect(statusTrigger).toHaveClass(ACCENT);
        await expect(page.locator('[data-testid="logs-clear-filters"]:visible')).toBeVisible();
        const emptyState = page.getByTestId('logs-empty-state');
        await expect(emptyState).toBeVisible();
        await expect(emptyState.getByText(/No results for the current filters/)).toBeVisible();
        await expect(emptyState.getByRole('button', { name: /DNS Setup/ })).toHaveCount(0);

        await page.getByTestId('logs-empty-clear-filters').click();
        await expect(page.getByTestId('querylog-card-toggle')).toHaveCount(2);
        await expect(statusTrigger).not.toHaveClass(ACCENT);
        await expect(page.getByTestId('logs-clear-filters')).toHaveCount(0);
    });

    test('search shows a clear button and the chip only after the debounce commits', async ({ page }) => {
        await setupLogsPage(page);
        await expect(page.getByTestId('querylog-card-toggle')).toHaveCount(2);

        const search = page.locator('input[aria-label="Search domain or its part"]:visible');
        await search.fill('example');
        await expect(page.locator('[data-testid="logs-search-clear"]:visible')).toBeVisible();

        // Debounce (500ms) commits the search → the clear-all chip appears.
        await expect(page.locator('[data-testid="logs-clear-filters"]:visible')).toBeVisible();

        await page.locator('[data-testid="logs-search-clear"]:visible').click();
        await expect(search).toHaveValue('');
        await expect(page.getByTestId('logs-clear-filters')).toHaveCount(0);
    });
});
