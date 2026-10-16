import { test, expect } from '@playwright/test';
import { registerMocks, registerStatisticsMocks } from '../../mocks/registerMocks';
import { createStatsResponse } from '../../mocks/statisticsMocks';

const PROFILE = { id: 'p1', profile_id: 'p1', name: 'Default', settings: { logs: { enabled: false }, statistics: { enabled: true }, custom_rules: [] } };

// tableRef: statistics-behaviour #N3
test.describe('@layout bottom navigation lists Statistics', { tag: ['@ios', '@android'] }, () => {
  test.beforeEach(async ({ page }) => {
    await registerMocks(page, { authenticated: true, customProfiles: [PROFILE] });
    await registerStatisticsMocks(page, { statistics: createStatsResponse() });
  });

  test('shows Blocklists, Rules, Statistics, Logs and More, without Setup', async ({ page }) => {
    await page.goto('/blocklists');
    const nav = page.getByTestId('bottom-nav');
    await expect(nav.getByRole('button')).toHaveText(['Blocklists', 'Rules', 'Statistics', 'Logs', 'More']);
  });

  test('marks Statistics active on /statistics and navigates there', async ({ page }) => {
    await page.goto('/blocklists');
    await page.getByTestId('bottom-nav').getByRole('button', { name: 'Statistics' }).click();
    await expect(page).toHaveURL(/\/statistics$/);
    await expect(page.getByTestId('bottom-nav').getByRole('button', { name: 'Statistics' })).toHaveClass(/rdns-600/);
    await expect(page.getByRole('heading', { name: 'Statistics', level: 2 }).first()).toBeVisible();
  });

  test('keeps DNS Setup reachable as the first item under More', async ({ page }) => {
    await page.goto('/statistics');
    await page.getByTestId('bottom-nav').getByRole('button', { name: /more/i }).click();
    const overlay = page.getByTestId('overlay-navigation');
    await expect(overlay).toBeVisible();
    await expect(overlay.getByRole('button', { name: 'DNS Setup' })).toBeVisible();
    await overlay.getByRole('button', { name: 'DNS Setup' }).click();
    await expect(page).toHaveURL(/\/setup$/);
  });
});
