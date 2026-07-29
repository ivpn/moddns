import { test, expect, type Page } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';
import { createMockBlocklists, createMockProfiles } from '../../mocks/apiMocks';

const BLOCKLISTS_ENDPOINT = /\/api\/v1\/blocklists(\/?|\?.*)$/i;
const PROFILE_ENDPOINT = /\/api\/v1\/profiles\/p1(\/?|\?.*)$/i;
const PROFILE_BLOCKLISTS_ENDPOINT = /\/api\/v1\/profiles\/p1\/blocklists(\/?|\?.*)$/i;

interface MutationLog {
  posts: string[][];
  deletes: string[][];
}

// Registers stateful routes on top of registerMocks (later routes take
// precedence). registerMocks' catch-all continues non-GET requests to the
// network and answers GET /profiles/p1 with the profiles *array*, so both
// must be overridden here for the enable/disable flow to round-trip.
async function registerStatefulRoutes(
  page: Page,
  blocklists: Record<string, unknown>[],
  enabledIds: Set<string>
): Promise<MutationLog> {
  const log: MutationLog = { posts: [], deletes: [] };

  await page.route(BLOCKLISTS_ENDPOINT, (route) =>
    route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(blocklists) })
  );

  await page.route(PROFILE_ENDPOINT, (route) => {
    if (route.request().method() !== 'GET') return route.fallback();
    const profile = createMockProfiles(1, {}, [...enabledIds])[0];
    return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(profile) });
  });

  await page.route(PROFILE_BLOCKLISTS_ENDPOINT, (route) => {
    const request = route.request();
    const method = request.method();
    if (method !== 'POST' && method !== 'DELETE') return route.fallback();
    const body = request.postDataJSON() ?? JSON.parse(request.postData() ?? '{}');
    const ids: string[] = body.blocklist_ids ?? [];
    if (method === 'POST') {
      log.posts.push(ids);
      ids.forEach((id) => enabledIds.add(id));
    } else {
      log.deletes.push(ids);
      ids.forEach((id) => enabledIds.delete(id));
    }
    return route.fulfill({ status: 200, contentType: 'application/json', body: '{}' });
  });

  return log;
}

function hagesiBlocklists(): Record<string, unknown>[] {
  return [
    ...createMockBlocklists(),
    {
      blocklist_id: 'bl-hagezi-2',
      name: 'Hagezi Extra',
      description: 'Second community curated list.',
      entries: 3456,
      last_modified: new Date().toISOString(),
      homepage: 'https://example.com/hagezi-2',
      tags: ['hagezi'],
    },
  ];
}

async function selectFilter(page: Page, optionName: string) {
  await page.getByLabel('Filter lists').filter({ visible: true }).click();
  await page.getByRole('option', { name: optionName }).click();
}

function toggleButton(page: Page) {
  return page.getByTestId('toggle-listed-blocklists').filter({ visible: true });
}

test.describe('@functional blocklists enable/disable all', () => {
  test('enables the not-yet-enabled filtered lists', async ({ page }) => {
    const enabledIds = new Set(['bl-hagezi']);
    await registerMocks(page, { authenticated: true, enableBlocklists: [...enabledIds] });
    const log = await registerStatefulRoutes(page, hagesiBlocklists(), enabledIds);

    await page.goto('/blocklists');
    await expect(page.getByTestId('blocklist-card').first()).toBeVisible();

    await selectFilter(page, 'Hagezi');
    const button = toggleButton(page);
    await expect(button).toBeEnabled();
    await expect(button).toHaveAttribute('aria-label', 'Enable listed blocklists');

    await button.click();

    await expect(page.getByText('Blocklists enabled').first()).toBeVisible();
    expect(log.posts).toEqual([['bl-hagezi-2']]);
    expect(log.deletes).toEqual([]);

    // After the profile refetch every filtered list is enabled, so the
    // button flips to its disable action.
    await expect(button).toHaveAttribute('aria-label', 'Disable listed blocklists');
  });

  test('disables all filtered lists when every one is enabled', async ({ page }) => {
    const enabledIds = new Set(['bl-hagezi', 'bl-hagezi-2']);
    await registerMocks(page, { authenticated: true, enableBlocklists: [...enabledIds] });
    const log = await registerStatefulRoutes(page, hagesiBlocklists(), enabledIds);

    await page.goto('/blocklists');
    await expect(page.getByTestId('blocklist-card').first()).toBeVisible();

    await selectFilter(page, 'Hagezi');
    const button = toggleButton(page);
    await expect(button).toBeEnabled();
    await expect(button).toHaveAttribute('aria-label', 'Disable listed blocklists');

    await button.click();

    await expect(page.getByText('Blocklists disabled').first()).toBeVisible();
    expect(log.deletes.map((ids) => [...ids].sort())).toEqual([['bl-hagezi', 'bl-hagezi-2']]);
    expect(log.posts).toEqual([]);

    await expect(button).toHaveAttribute('aria-label', 'Enable listed blocklists');
  });

  test('the Enabled status filter activates the button as disable-all', async ({ page }) => {
    const enabledIds = new Set(['bl-basic', 'bl-hagezi']);
    await registerMocks(page, { authenticated: true, enableBlocklists: [...enabledIds] });
    const log = await registerStatefulRoutes(page, hagesiBlocklists(), enabledIds);

    await page.goto('/blocklists');
    await expect(page.getByTestId('blocklist-card').first()).toBeVisible();

    await selectFilter(page, 'Enabled');
    const button = toggleButton(page);
    await expect(button).toBeEnabled();
    await expect(button).toHaveAttribute('aria-label', 'Disable listed blocklists');

    await button.click();

    await expect(page.getByText('Blocklists disabled').first()).toBeVisible();
    expect(log.deletes.map((ids) => [...ids].sort())).toEqual([['bl-basic', 'bl-hagezi']]);

    // The Enabled filter now matches nothing, so the button deactivates.
    await expect(page.getByTestId('blocklist-card')).toHaveCount(0);
    await expect(button).toBeDisabled();
  });
});
