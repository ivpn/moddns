import { test, expect, type Route } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';

// Reorder-groups feature (issue #634 follow-up): group sections render in the stored
// registry order (not alphabetical) and can be dragged into a new order, which PATCHes
// /custom_rule_groups/order with the new name list.
const ORDER_ENDPOINT = /\/api\/v1\/profiles\/[^/]+\/custom_rule_groups\/order(\/?|\?.*)$/i;

const profile = {
  id: 'prof1', profile_id: 'prof1', name: 'Default',
  settings: {
    // Registry order is deliberately NON-alphabetical (Work before Ads) so a stray
    // alphabetical sort would be visible.
    custom_rule_groups: { block: [{ name: 'Work', comment: '' }, { name: 'Ads', comment: '' }] },
    custom_rules: [
      { id: 'r1', action: 'block', value: 'work.example.com', group: 'Work', order: 0 },
      { id: 'r2', action: 'block', value: 'ads.example.com', group: 'Ads', order: 1 },
    ],
  },
};

test.describe('@functional custom rules group reorder', () => {
  // eslint-disable-next-line no-empty-pattern
  test.beforeEach(({}, testInfo) => {
    test.skip(!/chromium-desktop/i.test(testInfo.project.name), 'pointer-drag reorder is exercised on desktop');
  });

  test('renders groups in registry order (not alphabetical)', async ({ page }) => {
    await registerMocks(page, { authenticated: true, customProfiles: [profile] });
    await page.goto('/custom-rules');

    const work = await page.getByRole('button', { name: 'Drag to reorder group Work' }).boundingBox();
    const ads = await page.getByRole('button', { name: 'Drag to reorder group Ads' }).boundingBox();
    expect(work).not.toBeNull();
    expect(ads).not.toBeNull();
    // "Work" is listed before "Ads" — registry order, not A→Z.
    expect(work!.y).toBeLessThan(ads!.y);
  });

  test('dragging a group PATCHes the new order', async ({ page }) => {
    await registerMocks(page, { authenticated: true, customProfiles: [profile] });

    // Registered after registerMocks so this handler takes precedence over the catch-all.
    let orderBody: { action?: string; order?: string[] } | null = null;
    await page.route(ORDER_ENDPOINT, (r: Route) => {
      if (r.request().method() === 'PATCH') {
        orderBody = r.request().postDataJSON();
        return r.fulfill({ status: 200, contentType: 'application/json', body: '' });
      }
      return r.fallback();
    });

    await page.goto('/custom-rules');

    // dnd-kit PointerSensor drag: grab the "Ads" group grip and drop it onto the "Work"
    // header above it. The sensor needs a small move to clear its 5px activation
    // threshold before gliding to the target, with a beat for each rAF.
    const source = page.getByRole('button', { name: 'Drag to reorder group Ads' });
    const target = page.getByRole('button', { name: 'Drag to reorder group Work' });
    await source.hover();
    const s = (await source.boundingBox())!;
    const t = (await target.boundingBox())!;
    const sx = s.x + s.width / 2;
    const sy = s.y + s.height / 2;

    await page.mouse.move(sx, sy);
    await page.mouse.down();
    await page.mouse.move(sx, sy + 6); // clear the activation threshold
    await page.waitForTimeout(120);
    await page.mouse.move(t.x + t.width / 2, t.y + t.height / 2, { steps: 12 });
    await page.waitForTimeout(120);
    await page.mouse.up();

    await expect.poll(() => orderBody).not.toBeNull();
    expect(orderBody!.action).toBe('block');
    expect(orderBody!.order).toEqual(['Ads', 'Work']);
  });
});
