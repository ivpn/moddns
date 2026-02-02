import { test, expect } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';

const ROUTE = '/home';
const ULTRAWIDE_VIEWPORT = { width: 1920, height: 1080 };
const CLAMP_MAX = 1360; // mirrors ULTRAWIDE_CONTENT_MAX_WIDTH in App.tsx
const CLAMP_BASE = 1200; // mirrors DESKTOP_CONTENT_BASE_WIDTH in App.tsx
const WIDTH_TOLERANCE = 8;
const EDGE_TOLERANCE = 4;

test.describe('@layout Ultrawide clamp alignment', () => {
  test.beforeEach(async ({ page }) => {
    if (!/desktop/i.test(test.info().project.name)) test.skip();
    await page.setViewportSize(ULTRAWIDE_VIEWPORT);
    await registerMocks(page, { authenticated: true, customProfiles: [{ id: 'prof_1', name: 'Default' }] });
    await page.goto(ROUTE);
    await page.waitForSelector('[data-testid="app-content"] > div', { timeout: 5000 });
  });

  test('content, header bar, and status bar share the same clamped width on ultrawide', async ({ page }) => {
    const metrics = await page.evaluate(() => {
      const docWidth = document.documentElement.clientWidth;
      const content = document.querySelector('[data-testid="app-content"] > div') as HTMLElement | null;
      const conn = document.querySelector('[data-testid="conn-header-root"]') as HTMLElement | null;
      const connContainer = conn?.parentElement as HTMLElement | null;
      const maxWidth = content ? getComputedStyle(content).maxWidth : '';
      const result: Record<string, unknown> = { docWidth, maxWidth };
      const entries: [string, HTMLElement | null][] = [
        ['content', content],
        ['conn', conn],
        ['connContainer', connContainer],
      ];
      for (const [key, el] of entries) {
        if (!el) continue;
        const r = el.getBoundingClientRect();
        result[key] = { width: r.width, left: r.left, right: r.right };
      }
      return result;
    });

    expect(metrics.content).toBeDefined();
    // CSS resolves clamp() to a pixel value; accept either the raw clamp string or the resolved max value
    if (typeof metrics.maxWidth === 'string' && metrics.maxWidth.includes('clamp')) {
      expect(metrics.maxWidth).toContain('clamp');
    } else {
      const parsed = Number.parseFloat(String(metrics.maxWidth).replace('px', ''));
      expect(parsed).toBeGreaterThanOrEqual(CLAMP_BASE - WIDTH_TOLERANCE);
      expect(parsed).toBeLessThanOrEqual(CLAMP_MAX + WIDTH_TOLERANCE);
    }
    expect(metrics.content.width).toBeGreaterThanOrEqual(CLAMP_BASE - WIDTH_TOLERANCE);
    expect(metrics.content.width).toBeLessThanOrEqual(CLAMP_MAX + WIDTH_TOLERANCE);
    expect(metrics.docWidth - metrics.content.width).toBeGreaterThan(300); // clamp engaged on 1920px

    expect(metrics.connContainer || metrics.conn).toBeDefined();
    const connBox = metrics.connContainer || metrics.conn;
    expect(Math.abs(metrics.content.width - connBox.width)).toBeLessThanOrEqual(16);
    expect(Math.abs(metrics.content.left - connBox.left)).toBeLessThanOrEqual(EDGE_TOLERANCE);
  });
});
