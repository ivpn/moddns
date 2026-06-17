import { test, expect } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';
import { createMockProfiles } from '../../mocks/apiMocks';

// Repro for issue #118: on the blocklist Services page, switching the filter
// tabs (ALL / BLOCKED / UNBLOCKED) changes how many service cards render. The
// content column was sized to its content (a no-width child inside an
// `items-start` flex column), so when BLOCKED rendered few/zero cards the whole
// column — including the search bar — shrank, producing a visible layout shift.
// This is engine-specific (reproduces on Firefox/WebKit, not Chromium).
//
// The page content width must stay constant regardless of the active filter.

const SERVICES_ENDPOINT = /\/api\/v1\/services(\/?|\?.*)$/i;

function makeServices(n: number) {
  return Array.from({ length: n }, (_, i) => ({
    id: `svc-${i + 1}`,
    name: `Service ${i + 1}`,
    asns: [1000 + i],
  }));
}

test.describe('@layout services search bar layout stability', () => {
  test('search bar width is stable across ALL / BLOCKED / UNBLOCKED filters', async ({ page }) => {
    // No blocked services: clicking BLOCKED renders the empty state (fewest
    // cards) — the strongest trigger for the content column to collapse.
    const profiles = createMockProfiles(1, {}, ['bl-basic']);

    await registerMocks(page, { authenticated: true, customProfiles: profiles });
    await page.route(SERVICES_ENDPOINT, (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ services: makeServices(40) }),
      }),
    );

    // Width band where the bug manifests (content column is narrower than the
    // padded container, so a content-driven width is visible).
    await page.setViewportSize({ width: 1133, height: 900 });
    await page.goto('/blocklists');

    await page.getByRole('tab', { name: 'Services' }).click();

    const search = page.getByLabel('Search services');
    await expect(search).toBeVisible();

    const widthFor = async (filter: 'All' | 'Blocked' | 'Unblocked') => {
      await page.getByRole('button', { name: filter, exact: true }).click();
      await page.waitForTimeout(150); // let the grid re-render and layout settle
      const box = await search.boundingBox();
      return box!.width;
    };

    const all = await widthFor('All');
    const blocked = await widthFor('Blocked');
    const unblocked = await widthFor('Unblocked');

    // The page content (search bar) must not shift when the filter changes.
    expect(Math.abs(all - blocked)).toBeLessThanOrEqual(1);
    expect(Math.abs(all - unblocked)).toBeLessThanOrEqual(1);
  });
});
