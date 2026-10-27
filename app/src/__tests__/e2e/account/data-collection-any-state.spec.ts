import { test, expect, type Page } from '@playwright/test';
import { registerMocks, registerStatisticsMocks } from '../../mocks/registerMocks';
import { createStatsResponse } from '../../mocks/statisticsMocks';

// Turning data collection off and deleting stored data work in every subscription state
// (statistics-behaviour L10, D5, S5; api-endpoint-behaviour G27).

function profile() {
  return {
    id: 'p1',
    profile_id: 'p1',
    name: 'Home',
    settings: {
      logs: { enabled: true, log_domains: true, log_clients_ips: false, retention: '1h' as const },
      statistics: { enabled: true, retention: '30d' as const, enabled_at: new Date(Date.now() - 20 * 86_400_000).toISOString() },
      privacy: { default_rule: 'allow' as const, blocklists_subdomains_rule: 'allow' as const, blocklists: [] },
      custom_rules: [],
    },
  };
}

/** Mocks the account in `status`; returns the PATCH bodies the page sends. */
async function setup(page: Page, status: string) {
  const p = profile();
  const patches: unknown[] = [];
  await registerMocks(page, { authenticated: true, customProfiles: [p] });
  await registerStatisticsMocks(page, { statistics: createStatsResponse({ points: 168 }) });
  await page.route(/\/api\/v1\/profiles(\?.*)?$/i, r => r.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([p]) }));
  await page.route(/\/api\/v1\/profiles\/p1$/i, async r => {
    if (r.request().method() === 'PATCH') {
      patches.push(r.request().postDataJSON());
      p.settings.statistics.enabled = false;
    }
    await r.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(p) });
  });
  await page.route(/\/api\/v1\/sub$/i, r =>
    r.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ status, active_until: '2026-01-01T00:00:00Z' }) }));
  return patches;
}

const statsOff = { updates: [{ operation: 'replace', path: '/settings/statistics/enabled', value: false }] };

test.describe('Data collection in every subscription state', { tag: ['@desktop', '@ios'] }, () => {
  // tableRef: statistics-behaviour #L10
  test('limited access: Settings turns statistics off with only the false path', async ({ page }) => {
    const patches = await setup(page, 'limited_access');
    await page.goto('/settings');
    const stats = page.getByRole('checkbox', { name: 'Statistics', exact: true });
    await stats.scrollIntoViewIfNeeded();
    await expect(page.getByText(/In limited access mode you can turn data collection off/)).toBeVisible();
    await expect(page.getByRole('radio', { name: '90 days' })).toBeDisabled();
    await stats.click();
    await page.getByRole('button', { name: 'Save changes' }).click();
    const dialog = page.getByRole('dialog', { name: 'Stop keeping statistics?' });
    await dialog.getByRole('button', { name: 'Delete statistics' }).click();
    await expect.poll(() => patches).toEqual([statsOff]);
  });

  // tableRef: statistics-behaviour #S5
  test('limited access: Delete statistics history from the Statistics menu', async ({ page }) => {
    await setup(page, 'limited_access');
    let deleted = false;
    await page.route(/\/api\/v1\/profiles\/p1\/statistics$/i, async r => {
      if (r.request().method() === 'DELETE') {
        deleted = true;
        return r.fulfill({ status: 204 });
      }
      return r.fallback();
    });
    await page.goto('/statistics');
    await page.getByRole('button', { name: 'More statistics actions' }).click();
    await page.getByRole('menuitem', { name: 'Delete statistics history' }).click();
    const dialog = page.getByRole('dialog', { name: 'Delete statistics history?' });
    await expect(dialog).toContainText('The counts go back to');
    await dialog.getByRole('button', { name: 'Delete history' }).click();
    await expect.poll(() => deleted).toBe(true);
  });

  // tableRef: statistics-behaviour #D5
  test('inactive: Account preferences turns statistics off', async ({ page }) => {
    const patches = await setup(page, 'inactive');
    await page.goto('/account-preferences');
    const region = page.getByRole('region', { name: 'Home' });
    await expect(region).toBeVisible();
    await expect(page.getByText("Your account can't change data collection now. You can still turn it off and delete what is stored.")).toBeVisible();
    await region.getByRole('checkbox', { name: 'Statistics', exact: true }).click();
    await region.getByRole('button', { name: 'Save changes' }).click();
    await page.getByRole('dialog', { name: 'Stop keeping statistics?' }).getByRole('button', { name: 'Delete statistics' }).click();
    await expect.poll(() => patches).toEqual([statsOff]);
    await expect(region.getByRole('button', { name: 'Clear query logs' })).toBeVisible();
  });
});
