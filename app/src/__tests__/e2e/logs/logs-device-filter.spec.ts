import { test, expect, type Page } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';

// Device filter completeness (issue item 7): the dropdown is seeded from
// GET /profiles/{id}/logs/devices (complete within retention) merged with ids
// observed in fetched rows, and selecting a device never shrinks the list.

const profile = { id: 'prof1', profile_id: 'prof1', name: 'Default', settings: { logs: { enabled: true } } };

const row = (deviceId: string, domain: string) => ({
    profile_id: 'prof1',
    timestamp: '2026-08-14T10:00:00Z',
    status: 'processed',
    protocol: 'dns',
    device_id: deviceId,
    client_ip: '10.0.0.1',
    dns_request: { domain, query_type: 'A' },
});

const serverDevices = [
    { device_id: 'laptop', last_seen: '2026-08-14T10:00:00Z' },
    { device_id: 'phone', last_seen: '2026-08-14T09:00:00Z' },
    { device_id: 'tablet', last_seen: '2026-08-13T10:00:00Z' },
];

// Routes registered AFTER registerMocks win over its defaults and catch-all
// (Playwright matches routes in reverse registration order). The logs regex is
// anchored so it cannot swallow the /logs/devices endpoint.
async function setupLogsPage(page: Page, devicesStatus = 200) {
    await registerMocks(page, { authenticated: true, customProfiles: [profile] });
    await page.route(/\/api\/v1\/profiles\/prof1\/logs\/devices/i, route => {
        route.fulfill({
            status: devicesStatus,
            contentType: 'application/json',
            body: devicesStatus === 200 ? JSON.stringify(serverDevices) : '{}',
        });
    });
    const deviceParams: string[] = [];
    await page.route(/\/api\/v1\/profiles\/prof1\/logs(\?|$)/i, route => {
        const url = new URL(route.request().url());
        const deviceParam = url.searchParams.get('device_id');
        deviceParams.push(deviceParam ?? '');
        const rows = deviceParam
            ? [row(deviceParam, `${deviceParam}.example.test`)]
            : [row('laptop', 'one.example.test')];
        route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(rows) });
    });
    await page.goto('/query-logs');
    await page.getByTestId('logs-scroll-container').waitFor({ state: 'attached', timeout: 10000 });
    return { deviceParams };
}

const deviceTrigger = (page: Page) => page.locator('[aria-label="Filter by device"]:visible');

test.describe('Logs device filter', () => {
    test('dropdown lists all server devices and survives selecting one', async ({ page }) => {
        const { deviceParams } = await setupLogsPage(page);
        await expect(page.getByTestId('querylog-card-toggle')).toHaveCount(1);

        // Server-seeded list: rows only ever contained "laptop", yet all three
        // devices (incl. ones with no rows in the fetched window) are offered.
        await deviceTrigger(page).click();
        for (const name of ['All devices', 'laptop', 'phone', 'tablet']) {
            await expect(page.getByRole('option', { name })).toBeVisible();
        }
        await page.getByRole('option', { name: 'phone' }).click();

        // The logs refetch carries the device filter...
        await expect.poll(() => deviceParams.includes('phone')).toBe(true);
        await expect(page.getByTestId('querylog-card-toggle')).toHaveCount(1);

        // ...and reopening the dropdown still shows every device — the old
        // behavior collapsed it to just the selected one.
        await deviceTrigger(page).click();
        for (const name of ['All devices', 'laptop', 'phone', 'tablet']) {
            await expect(page.getByRole('option', { name })).toBeVisible();
        }
    });

    test('device endpoint failure degrades to row-observed ids', async ({ page }) => {
        await setupLogsPage(page, 500);
        await expect(page.getByTestId('querylog-card-toggle')).toHaveCount(1);

        await deviceTrigger(page).click();
        await expect(page.getByRole('option', { name: 'laptop' })).toBeVisible();
        await expect(page.getByRole('option', { name: 'phone' })).toHaveCount(0);
        // No error surface — silent degrade.
        await expect(page.getByTestId('logs-error')).toHaveCount(0);
    });
});
