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
  projects: [
    // Auth storage bootstrap project (runs first to produce storageState for dependent projects)
    {
      name: 'auth-setup',
      testMatch: /auth\.setup\.ts/,
    },
    // Mobile baseline: Pixel 5 (Chromium engine)
    { name: 'chromium-mobile-dark', use: { ...devices['Pixel 5'], colorScheme: 'dark', storageState: 'src/__tests__/e2e/.auth-storage.json' }, dependencies: ['auth-setup'] },
    // Safari/WebKit coverage: latest supported iPhone (15 Pro) using default WebKit engine
    { name: 'iphone15pro-dark', use: { ...devices['iPhone 15 Pro'], colorScheme: 'dark', storageState: 'src/__tests__/e2e/.auth-storage.json' }, dependencies: ['auth-setup'] },
    // Desktop without pre-auth storage so login flows can exercise authentication UI
    { name: 'chromium-desktop', use: { ...devices['Desktop Chrome'] } }
  ]
});