import { test, expect } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';
import { createMockBlocklists } from '../../mocks/apiMocks';
import { ApiV1BlocklistsGetSortByEnum } from '@/api/client/api';

const BLOCKLISTS_ENDPOINT = /\/api\/v1\/blocklists(\/?|\?.*)$/i;

test.describe('@functional blocklists sorting', () => {
  test('requests sorted blocklists when the sort option changes', async ({ page }) => {
    await registerMocks(page, { authenticated: true });

    const baseBlocklists = createMockBlocklists();
    const entriesSorted = baseBlocklists.slice(0, 2).map((blocklist, index) => ({
      ...blocklist,
      blocklist_id: `entries-${index}`,
      name: `Entries Variant ${index + 1}`,
      entries: 999999 - index * 5000,
    }));
    const nameSorted = [...baseBlocklists].sort((a, b) => (a.name || '').localeCompare(b.name || ''));

    const requestLog: string[] = [];

    await page.route(BLOCKLISTS_ENDPOINT, async (route) => {
      const url = new URL(route.request().url());
      const sort = url.searchParams.get('sort_by') ?? 'missing';
      requestLog.push(sort);
      const payload = sort === ApiV1BlocklistsGetSortByEnum.Entries
        ? entriesSorted
        : sort === ApiV1BlocklistsGetSortByEnum.Name
          ? nameSorted
          : baseBlocklists;
      await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(payload) });
    });

    await page.goto('/blocklists');

    const cards = page.getByTestId('blocklist-card');
    await expect(cards.first()).toBeVisible();

    await expect.poll(() => requestLog.length).toBeGreaterThanOrEqual(1);
    expect(requestLog[0]).toBe(ApiV1BlocklistsGetSortByEnum.Updated);

    await page.getByLabel('Sort blocklists').click();
    await page.getByRole('option', { name: 'Most entries' }).click();

    await expect.poll(() => requestLog.length).toBeGreaterThanOrEqual(2);
    expect(requestLog[requestLog.length - 1]).toBe(ApiV1BlocklistsGetSortByEnum.Entries);

    await expect(cards.first()).toContainText('Entries Variant 1');
  });
});
