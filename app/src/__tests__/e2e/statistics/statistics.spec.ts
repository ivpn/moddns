import { test, expect, type Page } from '@playwright/test';
import { registerMocks, registerStatisticsMocks } from '../../mocks/registerMocks';
import { createStatsResponse, devicesList, topBlocked, topBlocklists, topClients, topResolved } from '../../mocks/statisticsMocks';
import { expectNoHorizontalOverflow } from '../utils/layoutAssertions';

interface Cfg {
  logs: boolean;
  stats: boolean;
  retention?: string;
  statsRetention?: string;
  ips?: boolean;
}

const rangeTrigger = (page: Page) => page.getByRole('button', { name: /^Time range: / });

async function pickRange(page: Page, label: string) {
  await rangeTrigger(page).click();
  await page.getByRole('menuitemradio', { name: label, exact: true }).click();
}

function profileFor(c: Cfg) {
  return {
    id: 'p1',
    profile_id: 'p1',
    name: 'Default',
    settings: {
      logs: { enabled: c.logs, log_domains: true, log_clients_ips: c.ips ?? false, retention: c.retention ?? '1h' },
      statistics: { enabled: c.stats, retention: c.statsRetention ?? '30d' },
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
    await expect(rangeTrigger(page)).toHaveCount(0);
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
    await page.getByRole('checkbox', { name: 'Query logs', exact: true }).click();
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
  test('the range dropdown fits at 375px with 44px targets and follows the URL', { tag: '@mobile' }, async ({ page }) => {
    await page.setViewportSize({ width: 375, height: 812 });
    await setup(page, { logs: false, stats: true });
    await page.goto('/statistics?range=30d');
    const trigger = rangeTrigger(page);
    await expect(trigger).toHaveAccessibleName('Time range: Last 30 days');
    await expect(trigger).toHaveText('Last 30 days');
    const t = (await trigger.boundingBox())!;
    expect(t.height).toBeGreaterThanOrEqual(43.5);
    expect(t.width).toBeGreaterThan(250);
    const menuButton = (await page.getByRole('button', { name: 'More statistics actions' }).boundingBox())!;
    expect(menuButton.x).toBeGreaterThan(t.x + t.width);
    expect(menuButton.x + menuButton.width).toBeLessThanOrEqual(375);
    // Availability line first, the controls under it.
    const line = (await page.getByTestId('stats-availability').boundingBox())!;
    expect(line.y + line.height).toBeLessThanOrEqual(t.y);
    await expectNoHorizontalOverflow(page);

    await trigger.click();
    const items = page.getByRole('menuitemradio');
    await expect(items).toHaveCount(7);
    // offsetHeight ignores the menu's zoom-in transform while it opens.
    for (const b of await items.evaluateAll(els => els.map(e => ({ height: (e as HTMLElement).offsetHeight, right: e.getBoundingClientRect().right })))) {
      expect(b.height).toBeGreaterThanOrEqual(44);
      expect(b.right).toBeLessThanOrEqual(375);
    }
    await page.getByRole('menuitemradio', { name: 'Last 24 hours' }).click();
    await expect(page).toHaveURL(/range=24h$/);
    await pickRange(page, 'Last 7 days');
    await expect(page).toHaveURL(/\/statistics$/);
  });

  // tableRef: statistics-behaviour #K1, #K5, #P13
  test('the toolbar puts the availability line left and the range, menu and caption right', { tag: '@desktop' }, async ({ page }) => {
    await page.setViewportSize({ width: 1400, height: 1000 });
    await setup(page, { logs: false, stats: true });
    await page.goto('/statistics');
    const line = (await page.getByTestId('stats-availability').boundingBox())!;
    const trigger = (await rangeTrigger(page).boundingBox())!;
    const menu = (await page.getByRole('button', { name: 'More statistics actions' }).boundingBox())!;
    const caption = (await page.getByTestId('stats-range-caption').boundingBox())!;
    expect(line.x + line.width).toBeLessThanOrEqual(trigger.x);
    expect(menu.x).toBeGreaterThan(trigger.x + trigger.width);
    expect(caption.y).toBeGreaterThanOrEqual(trigger.y + trigger.height);
    expect(Math.abs(caption.x + caption.width - (menu.x + menu.width))).toBeLessThanOrEqual(2);
  });

  // tableRef: statistics-behaviour #P9, #K12
  test('P9: a young window keeps the full axis with the start marker, a dot, and a link to a shorter view', { tag: '@desktop' }, async ({ page }) => {
    await page.setViewportSize({ width: 1400, height: 1200 });
    await setup(page, { logs: false, stats: true }, createStatsResponse({ points: 168, enabledAt: '2026-10-06T14:20:00Z' }));
    await page.goto('/statistics');
    const panel = page.getByRole('region', { name: 'Queries over time' });
    await expect(panel.getByText(/^Counting since [^.]*\./)).toBeVisible();
    await expect(panel.getByText('Statistics on', { exact: true })).toBeVisible();
    await expect(panel.locator('.recharts-dot').first()).toBeVisible();
    expect(await panel.locator('.recharts-xAxis .recharts-cartesian-axis-tick').count()).toBeGreaterThanOrEqual(3);
    await panel.getByRole('button', { name: 'Show last 3 hours' }).click();
    await expect(page).toHaveURL(/range=3h$/);
    await expect(rangeTrigger(page)).toHaveAccessibleName('Time range: Last 3 hours');
  });

  // tableRef: statistics-behaviour #P25, #P26, #X10
  test('P25: a quick rule from a blocked-domain row opens the sheet, adds the rule and tags the row', async ({ page }) => {
    await setup(page, { logs: true, stats: true });
    let posted: unknown = null;
    await page.route(/\/api\/v1\/profiles\/p1\/custom_rules\/batch/i, r => {
      posted = r.request().postDataJSON();
      return r.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ created: [{ value: '*.ads.example.com' }], skipped: [] }) });
    });
    await page.goto('/statistics');
    const card = page.getByRole('region', { name: 'Top blocked domains' });
    const button = card.getByRole('button', { name: 'Create a custom rule for ads.example.com' });
    await expect(button).toBeVisible();
    await button.click();
    const sheet = page.getByRole('dialog', { name: 'Add custom rule' });
    await expect(sheet.getByRole('radio', { name: 'Allow domain' })).toHaveAttribute('data-state', 'on');
    await sheet.getByRole('button', { name: 'Add to Allowlist' }).click();
    await expect(page.getByText(/added to the Allowlist\./)).toBeVisible();
    await expect(page.getByText('Past queries still count here - new queries follow the rule.')).toBeVisible();
    await expect(sheet).toHaveCount(0);
    await expect(card.getByText('Rule added')).toBeVisible();
    await expect(button).toBeFocused();
    expect((posted as { action: string }).action).toBe('allow');
    await expectNoHorizontalOverflow(page);
    await page.getByRole('button', { name: 'View rules' }).click();
    await expect(page).toHaveURL(/\/custom-rules\?list=allowlist$/);
    await expect(page.getByRole('tab', { name: 'Allowlist' })).toHaveAttribute('data-state', 'active');
  });

  // tableRef: statistics-behaviour #X3, #X9
  test('X3: Queries over time switches between Line, Bars and Table; bars overlay blocked and keep the axis label', { tag: '@desktop' }, async ({ page }) => {
    await page.setViewportSize({ width: 1400, height: 1200 });
    await setup(page, { logs: false, stats: true });
    await page.goto('/statistics');
    const panel = page.getByRole('region', { name: 'Queries over time' });
    await expect(panel.getByRole('button', { name: 'Line' })).toHaveAttribute('aria-pressed', 'true');
    await expect(panel.getByTestId('bar-bucket')).toHaveCount(0);
    await panel.getByRole('button', { name: 'Bars' }).click();
    await expect(panel.getByTestId('bar-bucket').first()).toBeVisible();
    await expect(panel.getByTestId('bar-in-progress')).toHaveCount(1);
    const box = (await panel.locator('svg.recharts-surface').first().boundingBox())!;
    await page.mouse.move(box.x + box.width / 2, box.y + box.height - 8);
    await expect(page.getByTestId('axis-hover-label')).toHaveCount(1);
    await panel.getByRole('button', { name: 'Table' }).click();
    await expect(panel.getByRole('table')).toBeVisible();
    await expectNoHorizontalOverflow(page);
  });

  // tableRef: statistics-behaviour #K11, #K2
  test('K11: views the retention does not cover are disabled with the reason, and a hidden view in the URL is corrected', async ({ page }) => {
    await setup(page, { logs: false, stats: true });
    await page.goto('/statistics?range=12m');
    await expect(rangeTrigger(page)).toHaveAccessibleName('Time range: Last 30 days');
    await expect(page).toHaveURL(/range=30d$/);
    await rangeTrigger(page).click();
    await expect(page.getByRole('menuitemradio')).toHaveCount(7);
    const months = page.getByRole('menuitemradio', { name: /Last 3 months/ });
    await expect(months).toHaveAttribute('aria-disabled', 'true');
    await expect(months).toContainText('Needs statistics kept for 90 days');
    await expect(page.getByRole('menuitemradio', { name: /Last 12 months/ })).toContainText('Needs statistics kept for 1 year');
    const retentionItem = page.getByRole('menuitem', { name: 'Change retention…' });
    await expect(retentionItem).toHaveAttribute('aria-haspopup', 'dialog');
    await expect(retentionItem).toHaveCSS('color', 'rgb(18, 164, 149)');
    // Lines up with the view labels, which leave room for the selected-item dot.
    const viewPadding = await page.getByRole('menuitemradio', { name: 'Last 7 days' }).evaluate(e => getComputedStyle(e).paddingLeft);
    await expect(retentionItem).toHaveCSS('padding-left', viewPadding);
    await retentionItem.click();
    const dialog = page.getByRole('dialog', { name: 'Data collection' });
    await expect(dialog.getByRole('radiogroup', { name: 'Statistics Retention period' }).getByRole('radio', { name: '30 days' })).toBeFocused();
    await page.keyboard.press('Escape');
    await expect(dialog).toHaveCount(0);
    await expect(rangeTrigger(page)).toBeFocused();
  });

  // tableRef: statistics-behaviour #K11
  test('K11: a one-year retention enables all seven views and needs no Settings link', async ({ page }) => {
    await setup(page, { logs: false, stats: true, statsRetention: '1y' });
    await page.goto('/statistics?range=12m');
    await expect(rangeTrigger(page)).toHaveAccessibleName('Time range: Last 12 months');
    await rangeTrigger(page).click();
    await expect(page.getByRole('menuitemradio')).toHaveCount(7);
    await expect(page.locator('[role="menuitemradio"][aria-disabled="true"]')).toHaveCount(0);
    await expect(page.getByRole('menuitem', { name: 'Change retention…' })).toHaveCount(0);
    await expect(page).toHaveURL(/range=12m$/);
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

  // tableRef: statistics-behaviour #U7, #S2, #T14, #P13, #D4
  test('U7: Change opens the retention in a dialog; raising it enables the longer views in place', async ({ page }) => {
    await setup(page, { logs: false, stats: true });
    await page.goto('/statistics');
    const line = page.getByTestId('stats-availability');
    await expect(line).toContainText('kept for 30 days · Change');
    const change = line.getByRole('button', { name: 'Change how long statistics are kept' });
    await expect(change).toHaveCSS('color', 'rgb(18, 164, 149)');
    await expect(change).toHaveCSS('text-decoration-line', 'underline');
    await change.click();

    const dialog = page.getByRole('dialog', { name: 'Data collection' });
    const pills = dialog.getByRole('radiogroup', { name: 'Statistics Retention period' });
    await expect(pills.getByRole('radio', { name: '30 days' })).toBeFocused();
    await pills.getByRole('radio', { name: '1 year' }).click();
    await expect(dialog.getByText("modDNS will keep this profile's counts for up to 1 year.")).toBeVisible();
    await dialog.getByRole('button', { name: 'Keep for 1 year' }).click();
    await expect(dialog).toHaveCount(0);
    await expect(page).toHaveURL(/\/statistics$/);

    await pickRange(page, 'Last 12 months');
    await expect(page).toHaveURL(/range=12m$/);
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
