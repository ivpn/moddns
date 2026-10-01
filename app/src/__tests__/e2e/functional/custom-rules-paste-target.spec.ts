import { test, expect } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';

// The browser only offers Copy/Paste in its context or long-press menu when the
// pointer lands on the <input> itself. The empty add-rule box must therefore keep
// its input stretched across the visible field rather than collapsing to the
// width of the (empty) typed text behind the placeholder.
test.describe('@functional custom rules add box exposes a pasteable input', () => {
  test('the empty add-rule field is hit by the input, not its wrapper', async ({ page }) => {
    await registerMocks(page, {
      authenticated: true,
      customProfiles: [{ id: 'prof1', profile_id: 'prof1', name: 'Default', settings: { custom_rules: [] } }],
    });
    await page.goto('/custom-rules');

    const input = page.locator('input#react-select-rule-composer-denylist-input');
    await expect(input).toBeAttached();

    const control = page.locator('.rule-composer__control').first();
    const placeholder = page.locator('.rule-composer__placeholder').first();
    await expect(placeholder).toBeVisible();
    // Centre the field so neither the sticky header nor the mobile bottom nav overlaps it.
    await control.evaluate((el) => el.scrollIntoView({ block: 'center' }));

    const box = await placeholder.boundingBox();
    const controlBox = await control.boundingBox();
    if (!box || !controlBox) throw new Error('add-rule field did not render');

    // Right-click / long-press where the placeholder text is drawn.
    const hit = await page.evaluate(
      ([x, y]) => { const el = document.elementFromPoint(x, y); return el ? el.tagName + '.' + el.className : null; },
      [box.x + box.width / 2, box.y + box.height / 2],
    );
    expect(hit).toMatch(/^INPUT\b/);

    // And the input spans the free space of the field, not a few pixels.
    const inputBox = await input.boundingBox();
    if (!inputBox) throw new Error('input has no box');
    expect(inputBox.width).toBeGreaterThan(controlBox.width * 0.5);
  });

  // Context menus are a desktop pointer path.
  test('a real right-click on the empty field reaches the input', { tag: '@desktop' }, async ({ page }) => {
    await registerMocks(page, {
      authenticated: true,
      customProfiles: [{ id: 'prof1', profile_id: 'prof1', name: 'Default', settings: { custom_rules: [] } }],
    });
    await page.goto('/custom-rules');

    const control = page.locator('.rule-composer__control').first();
    const placeholder = page.locator('.rule-composer__placeholder').first();
    await expect(placeholder).toBeVisible();
    await control.evaluate((el) => el.scrollIntoView({ block: 'center' }));

    // Record what the contextmenu event lands on and whether anything cancels it.
    await page.evaluate(() => {
      const w = window as Window & { __ctx?: { tag: string; id: string; prevented: boolean } };
      document.addEventListener(
        'contextmenu',
        (e) => {
          const t = e.target as HTMLElement;
          w.__ctx = { tag: t.tagName, id: t.id, prevented: e.defaultPrevented };
        },
        { capture: false },
      );
    });

    const box = await placeholder.boundingBox();
    if (!box) throw new Error('add-rule field did not render');
    await page.mouse.click(box.x + box.width / 2, box.y + box.height / 2, { button: 'right' });

    const ctx = await page.evaluate(() => (window as Window & { __ctx?: unknown }).__ctx);
    expect(ctx).toEqual({ tag: 'INPUT', id: 'react-select-rule-composer-denylist-input', prevented: false });
    await expect(page.locator('input#react-select-rule-composer-denylist-input')).toBeFocused();
  });
});
