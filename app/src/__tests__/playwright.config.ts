import { defineConfig, devices } from '@playwright/test';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

export default defineConfig({
  testDir: './e2e',
  timeout: 30 * 1000,
  expect: { timeout: 5000 },
  // Must stay well under the 30-minute job timeout in .github/workflows/frontend.yml.
  globalTimeout: 20 * 60 * 1000,
  maxFailures: 10,
  fullyParallel: true,
  // Public-repo ubuntu-latest runners have 4 vCPUs; leave one for the vite server.
  workers: process.env.CI ? 3 : undefined,
  retries: 1,
  reportSlowTests: { max: 10, threshold: 30 * 1000 },
  reporter: [
    ['github'],
    ['html', { open: 'never' }]
  ],
  use: {
    baseURL: 'http://localhost:5173',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure'
  },
  webServer: {
    command: 'npm run ci',
    cwd: path.join(__dirname, '../../'), // back to app root
    port: 5173,
    reuseExistingServer: !process.env.CI,
    timeout: 60_000
  },
  // The manual snapshot suite only runs when explicitly requested (npm run snapshots:mobile).
  testIgnore: process.env.MOBILE_SNAPSHOTS === '1' ? [] : [/\/manual\//],
  // Project scoping is declared with tags, not runtime test.skip() calls (a skipped test
  // still pays for its browser context). Tags: @desktop (Chromium desktop), @android
  // (Pixel 5, Chromium), @ios (iPhone 15 Pro, WebKit), @mobile (both mobile projects).
  // Untagged tests run on every project; a tagged test runs only on the projects named.
  // Every spec seeds its own auth state (registerMocks / addInitScript), so no project
  // needs a shared storageState.
  projects: [
    // Mobile baseline: Pixel 5 (Chromium engine)
    {
      name: 'chromium-mobile-dark',
      use: { ...devices['Pixel 5'], colorScheme: 'dark' },
      grepInvert: /^(?!.*@(?:android|mobile)\b)(?=.*@(?:desktop|ios)\b)/,
    },
    // Safari/WebKit coverage: latest supported iPhone (15 Pro) using default WebKit engine
    {
      name: 'iphone15pro-dark',
      use: { ...devices['iPhone 15 Pro'], colorScheme: 'dark' },
      grepInvert: /^(?!.*@(?:ios|mobile)\b)(?=.*@(?:desktop|android)\b)/,
    },
    {
      name: 'chromium-desktop',
      use: { ...devices['Desktop Chrome'] },
      grepInvert: /^(?!.*@desktop\b)(?=.*@(?:android|ios|mobile)\b)/,
    },
  ]
});