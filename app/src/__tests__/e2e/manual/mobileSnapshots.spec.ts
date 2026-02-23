import { test, expect } from '@playwright/test';
import fs from 'node:fs';
import path from 'node:path';
import { registerMocks } from '../../mocks/registerMocks';

// Manual mobile snapshot suite.
// Trigger ONLY via: MOBILE_SNAPSHOTS=1 npm run snapshots:mobile
// Not included in CI because CI won't set MOBILE_SNAPSHOTS.

// Order: public first (baseline) then protected pages (should render authenticated views)
const ROUTES = [
  // Public (still useful to snapshot their appearance)
  '/login',
  '/signup',
  '/faq',
  // Protected
  '/home',
  '/setup',
  '/settings',
  '/blocklists',
  '/custom-rules',
  '/account-preferences',
  '/mobileconfig',
  '/query-logs',
];

const PROTECTED_PREFIXES = [
  '/home',
  '/setup',
  '/settings',
  '/blocklists',
  '/custom-rules',
  '/account-preferences',
  '/mobileconfig',
  '/query-logs',
];


const shouldRun = process.env.MOBILE_SNAPSHOTS === '1';

(shouldRun ? test.describe : test.describe.skip)('Manual mobile full-page snapshots', () => {
  for (const route of ROUTES) {
    test(`snapshot: ${route}`, async ({ page }) => {
      const project = test.info().project.name;
      // Only run on selected mobile device variants (android baseline + ios baseline)
      if (!['chromium-mobile', 'iphone15pro'].includes(project)) test.skip();
      const isProtected = PROTECTED_PREFIXES.some(p => route.startsWith(p));
      await registerMocks(page, { authenticated: isProtected });
      await page.goto(route);
      await page.waitForTimeout(50);
      // Use Playwright snapshot for baseline comparison
      const sanitized = route === '/' ? 'root' : route.replace(/^\//,'').replace(/\//g,'_');
      const pwName = `${project}-${sanitized}.png`;
      await expect(page).toHaveScreenshot(pwName, {
        fullPage: true,
        animations: 'disabled'
      });

      // After assertion, copy the actual (baseline) file into dedicated folder
      // Baseline lives under: <testFile>-snapshots/<pwName>-<project>-linux.png
      const testDir = path.dirname(test.info().file); // path to spec file directory
      const snapshotsDir = path.join(testDir, 'mobile-device-snapshots');
      const device = project === 'chromium-mobile' ? 'android' : 'ios';
      const targetDir = path.join(snapshotsDir, device);
      fs.mkdirSync(targetDir, { recursive: true });
      // Playwright baseline naming: <spec>-snapshots/<test-title-part>.png
      const specBase = `${path.basename(test.info().file)}-snapshots`;
      const baselinePath = path.join(path.dirname(test.info().file), `${specBase}`, `${pwName.replace(/\.png$/, '')}-${project}-linux.png`);
      if (fs.existsSync(baselinePath)) {
        const destPath = path.join(targetDir, `${sanitized}.png`);
        fs.copyFileSync(baselinePath, destPath);
      }
    });
  }
});
