import { test, expect, type Page } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';
import { createMockBlocklists, createMockProfiles } from '../../mocks/apiMocks';

const BLOCKLISTS_ENDPOINT = /\/api\/v1\/blocklists(\/?|\?.*)$/i;
const PROFILE_ENDPOINT = /\/api\/v1\/profiles\/p1(\/?|\?.*)$/i;
const PROFILE_BLOCKLISTS_ENDPOINT = /\/api\/v1\/profiles\/p1\/blocklists(\/?|\?.*)$/i;

const NATIVE_IDS = ['hagezi_native_apple', 'hagezi_native_winoffice', 'hagezi_native_samsung'];

interface MutationLog {
  posts: string[][];
  deletes: string[][];
}

// Same stateful overrides as blocklists-enable-all.spec.ts: later routes take
// precedence over registerMocks, and GET /profiles/p1 must return one profile.
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

function nativeList(id: string, name: string, description: string) {
  return {
    blocklist_id: id,
    name,
    description,
    entries: 200,
    last_modified: new Date().toISOString(),
    homepage: 'https://github.com/hagezi/dns-blocklists',
    kind: 'general',
    intensity: [],
    tags: ['hagezi', 'native'],
  };
}

function blocklistsWithNative(): Record<string, unknown>[] {
  return [
    ...createMockBlocklists(),
    nativeList('hagezi_native_apple', 'Apple', 'Blocks native trackers built into iOS, macOS and tvOS.'),
    nativeList('hagezi_native_winoffice', 'Microsoft', 'Blocks native trackers built into Windows, Office and MSN.'),
    nativeList('hagezi_native_samsung', 'Samsung', 'Blocks native trackers built into Samsung devices.'),
  ];
}

async function setup(page: Page, enabled: string[] = ['bl-basic']) {
  const enabledIds = new Set(enabled);
  await registerMocks(page, { authenticated: true, enableBlocklists: [...enabledIds] });
  const log = await registerStatefulRoutes(page, blocklistsWithNative(), enabledIds);
  await page.goto('/blocklists');
  await expect(page.getByTestId('blocklist-card').first()).toBeVisible();
  return log;
}

async function selectFilter(page: Page, optionName: string) {
  await page.getByLabel('Filter lists').filter({ visible: true }).click();
  await page.getByRole('option', { name: optionName }).click();
}

function nativeCard(page: Page) {
  return page.getByTestId('native-trackers-group').getByTestId('category-card');
}

function panelCards(page: Page) {
  return page.getByTestId('native-trackers-group').getByTestId('blocklist-card');
}

test.describe('@functional blocklists native trackers group', () => {
  test('renders one group card after the regular lists, not individual cards', async ({ page }) => {
    await setup(page);

    await expect(nativeCard(page)).toBeVisible();
    await expect(nativeCard(page)).toContainText('Hagezi Native Trackers');
    await expect(nativeCard(page)).toContainText('3 lists');
    // Only the four regular mock lists render as top-level cards.
    await expect(page.getByTestId('blocklist-card')).toHaveCount(4);
    await expect(panelCards(page)).toHaveCount(0);

    const lastRegular = await page.getByTestId('blocklist-card').last().boundingBox();
    const group = await nativeCard(page).boundingBox();
    expect(lastRegular && group).toBeTruthy();
    expect(group!.y > lastRegular!.y || (group!.y === lastRegular!.y && group!.x > lastRegular!.x)).toBe(true);
  });

  test('shows under All and Hagezi, hides under intensity and other providers', async ({ page }) => {
    await setup(page);

    await selectFilter(page, 'Hagezi');
    await expect(nativeCard(page)).toBeVisible();

    await selectFilter(page, 'Basic');
    await expect(nativeCard(page)).toHaveCount(0);

    await selectFilter(page, 'OISD');
    await expect(nativeCard(page)).toHaveCount(0);

    await selectFilter(page, 'All lists');
    await expect(nativeCard(page)).toBeVisible();
  });

  test('expands to per-vendor cards and the badge tracks enabled vendors', async ({ page }) => {
    const log = await setup(page);

    await nativeCard(page).getByRole('button', { name: /3 lists/ }).click();
    await expect(panelCards(page)).toHaveCount(3);
    await expect(nativeCard(page)).not.toContainText('/3');
    for (const vendor of ['Apple', 'Microsoft', 'Samsung']) {
      await expect(
        panelCards(page).filter({ hasText: vendor }).getByRole('img', { name: `${vendor} logo` })
      ).toBeVisible();
    }

    await panelCards(page).filter({ hasText: 'Microsoft' }).getByRole('switch').click();
    await expect(nativeCard(page)).toContainText('1/3');
    expect(log.posts).toEqual([['hagezi_native_winoffice']]);
  });

  test('card switch enables every vendor, then disables them all', async ({ page }) => {
    const log = await setup(page);

    await nativeCard(page).getByRole('switch').click();
    await expect(page.getByText('Hagezi Native Trackers enabled').first()).toBeVisible();
    await expect(nativeCard(page)).toContainText('3/3');
    expect(log.posts.map((ids) => [...ids].sort())).toEqual([[...NATIVE_IDS].sort()]);

    await nativeCard(page).getByRole('switch').click();
    await expect(page.getByText('Hagezi Native Trackers disabled').first()).toBeVisible();
    await expect(nativeCard(page)).not.toContainText('/3');
    expect(log.deletes.map((ids) => [...ids].sort())).toEqual([[...NATIVE_IDS].sort()]);
  });

  test('the Hagezi bulk toggle leaves native lists alone', async ({ page }) => {
    const log = await setup(page);

    await selectFilter(page, 'Hagezi');
    const button = page.getByTestId('toggle-listed-blocklists').filter({ visible: true });
    await expect(button).toHaveAttribute('aria-label', 'Enable listed blocklists');
    await button.click();

    await expect(page.getByText('Blocklists enabled').first()).toBeVisible();
    expect(log.posts).toEqual([['bl-hagezi']]);
    await expect(nativeCard(page)).not.toContainText('/3');
  });

  test('searching a vendor opens the group narrowed to that vendor', async ({ page }) => {
    await setup(page);

    await page.getByPlaceholder(/search/i).filter({ visible: true }).fill('samsung');
    await expect(nativeCard(page)).toBeVisible();
    await expect(panelCards(page)).toHaveCount(1);
    await expect(panelCards(page).first()).toContainText('Samsung');
    await expect(page.getByTestId('blocklist-card')).toHaveCount(1);
  });

  test('the Enabled status filter shows the group only when a vendor is on', async ({ page }) => {
    await setup(page, ['bl-basic']);
    await selectFilter(page, 'Enabled');
    await expect(nativeCard(page)).toHaveCount(0);
  });

  test('the Enabled status filter keeps the group when a vendor is on', async ({ page }) => {
    await setup(page, ['bl-basic', 'hagezi_native_apple']);
    await selectFilter(page, 'Enabled');
    await expect(nativeCard(page)).toBeVisible();
    await expect(nativeCard(page)).toContainText('1/3');
  });
});
