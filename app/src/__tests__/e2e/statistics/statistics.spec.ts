import { test, expect, type Page } from '@playwright/test';
import { registerMocks, registerStatisticsMocks } from '../../mocks/registerMocks';
import { createStatsResponse, devicesList, topBlocked, topBlocklists, topClients, topResolved } from '../../mocks/statisticsMocks';
import { expectNoHorizontalOverflow } from '../utils/layoutAssertions';

interface Cfg {
  logs: boolean;
  stats: boolean;
  retention?: string;
  ips?: boolean;
}

function profileFor(c: Cfg) {
  return {
    id: 'p1',
    profile_id: 'p1',
    name: 'Default',
    settings: {
      logs: { enabled: c.logs, log_domains: true, log_clients_ips: c.ips ?? false, retention: c.retention ?? '1h' },
      statistics: { enabled: c.stats, retention: '30d' },
      custom_rules: [],
    },
  };
}

// A stateful profile: PATCH applies the updates so in-app navigation keeps reading the saved state.
async function setup(page: Page, cfg: Cfg, stats: unknown = createStatsResponse({ points: 168 })) {
  const profile = profileFor(cfg);
  await registerMocks(page, { authenticated: true, customProfiles: [profile] });
  await registerStatisticsMocks(page, {
    statistics: stats,
    top: { blocked: topBlocked, resolved: topResolved },
    clients: topClients,
    blocklists: topBlocklists,
    devices: devicesList,
  });
  await page.route(/\/api\/v1\/profiles(\?.*)?$/i, r => r.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([profile]) }));
  await page.route(/\/api\/v1\/profiles\/p1$/i, async r => {
    if (r.request().method() === 'PATCH') {
      const { updates } = r.request().postDataJSON() as { updates: { path: string; value: unknown }[] };
      for (const u of updates) {
        const [, , group, key] = u.path.split('/');
        (profile.settings as unknown as Record<string, Record<string, unknown>>)[group][key] = u.value;
      }
    }
    await r.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(profile) });
  });
  return profile;
}

