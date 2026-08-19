import { test, expect, type Page } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';

// Refresh controls on the Query Logs page: the icon button is a one-shot refresh and
// the labeled "Auto" toggle owns the 10s loop. Interval/pill mechanics live in unit
// tests (QueryLogs.test.tsx); here we pin the wire-level behavior and accessibility.

const profile = { id: 'prof1', profile_id: 'prof1', name: 'Default', settings: { logs: { enabled: true } } };

const logItem = (i: number, domain: string) => ({
    profile_id: 'prof1',
    timestamp: `2026-08-12T10:00:${(59 - i).toString().padStart(2, '0')}Z`,
    status: 'processed',
    protocol: 'dns',
    device_id: `device-${i}`,
    client_ip: `10.0.0.${i}`,
    dns_request: { domain, query_type: 'A' },
});

// Register the logs route AFTER registerMocks so it is tested BEFORE the catch-all
// route (Playwright matches routes in reverse registration order). The catch-all in
// registerMocks matches `/api/v1/profiles` and would otherwise shadow this endpoint.
// `respond` is re-evaluated per request (never keyed on call count — StrictMode
// double-fires the mount fetch in dev, so call indices are not deterministic).
async function setupLogsPage(page: Page, respond: () => object[]) {
    await registerMocks(page, { authenticated: true, customProfiles: [profile] });
    let calls = 0;
    await page.route(/\/api\/v1\/profiles\/prof1\/logs(\?|$)/i, route => {
        calls++;
        route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(respond()) });
    });
    await page.goto('/query-logs');
    await page.getByTestId('logs-scroll-container').waitFor({ state: 'attached', timeout: 10000 });
    return { logsCalls: () => calls };
}

// Two instances render (mobile row / desktop row); only one is visible per breakpoint.
const visibleControl = (page: Page, testId: string) =>
    page.locator(`[data-testid="${testId}"]:visible`);

