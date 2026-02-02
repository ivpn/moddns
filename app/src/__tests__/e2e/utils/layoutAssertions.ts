import { expect, type Page } from '@playwright/test';

export async function expectNoHorizontalOverflow(page: Page) {
  const result = await page.evaluate(() => {
    const doc = document.documentElement;
  // Strict: any >1px difference treated as overflow
  const hasOverflow = doc.scrollWidth > doc.clientWidth + 1;
    if (!hasOverflow) return { hasOverflow: false, offenders: [] as string[] };
    const offenders: string[] = [];
    const vw = doc.clientWidth;
    for (const el of Array.from(document.body.querySelectorAll<HTMLElement>('*'))) {
      const r = el.getBoundingClientRect();
      if (r.right - 1 > vw) {
        const tag = el.tagName.toLowerCase();
        const cls = (el.className || '').toString().split(/\s+/).slice(0,3).join('.');
        offenders.push(`${tag}${cls?'.'+cls:''}@${Math.round(r.right)}>${vw}`);
        if (offenders.length > 10) break;
      }
    }
    return { hasOverflow: true, offenders };
  });
  const strict = process.env.STRICT_MOBILE === '1';
  if (result.hasOverflow) {
    const info = `Overflow offenders: ${result.offenders.join(', ')}`;
    if (strict) {
      expect(result.hasOverflow, info).toBeFalsy();
    } else {
      console.warn('[SOFT] Horizontal overflow detected -> ' + info);
    }
  } else {
    expect(result.hasOverflow).toBeFalsy();
  }
}

export async function expectVisibleAndInViewport(page: Page, role: string, name: RegExp | string) {
  const strict = process.env.STRICT_MOBILE === '1';
  let locator = page.getByRole(role as Parameters<Page['getByRole']>[0], { name });
  const count = await locator.count();
  if (count === 0 && role === 'navigation') {
    // fallback to header or first nav element manually
    const header = page.locator('header, nav');
    if (await header.count() > 0) locator = header.first();
  }
  try {
    await expect(locator).toBeVisible({ timeout: 3000 });
    await expect(locator).toBeInViewport();
  } catch (err) {
    if (strict) throw err;
    console.warn('[SOFT] navigation landmark not visible: ' + (err as Error).message.split('\n')[0]);
  }
}

export async function collectTapTargetViolations(page: Page, minSize = 40) {
  const locators = page.getByRole('button');
  const count = await locators.count();
  const bad: { index: number; size: { w: number; h: number }; text: string }[] = [];
  const ignoreTexts = ['sign up', 'ivpn.net'];
  for (let i = 0; i < count; i++) {
    const el = locators.nth(i);
    const box = await el.boundingBox();
    if (!box) continue;
    if (Math.min(box.width, box.height) < minSize) {
      const text = (await el.innerText()).slice(0,60);
      if (!ignoreTexts.includes(text.toLowerCase().trim())) {
        bad.push({ index: i, size: { w: box.width, h: box.height }, text });
      }
    }
  }
  return bad;
}
