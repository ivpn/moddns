import { test, expect } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';

(test.describe as typeof test.describe)('@layout @ios Logs iOS visibility', () => {
  test('renders logs page structure on iPhone', async ({ page }) => {
    test.skip(!/iphone15pro/i.test(test.info().project.name), 'Only run on iPhone project');

  await registerMocks(page, { authenticated: true, customProfiles: [{ id: 'prof1', profile_id: 'prof1', name: 'Default', settings: { custom_rules: [], logs: { enabled: true } } }], extraRoutes: async (p) => {
      await p.route('**/api/v1/profiles/prof1/logs*', route => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([]) }));
    } });

    await page.goto('/query-logs');

    await expect(page.getByText('Monitor and analyze DNS queries', { exact: false })).toBeVisible();
  await expect(page.locator('input[placeholder="Search domain or its part"]').first()).toBeVisible({ timeout: 5000 });
  });

  test('search placeholder fits the input on iPhone', async ({ page }) => {
    test.skip(!/iphone15pro/i.test(test.info().project.name), 'Only run on iPhone project');

  await registerMocks(page, { authenticated: true, customProfiles: [{ id: 'prof1', profile_id: 'prof1', name: 'Default', settings: { custom_rules: [], logs: { enabled: true } } }], extraRoutes: async (p) => {
      await p.route('**/api/v1/profiles/prof1/logs*', route => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([]) }));
    } });

    await page.goto('/query-logs');

    const input = page.locator('input[placeholder="Search domain or its part"]:visible').first();
    await expect(input).toBeVisible({ timeout: 5000 });

    // Empty input: the clear-button gutter is not reserved (the button only exists
    // once the placeholder is hidden), so the placeholder gets that width back.
    expect(await input.evaluate(el => parseFloat(getComputedStyle(el).paddingRight))).toBeLessThan(20);

    // The full placeholder must fit the content box at the iPhone viewport width.
    const { textWidth, contentWidth } = await input.evaluate(el => {
      const cs = getComputedStyle(el);
      const probe = document.createElement('span');
      probe.style.position = 'absolute';
      probe.style.visibility = 'hidden';
      probe.style.whiteSpace = 'nowrap';
      probe.style.fontSize = cs.fontSize;
      probe.style.fontFamily = cs.fontFamily;
      probe.style.fontWeight = cs.fontWeight;
      probe.style.letterSpacing = cs.letterSpacing;
      probe.textContent = (el as HTMLInputElement).placeholder;
      document.body.appendChild(probe);
      const measured = probe.getBoundingClientRect().width;
      probe.remove();
      return {
        textWidth: measured,
        contentWidth: el.clientWidth - parseFloat(cs.paddingLeft) - parseFloat(cs.paddingRight),
      };
    });
    expect(contentWidth).toBeGreaterThanOrEqual(textWidth);
  });
});