test.describe('Logs refresh controls', () => {
    test('split button halves stay equal height at tablet width', async ({ page }) => {
        // The Button default size carries sm:h-9, which silently shrinks the interval
        // trigger below the icon half on 640-1024px viewports unless pinned.
        await page.setViewportSize({ width: 834, height: 1112 });
        await setupLogsPage(page, () => [logItem(1, 'one.example.test')]);
        const refresh = await visibleControl(page, 'logs-refresh-button').boundingBox();
        const trigger = await visibleControl(page, 'logs-refresh-interval-trigger').boundingBox();
        expect(refresh && trigger && refresh.height === trigger.height && refresh.y === trigger.y).toBe(true);
    });

    test('refresh button is a one-shot refresh with an accessible name', async ({ page }) => {
        const { logsCalls } = await setupLogsPage(page, () => [logItem(1, 'one.example.test')]);
        const initialCalls = logsCalls();

        const refresh = visibleControl(page, 'logs-refresh-button');
        await expect(refresh).toHaveAccessibleName('Refresh query logs');

        // Desktop-only freshness label appears beside the controls after the first load.
        if (test.info().project.name === 'chromium-desktop') {
            await expect(page.getByTestId('logs-freshness')).toHaveText(/Updated (just now|\d+s ago)/);
        }

        await refresh.click();
        await expect.poll(logsCalls).toBe(initialCalls + 1);

        // The click is acknowledged with at least one full rotation even when the
        // response lands instantly...
        await expect(refresh.locator('svg')).toHaveClass(/animate-spin/);
        // ...then the spin stops: one-shot means no lingering animation and no
        // auto-refresh mode (no interval label on the split button).
        await expect(refresh.locator('svg')).not.toHaveClass(/animate-spin|animate-\[/, { timeout: 3000 });
        await expect(page.getByTestId('logs-refresh-interval-label')).toHaveCount(0);
        expect(logsCalls()).toBe(initialCalls + 1);
    });

    test('interval menu enables live mode and stages new entries behind the pill', async ({ page }) => {
        const initial = [logItem(1, 'one.example.test'), logItem(2, 'two.example.test')];
        const fresh = logItem(0, 'fresh.example.test');
        let dataset = initial;
        await setupLogsPage(page, () => dataset);

        await expect(page.getByTestId('querylog-card-toggle')).toHaveCount(2);
        // From here on, one new entry sits above the known head — the immediate tick
        // fired by enabling auto-refresh should stage it, not apply it.
        dataset = [fresh, ...initial];

        const intervalTrigger = visibleControl(page, 'logs-refresh-interval-trigger');
        await expect(intervalTrigger).toHaveAccessibleName('Auto-refresh interval');

        // Desktop: freshness sits on its own line ABOVE the controls, so the button
        // widening in live mode must not move it (the old inline placement jittered).
        const isDesktop = test.info().project.name === 'chromium-desktop';
        let freshnessBefore: { x: number; y: number } | null = null;
        if (isDesktop) {
            const box = await page.getByTestId('logs-freshness').boundingBox();
            const triggerBox = await intervalTrigger.boundingBox();
            expect(box && triggerBox && box.y + box.height <= triggerBox.y + 1).toBe(true);
            freshnessBefore = box && { x: box.x, y: box.y };
        }

        // The menu offers the full interval set.
        await intervalTrigger.click();
        for (const key of ['off', 'auto', '5s', '10s', '15s', '30s', '60s']) {
            await expect(page.getByTestId(`logs-refresh-interval-${key}`)).toBeVisible();
        }
        // Desktop: the menu opens down-right (extends past the trigger's right edge,
        // into the page margin) so it doesn't drop over the quick-rule column at the
        // right edge of the cards below. Radix may clamp the exact left edge to keep
        // the menu inside the viewport, so assert the direction, not exact alignment.
        if (test.info().project.name === 'chromium-desktop') {
            const menuBox = await page.getByTestId('logs-refresh-interval-auto').boundingBox();
            const triggerBox = await intervalTrigger.boundingBox();
            expect(menuBox && triggerBox && menuBox.x + menuBox.width > triggerBox.x + triggerBox.width + 10).toBe(true);
        }
        await page.getByTestId('logs-refresh-interval-auto').click();

        // Live cues: compact label on the split button + continuously spinning icon.
        await expect(visibleControl(page, 'logs-refresh-interval-label')).toHaveText('Auto');
        if (isDesktop && freshnessBefore) {
            const after = (await page.getByTestId('logs-freshness').boundingBox())!;
            expect(Math.abs(after.x - freshnessBefore.x)).toBeLessThanOrEqual(1);
            expect(Math.abs(after.y - freshnessBefore.y)).toBeLessThanOrEqual(1);
        }
        await expect(visibleControl(page, 'logs-refresh-button').locator('svg')).toHaveClass(/animate-\[spin_3s_linear_infinite\]/);

        // The immediate tick stages the new entry — the list itself must not change.
        const pill = page.getByTestId('logs-new-queries-pill');
        await expect(pill).toBeVisible();
        await expect(pill).toHaveText(/1 new query/);
        await expect(page.getByTestId('querylog-card-toggle')).toHaveCount(2);
        // Clickability affordance: pointer cursor on hover.
        expect(await pill.evaluate(el => getComputedStyle(el).cursor)).toBe('pointer');

        // Revealing prepends without a reload.
        await pill.click();
        await expect(page.getByTestId('querylog-card-toggle')).toHaveCount(3);
        await expect(pill).toHaveCount(0);

        // Off stops live mode: label disappears, icon stops spinning.
        await intervalTrigger.click();
        await page.getByTestId('logs-refresh-interval-off').click();
        await expect(page.getByTestId('logs-refresh-interval-label')).toHaveCount(0);
        await expect(visibleControl(page, 'logs-refresh-button').locator('svg')).not.toHaveClass(/animate-spin|animate-\[/);
    });
});
