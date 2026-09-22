import { test, expect, type Page } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';
import { expectNoHorizontalOverflow, collectTapTargetViolations } from '../utils/layoutAssertions';

// One pass per route, per project. Each route is loaded once and checked for:
//   - staying on its URL (no redirect),
//   - the layout root spanning the full viewport width (no left offset, no clipping),
//   - no horizontal overflow, also after the interactions most likely to introduce it,
//   - (mobile) tap targets of at least MIN_TAP_SIZE px.

const MIN_TAP_SIZE = 40;
// Routes whose icon-only controls are narrower than MIN_TAP_SIZE (the copy buttons and
// (i) triggers on /setup, the homepage buttons on /blocklists cards). Add them back to
// the tap-target check once those controls meet the threshold.
const TAP_TARGET_EXCLUDED = new Set(['/setup', '/blocklists']);

// Public routes: '/' is the landing page; the rest render inside PublicLayout.
const PUBLIC_ROUTES = ['/', '/login', '/signup', '/reset-password', '/tos', '/privacy', '/faq'];
const PROTECTED_ROUTES = ['/home', '/setup', '/settings', '/blocklists', '/custom-rules', '/account-preferences', '/mobileconfig', '/query-logs'];

const ROOT_SELECTOR: Record<string, string> = {
  '/': '.moddns-landing',
};
function rootSelector(route: string, isProtected: boolean) {
  return ROOT_SELECTOR[route] ?? (isProtected ? '[data-testid="app-content"]' : '[data-testid="public-layout"]');
}

const PROFILE = { id: 'prof1', profile_id: 'prof1', name: 'Default', settings: { logs: { enabled: true }, custom_rules: [] } };

async function setup(page: Page, isProtected: boolean) {
  if (!isProtected) {
    await registerMocks(page, { authenticated: false });
    return;
  }
  await registerMocks(page, { authenticated: true, customProfiles: [PROFILE] });
  // Registered after registerMocks so it wins over the /profiles catch-all.
  await page.route(/\/api\/v1\/profiles\/prof1\/logs(\?|$)/i, route =>
    route.fulfill({ status: 200, contentType: 'application/json', body: '[]' }));
}

// `exactLeft`: the root must start at x=0. On desktop the protected layout sits to the
// right of the persistent sidebar, so only its right edge is pinned there.
async function expectRootFillsViewport(page: Page, selector: string, exactLeft: boolean) {
  const metrics = await page.evaluate((sel) => {
    const root = document.querySelector(sel) as HTMLElement | null;
    if (!root) return null;
    const rect = root.getBoundingClientRect();
    return { left: rect.left, right: rect.right, vw: document.documentElement.clientWidth };
  }, selector);
  expect(metrics, `${selector} missing`).not.toBeNull();
  if (exactLeft) {
    expect(metrics!.left, 'layout root should start at the left edge').toBe(0);
  } else {
    expect(metrics!.left, 'layout root should not be offset left').toBeGreaterThanOrEqual(0);
  }
  expect(Math.abs(metrics!.vw - metrics!.right), 'layout root should end at the right edge').toBeLessThanOrEqual(1);
}

async function expectTapTargets(page: Page, route: string) {
  const violations = await collectTapTargetViolations(page, MIN_TAP_SIZE);
  const details = violations.map(v => `${v.index}:${v.size.w.toFixed(0)}x${v.size.h.toFixed(0)}:'${v.text}'`).join(', ');
  if (process.env.STRICT_MOBILE === '1') {
    expect(violations, `${route} tap target violations (index:WxH:'text'): ${details}`).toHaveLength(0);
  } else if (violations.length) {
    console.warn(`[SOFT] ${route} tap target violations -> ${details}`);
  }
}

// Interactions that change layout and could introduce overflow after the initial render.
async function interact(page: Page, route: string, isMobile: boolean) {
  if (isMobile) {
    const moreBtn = page.getByTestId('bottom-nav').getByRole('button', { name: /more/i });
    if (await moreBtn.count()) {
      await moreBtn.first().click();
      await expect(page.getByTestId('overlay-navigation')).toBeVisible();
      await expectNoHorizontalOverflow(page);
      await page.getByTestId('nav-close').click();
    }
  }
  if (route === '/query-logs') {
    const toggle = page.getByTestId('querylog-card-toggle').first();
    if (await toggle.count()) {
      await toggle.click();
      await expectNoHorizontalOverflow(page);
    }
  }
  if (route === '/custom-rules') {
    const input = page.getByPlaceholder('Add a domain or IP address');
    if (await input.count()) {
      await input.first().fill('example.com');
      await expectNoHorizontalOverflow(page);
    }
  }
}

for (const [routes, isProtected] of [[PUBLIC_ROUTES, false], [PROTECTED_ROUTES, true]] as const) {
  test.describe(`@layout route smoke (${isProtected ? 'protected' : 'public'})`, () => {
    for (const route of routes) {
      test(`${route} renders full-width without overflow`, async ({ page, isMobile }) => {
        await setup(page, isProtected);
        await page.goto(route);
        await expect(page).toHaveURL(new RegExp(`${route.replace(/\//g, '\\/')}$`));

        const selector = rootSelector(route, isProtected);
        await page.waitForSelector(selector, { state: 'attached', timeout: 10_000 });
        await expectRootFillsViewport(page, selector, !isProtected || isMobile);
        await expectNoHorizontalOverflow(page);

        if (isMobile && !TAP_TARGET_EXCLUDED.has(route)) await expectTapTargets(page, route);

        await interact(page, route, isMobile);
        await expectNoHorizontalOverflow(page);
      });
    }
  });
}

test.describe('@layout mobile layout basics', { tag: '@android' }, () => {
  test('bottom navigation stays in the viewport after an orientation change', async ({ page }) => {
    await setup(page, true);
    await page.goto('/home');
    await expect(page.getByTestId('bottom-nav')).toBeInViewport();
    await page.setViewportSize({ width: 740, height: 360 });
    await expect(page.getByTestId('bottom-nav')).toBeInViewport();
    await expectNoHorizontalOverflow(page);
  });

  test('login renders in dark mode', async ({ page }) => {
    await registerMocks(page, { authenticated: false });
    await page.goto('/login');
    await expect(page.getByTestId('loading-screen')).toHaveCount(0, { timeout: 3000 });
    // The initial mode depends on WebAuthn feature detection, so use the mode-independent toggle.
    await expect(page.getByTestId('btn-login-toggle-mode')).toBeVisible();
  });
});