test.describe('@statistics Statistics page', () => {
  // tableRef: statistics-behaviour #U1
  test('U1: nothing stored shows one hero and no charts', async ({ page }) => {
    await setup(page, { logs: false, stats: false });
    await page.goto('/statistics');
    await expect(page.getByRole('heading', { name: 'Statistics are off' })).toBeVisible();
    await expect(page.getByRole('radiogroup', { name: 'Time range' })).toHaveCount(0);
    await expect(page.getByRole('heading', { name: 'Counts' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Turn on statistics' })).toBeEnabled();
    await expectNoHorizontalOverflow(page);
  });

  // tableRef: statistics-behaviour #U2
  test('U2: statistics only shows counts and one logs gate; the table view lists the series', async ({ page }) => {
    await setup(page, { logs: false, stats: true });
    await page.goto('/statistics');
    await expect(page.getByLabel(/^Total queries:/)).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Domains and clients come from query logs' })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Top blocked domains' })).toHaveCount(0);

    const series = page.getByRole('region', { name: 'Queries over time' });
    await series.getByRole('button', { name: 'Table' }).click();
    await expect(series.getByRole('table')).toBeVisible();
    await expect(series.getByRole('columnheader')).toHaveText(['Time', 'Total', 'Blocked', 'DNSSEC']);
    await expectNoHorizontalOverflow(page);
  });

  // tableRef: statistics-behaviour #U3
  test('U3: logs at one hour name their window, then Statistics only turns the logs group into one gate', async ({ page, isMobile }) => {
    test.skip(isMobile, 'settings flow is exercised on desktop');
    await setup(page, { logs: true, stats: true, retention: '1h', ips: true });
    await page.goto('/statistics');
    await expect(page.getByText('From the last 1 hour of query logs')).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Top blocked domains' })).toBeVisible();

    await page.getByTestId('main-navigation').getByRole('button', { name: 'Settings' }).click();
    await page.getByRole('radio', { name: 'Statistics', exact: true }).check({ force: true });
    await page.getByRole('button', { name: 'Save changes' }).click();
    await page.getByRole('dialog', { name: 'Turn off query logs?' }).getByRole('button', { name: 'Delete logs' }).click();
    await expect(page.getByText('No unsaved changes.')).toBeVisible();

    await page.getByTestId('main-navigation').getByRole('button', { name: 'Statistics' }).click();
    await expect(page.getByRole('heading', { name: 'Domains and clients come from query logs' })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Top blocked domains' })).toHaveCount(0);
  });

  // tableRef: statistics-behaviour #U6
  test('U6: logs only shows one counts gate and keeps the logs panels', async ({ page }) => {
    await setup(page, { logs: true, stats: false, ips: true });
    await page.goto('/statistics');
    await expect(page.getByRole('heading', { name: 'Counts are off' })).toBeVisible();
    await expect(page.getByRole('heading', { level: 3, name: /Queries over time|Blocked by reason|Protocols|Devices/ })).toHaveCount(0);
    await expect(page.getByRole('heading', { name: 'Top blocked domains' })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Top clients' })).toBeVisible();
    await expect(page.getByTestId('stats-availability')).toHaveCount(0);
  });

  // tableRef: statistics-behaviour #P22
  test('P22: Top blocklists shows the names from the response without a catalog request', async ({ page }) => {
    await setup(page, { logs: true, stats: false });
    const catalogRequests: string[] = [];
    page.on('request', r => {
      if (/\/api\/v1\/blocklists/i.test(new URL(r.url()).pathname)) catalogRequests.push(r.url());
    });
    await page.goto('/statistics');
    const panel = page.getByRole('region', { name: 'Top blocklists' });
    await expect(panel.getByText('Basic Protection')).toBeVisible();
    await expect(panel.getByText('bl-nameless')).toBeVisible();
    await expect(panel.getByText('A query blocked by several lists counts once for each.')).toBeVisible();
    await expect(panel.getByRole('row', { name: /Basic Protection/ })).toContainText('90');
    await panel.getByRole('button', { name: 'Chart' }).click();
    await expect(panel.getByText('Basic Protection')).toBeVisible();
    expect(catalogRequests).toEqual([]);
    await expectNoHorizontalOverflow(page);
  });

  // tableRef: statistics-behaviour #P5, #P24
  test('collecting: the first counts are announced with the expected time', async ({ page }) => {
    await page.clock.setFixedTime(new Date('2026-10-06T14:53:30Z'));
    const empty = createStatsResponse({ empty: true, points: 4, enabledAt: '2026-10-06T14:53:00Z' });
    await setup(page, { logs: false, stats: true }, empty);
    await page.goto('/statistics');
    await expect(page.getByRole('heading', { name: 'Collecting statistics' })).toBeVisible();
    await expect(page.getByText(/so the first counts appear at about .*\. This page updates by itself\./)).toBeVisible();
    await expect(page.getByRole('link', { name: 'Check your device setup' })).toBeVisible();
  });

  // tableRef: statistics-behaviour #P23
  test('no queries yet: one card with the setup link', async ({ page }) => {
    const empty = createStatsResponse({ empty: true, points: 4, enabledAt: '2026-10-06T14:50:00Z' });
    await setup(page, { logs: false, stats: true }, empty);
    await page.goto('/statistics');
    await expect(page.getByRole('heading', { name: 'No queries counted yet' })).toBeVisible();
    await expect(page.getByText(/No queries have reached this profile since .*\. If your devices should be using it, check the device setup\./)).toBeVisible();
    await page.getByRole('link', { name: 'Check your device setup' }).click();
    await expect(page).toHaveURL(/\/setup$/);
  });

  // tableRef: statistics-behaviour #K1, #K2
  test('the picker fits at 375px with 44px segments and follows the URL', { tag: '@mobile' }, async ({ page }) => {
    await page.setViewportSize({ width: 375, height: 812 });
    await setup(page, { logs: false, stats: true });
    await page.goto('/statistics?range=30d');
    const group = page.getByRole('radiogroup', { name: 'Time range' });
    await expect(group).toBeVisible();
    await expect(group.getByRole('radio', { name: 'Last 30 days' })).toBeChecked();
    const boxes = await group.getByRole('radio').evaluateAll(els => els.map(e => e.getBoundingClientRect()));
    expect(boxes).toHaveLength(7);
    for (const b of boxes) {
      expect(b.width).toBeGreaterThanOrEqual(43.5);
      expect(b.height).toBeGreaterThanOrEqual(43.5);
      expect(b.right).toBeLessThanOrEqual(375);
    }
    await expectNoHorizontalOverflow(page);
    await group.getByText('24h').click();
    await expect(page).toHaveURL(/range=24h$/);
    await group.getByText('7d').click();
    await expect(page).toHaveURL(/\/statistics$/);
  });
  // tableRef: statistics-behaviour #K2
  test('the retention select stretches across the width on mobile with the menu button at the end', { tag: '@mobile' }, async ({ page }) => {
    await page.setViewportSize({ width: 375, height: 812 });
    await setup(page, { logs: false, stats: true });
    await page.goto('/statistics');
    const select = page.getByLabel('Kept for');
    const menu = page.getByRole('button', { name: 'More statistics actions' });
    await expect(select).toBeVisible();
    const [s, m] = await Promise.all([select.boundingBox(), menu.boundingBox()]);
    expect(s!.height).toBeGreaterThanOrEqual(43.5);
    expect(s!.width).toBeGreaterThan(200);
    expect(m!.x).toBeGreaterThan(s!.x + s!.width);
    expect(m!.x + m!.width).toBeGreaterThan(375 - 40);
    await expectNoHorizontalOverflow(page);
  });

  // tableRef: statistics-behaviour #X9
  test('X9: hovering the time axis shows the bucket label on the axis', { tag: '@desktop' }, async ({ page }) => {
    await page.setViewportSize({ width: 1400, height: 1200 });
    await setup(page, { logs: false, stats: true });
    await page.goto('/statistics');
    const panel = page.getByRole('region', { name: 'Queries over time' });
    const svg = panel.locator('svg.recharts-surface').first();
    await expect(svg).toBeVisible();
    const box = (await svg.boundingBox())!;
    await expect(page.getByTestId('axis-hover-label')).toHaveCount(0);
    await page.mouse.move(box.x + box.width / 2, box.y + box.height - 8);
    const label = page.getByTestId('axis-hover-label');
    await expect(label).toHaveCount(1);
    await expect(label).toContainText(/^[A-Z][a-z]{2} \d{1,2}, .+-.+$/);
    await expect(label).toHaveAttribute('aria-hidden', 'true');
    await page.mouse.move(box.x + box.width / 2, box.y - 40);
    await expect(label).toHaveCount(0);
  });

  // tableRef: statistics-behaviour #U7, #S1
  test('U7: raising retention asks for confirmation and keeps the choice', async ({ page }) => {
    await setup(page, { logs: false, stats: true });
    await page.goto('/statistics');
    const select = page.getByLabel('Kept for');
    await expect(select).toHaveValue('30d');
    await select.selectOption('1y');
    const dialog = page.getByRole('dialog', { name: 'Keep statistics for 1 year?' });
    await expect(dialog.getByRole('button', { name: 'Cancel' })).toBeFocused();
    await dialog.getByRole('button', { name: 'Keep for 1 year' }).click();
    await expect(dialog).toHaveCount(0);
    await expect(select).toHaveValue('1y');
    await expectNoHorizontalOverflow(page);
  });

  // tableRef: statistics-behaviour #S3, #P5
  test('Delete statistics history empties the counts and lands on the no-queries-yet card', async ({ page }) => {
    await setup(page, { logs: false, stats: true });
    let deleted = false;
    const empty = createStatsResponse({ empty: true, points: 4, enabledAt: '2026-09-14T09:12:00Z' });
    await page.route(/\/api\/v1\/profiles\/p1\/statistics/i, r => {
      if (r.request().method() === 'DELETE') {
        deleted = true;
        return r.fulfill({ status: 204 });
      }
      return r.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(deleted ? { ...empty, history_deleted_at: '2026-10-06T15:10:00Z' } : createStatsResponse({ points: 168 })) });
    });
    await page.goto('/statistics');
    await expect(page.getByLabel(/^Total queries:/)).toBeVisible();
    await page.getByRole('button', { name: 'More statistics actions' }).click();
    await page.getByRole('menuitem', { name: 'Delete statistics history' }).click();
    await page.getByRole('dialog', { name: 'Delete statistics history?' }).getByRole('button', { name: 'Delete history' }).click();
    await expect(page.getByRole('heading', { name: 'No queries counted yet' })).toBeFocused();
  });
});
