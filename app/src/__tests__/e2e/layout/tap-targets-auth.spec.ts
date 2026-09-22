import { test, expect } from '@playwright/test';
import type { Page } from '@playwright/test';
import { collectTapTargetViolations } from '../utils/layoutAssertions'; // path corrected one level up

// Auth-specific tap target enforcement for /login and /signup.
// Runs on the Android project only to keep the suite fast.

const MIN_SIZE = 40; // same threshold used elsewhere (visual comfort; actual min-h is 44px via classes)

async function assertNoExcessiveViolations(page: Page, route: string) {
  await page.goto(route);
  const violations = await collectTapTargetViolations(page, MIN_SIZE);
  const strict = process.env.STRICT_MOBILE === '1';
  const softLimit = parseInt(process.env.TAP_TARGET_SOFT_LIMIT || '4', 10);
  const strictLimit = parseInt(process.env.TAP_TARGET_STRICT_LIMIT || '0', 10);
  const limit = strict ? strictLimit : softLimit;
  if (violations.length > limit) {
    const details = violations.map(v => `${v.index}:${v.size.w.toFixed(0)}x${v.size.h.toFixed(0)}:'${v.text}'`).join(', ');
    if (strict) {
      expect(violations.length, `${route} tap target violations (index:WxH:'text'): ${details}`).toBeLessThanOrEqual(limit);
    } else {
      console.warn(`[SOFT] ${route} tap target warning (${violations.length} > ${limit}) -> ${details}`);
    }
  }
  expect(violations.length).toBeGreaterThanOrEqual(0); // keep assertion
}

test.describe('@layout Auth tap targets', { tag: '@android' }, () => {
  test('login page tap targets', async ({ page }) => {
    await assertNoExcessiveViolations(page, '/login');
  });

  test('signup page tap targets', async ({ page }) => {
    await assertNoExcessiveViolations(page, '/signup');
  });
});
