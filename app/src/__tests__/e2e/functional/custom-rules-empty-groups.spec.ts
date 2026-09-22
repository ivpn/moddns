import { test, expect, type Route } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';

// Issue #196: group folders live in the profile's group registry, independently of
// the rules that point at them. The list must keep rendering those folders when the
// rule list is empty — including right after the last rule is deleted — instead of
// collapsing to the "no rules yet" empty state.
const PROFILE_ENDPOINT = /\/api\/v1\/profiles\/prof1(\/?|\?.*)$/i;
const RULE_DELETE_ENDPOINT = /\/api\/v1\/profiles\/prof1\/custom_rules\/r1(\/?|\?.*)$/i;

const groups = { block: [{ name: 'Work', comment: '' }, { name: 'Ads', comment: '' }] };

const withoutRules = {
  id: 'prof1', profile_id: 'prof1', name: 'Default',
  settings: { custom_rule_groups: groups, custom_rules: [] },
};

const withOneRule = {
  ...withoutRules,
  settings: {
    ...withoutRules.settings,
    custom_rules: [{ id: 'r1', action: 'block', value: 'work.example.com', group: 'Work', order: 0 }],
  },
};

test.describe('@functional custom rules keep empty groups visible', () => {
  // eslint-disable-next-line no-empty-pattern
  test.beforeEach(({}, testInfo) => {
    test.skip(!/chromium-desktop/i.test(testInfo.project.name), 'group folders are exercised on desktop');
  });

  test('renders registry groups when the profile has no custom rules', async ({ page }) => {
    await registerMocks(page, { authenticated: true, customProfiles: [withoutRules] });
    await page.goto('/custom-rules');

    await expect(page.getByRole('button', { name: 'Drag to reorder group Work' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Drag to reorder group Ads' })).toBeVisible();
    await expect(page.getByText('There are no denied domains yet.')).toHaveCount(0);
  });

  test('deleting the last rule keeps its group folder on screen', async ({ page }) => {
    await registerMocks(page, { authenticated: true, customProfiles: [withOneRule] });

    // Registered after registerMocks so these take precedence over the catch-all.
    // The single-profile GET flips to the rule-less payload once the DELETE has landed,
    // mirroring the server state the page refetches after a delete.
    let deleted = false;
    await page.route(RULE_DELETE_ENDPOINT, (r: Route) => {
      if (r.request().method() !== 'DELETE') return r.fallback();
      deleted = true;
      return r.fulfill({ status: 200, contentType: 'application/json', body: '' });
    });
    await page.route(PROFILE_ENDPOINT, (r: Route) => {
      if (r.request().method() !== 'GET') return r.fallback();
      return r.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify(deleted ? withoutRules : withOneRule),
      });
    });

    await page.goto('/custom-rules');
    await expect(page.getByText('work.example.com')).toBeVisible();

    await page.getByRole('button', { name: 'Delete rule' }).click();

    await expect.poll(() => deleted).toBe(true);
    await expect(page.getByText('work.example.com')).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Drag to reorder group Work' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Drag to reorder group Ads' })).toBeVisible();
    await expect(page.getByText('There are no denied domains yet.')).toHaveCount(0);
  });
});
