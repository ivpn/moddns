/**
 * Playwright integration tests for the Backup & Restore UI (export/import profiles).
 *
 * specRef: E1, E3, E13-E15, I1, I2, I4, I8, I19-I20, S5
 * (rows from docs/specs/account-export-import-behaviour.md)
 *
 * All API calls are mocked via page.route() — no real backend required.
 * The app server must be running on localhost:5173 (started by playwright.config.ts).
 */
import { test, expect } from '@playwright/test';

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

const ACCOUNT_URL = 'http://localhost:5173/account-preferences';

/** Minimum API mocks required to render the account page without errors. */
async function setupBaseMocks(page: import('@playwright/test').Page, opts: { subscriptionStatus?: string } = {}) {
    const status = opts.subscriptionStatus ?? 'active';

    await page.addInitScript(() => {
        window.localStorage.setItem('isAuthenticated', 'true');
    });

    await page.route('**/api/v1/accounts/current', route =>
        route.fulfill({
            status: 200,
            contentType: 'application/json',
            body: JSON.stringify({
                account_id: 'acct-1',
                email: 'user@example.com',
                email_verified: true,
                mfa: { totp: { enabled: false } },
            }),
        })
    );

    await page.route('**/api/v1/profiles', route => {
        if (route.request().method() === 'GET') {
            return route.fulfill({
                status: 200,
                contentType: 'application/json',
                body: JSON.stringify([
                    { profile_id: 'prof-1', account_id: 'acct-1', name: 'Home', settings: { profile_id: 'prof-1', advanced: {}, logs: { enabled: false }, privacy: { default_rule: 'allow', blocklists_subdomains_rule: 'allow', blocklists: [] }, security: {}, statistics: { enabled: false }, custom_rules: [] } },
                    { profile_id: 'prof-2', account_id: 'acct-1', name: 'Work', settings: { profile_id: 'prof-2', advanced: {}, logs: { enabled: false }, privacy: { default_rule: 'allow', blocklists_subdomains_rule: 'allow', blocklists: [] }, security: {}, statistics: { enabled: false }, custom_rules: [] } },
                ]),
            });
        }
        return route.continue();
    });

    await page.route('**/api/v1/sub', route =>
        route.fulfill({
            status: 200,
            contentType: 'application/json',
            body: JSON.stringify({ status, plan: 'plus', active_until: '2027-01-01T00:00:00Z' }),
        })
    );

    // Passkeys list — empty so password method is shown by default
    await page.route('**/api/v1/webauthn/passkeys', route =>
        route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify([]) })
    );
}

/** Navigate to the account page and wait for the backup section to be visible. */
async function gotoAccount(page: import('@playwright/test').Page) {
    await page.goto(ACCOUNT_URL);
    const section = page.getByTestId('backup-restore-section');
    await expect(section).toBeVisible({ timeout: 10_000 });
}

/** Minimal valid moddns export payload. */
function makeExportPayload(profileNames: string[] = ['Home', 'Work', 'Guest']) {
    return {
        schemaVersion: 1,
        kind: 'moddns-export',
        exportedAt: '2026-05-13T00:00:00Z',
        profiles: profileNames.map((name, i) => ({
            profile_id: `exported-prof-${i + 1}`,
            name,
            settings: { profile_id: `exported-prof-${i + 1}`, advanced: {}, logs: { enabled: false }, privacy: { default_rule: 'allow', blocklists_subdomains_rule: 'allow', blocklists: [] }, security: {}, statistics: { enabled: false }, custom_rules: [] },
        })),
    };
}

/** Upload a JSON object as a file to the dropzone via the hidden <input type="file">. */
async function uploadJsonToDropzone(page: import('@playwright/test').Page, payload: object) {
    const json = JSON.stringify(payload);
    const fileInput = page.locator('[data-testid="import-dropzone"] input[type="file"]');
    await fileInput.setInputFiles({
        name: 'test-export.moddns.json',
        mimeType: 'application/json',
        buffer: Buffer.from(json),
    });
}

// ---------------------------------------------------------------------------
// Test 1: Happy-path export — specRef E1, E13-E15
// ---------------------------------------------------------------------------

test.describe('Backup & Restore — Export', () => {
    test('happy-path export triggers file download', async ({ page }) => {
        await setupBaseMocks(page);

        const exportBlob = JSON.stringify({
            schemaVersion: 1,
            kind: 'moddns-export',
            profiles: [{ profile_id: 'prof-1', name: 'Home' }],
        });

        await page.route('**/api/v1/profiles/export', route =>
            route.fulfill({
                status: 200,
                headers: {
                    'Content-Type': 'application/vnd.moddns.export+json',
                    'Content-Disposition': 'attachment; filename="moddns-export-2026-05-13.moddns.json"',
                },
                body: exportBlob,
            })
        );

        await gotoAccount(page);

        // Open export dialog
        await page.getByTestId('btn-export-profiles').click();
        await expect(page.getByTestId('export-dialog')).toBeVisible();

        // Fill password — password method is active (no passkeys registered)
        await page.getByLabel('Current password').fill('correct-password');

        // Start listening for download before clicking submit
        const downloadPromise = page.waitForEvent('download');

        await page.getByTestId('export-submit-btn').click();

        // File download must be triggered
        const download = await downloadPromise;
        expect(download.suggestedFilename()).toMatch(/moddns-export.*\.json$/);
    });

    // specRef: E3 — 401 reauth failure keeps dialog open with inline error
    test('401 on export shows inline reauth error and keeps dialog open', async ({ page }) => {
        await setupBaseMocks(page);

        await page.route('**/api/v1/profiles/export', route =>
            route.fulfill({
                status: 401,
                contentType: 'application/json',
                body: JSON.stringify({ error: 'authentication required' }),
            })
        );

        await gotoAccount(page);

        await page.getByTestId('btn-export-profiles').click();
        await expect(page.getByTestId('export-dialog')).toBeVisible();

        await page.getByLabel('Current password').fill('wrong-password');
        await page.getByTestId('export-submit-btn').click();

        // Dialog stays open
        await expect(page.getByTestId('export-dialog')).toBeVisible();

        // Inline error visible — specRef E3
        const reauthError = page.getByTestId('reauth-error');
        await expect(reauthError).toBeVisible();
        await expect(reauthError).toContainText(/reauth/i);
    });
});

// ---------------------------------------------------------------------------
// Test 2: Happy-path import — specRef I1, I4, I19-I20
// ---------------------------------------------------------------------------

test.describe('Backup & Restore — Import', () => {
    test('happy-path import shows results in step 3 and Done closes dialog', async ({ page }) => {
        await setupBaseMocks(page);

        await page.route('**/api/v1/profiles/import', route =>
            route.fulfill({
                status: 200,
                contentType: 'application/json',
                body: JSON.stringify({ createdProfileIds: ['new-p1'], warnings: [] }),
            })
        );

        // Refresh profiles after done
        await page.route('**/api/v1/profiles', route => {
            if (route.request().method() === 'GET') {
                return route.fulfill({
                    status: 200,
                    contentType: 'application/json',
                    body: JSON.stringify([]),
                });
            }
            return route.continue();
        });

        await gotoAccount(page);

        await page.getByTestId('btn-import-profiles').click();
        await expect(page.getByTestId('import-dialog')).toBeVisible();

        // Step 1: upload file
        await uploadJsonToDropzone(page, makeExportPayload(['Home']));

        // Wait for dialog to advance to step 2
        await expect(page.getByTestId('import-submit-btn')).toBeVisible({ timeout: 5_000 });

        // Step 2: enter password
        await page.getByLabel('Current password').fill('correct-password');

        // Submit import
        await page.getByTestId('import-submit-btn').click();

        // Step 3: results
        const resultsStep = page.getByTestId('import-results-step');
        await expect(resultsStep).toBeVisible({ timeout: 5_000 });
        await expect(resultsStep).toContainText('new-p1');

        // Done closes dialog — specRef I19-I20
        await page.getByTestId('import-done-btn').click();
        await expect(page.getByTestId('import-dialog')).not.toBeVisible();
    });

    // specRef: I8 — partial selection filters payload.profiles[]
    test('partial import sends only selected profiles in request body', async ({ page }) => {
        await setupBaseMocks(page);

        let capturedBody: { payload?: { profiles?: unknown[] } } | null = null;

        await page.route('**/api/v1/profiles/import', async route => {
            const raw = route.request().postData();
            capturedBody = raw ? JSON.parse(raw) : null;
            return route.fulfill({
                status: 200,
                contentType: 'application/json',
                body: JSON.stringify({ createdProfileIds: ['new-p1', 'new-p2'], warnings: [] }),
            });
        });

        await gotoAccount(page);

        await page.getByTestId('btn-import-profiles').click();

        // Upload file with 3 profiles
        await uploadJsonToDropzone(page, makeExportPayload(['Alpha', 'Beta', 'Gamma']));

        // Wait for step 2
        await expect(page.getByTestId('import-submit-btn')).toBeVisible({ timeout: 5_000 });

        // Uncheck the last checkbox (Gamma, index 2)
        // The checkboxes are rendered in the MultiSelectProfileList
        const checkboxes = page.locator('[role="checkbox"]');
        const count = await checkboxes.count();
        expect(count).toBeGreaterThanOrEqual(3);
        // The last profile checkbox (Gamma)
        await checkboxes.nth(count - 1).click();

        // Fill password and submit
        await page.getByLabel('Current password').fill('correct-password');
        await page.getByTestId('import-submit-btn').click();

        // Wait for result step
        await expect(page.getByTestId('import-results-step')).toBeVisible({ timeout: 5_000 });

        // Assert body had only 2 profiles — specRef I8
        expect(capturedBody).not.toBeNull();
        expect(capturedBody!.payload?.profiles).toHaveLength(2);
    });

    // specRef: S5 — IDN warning row has amber/yellow treatment
    test('IDN warning gets amber treatment; generic warning gets neutral treatment', async ({ page }) => {
        await setupBaseMocks(page);

        await page.route('**/api/v1/profiles/import', route =>
            route.fulfill({
                status: 200,
                contentType: 'application/json',
                body: JSON.stringify({
                    createdProfileIds: ['new-p1'],
                    warnings: [
                        'customRules[0] contains internationalized domain — decoded form: xn--nxasmq6b.com',
                        'blocklist "unknown-list" not found in catalog',
                    ],
                }),
            })
        );

        await gotoAccount(page);

        await page.getByTestId('btn-import-profiles').click();
        await uploadJsonToDropzone(page, makeExportPayload(['Home']));

        await expect(page.getByTestId('import-submit-btn')).toBeVisible({ timeout: 5_000 });
        await page.getByLabel('Current password').fill('correct-password');
        await page.getByTestId('import-submit-btn').click();

        await expect(page.getByTestId('import-results-step')).toBeVisible({ timeout: 5_000 });

        // IDN warning row — amber/yellow classes — specRef S5
        const idnRow = page.getByTestId('idn-warning-row');
        await expect(idnRow).toBeVisible();
        await expect(idnRow).toHaveClass(/yellow/);

        // Generic warning row — neutral (slate) classes
        const genericRow = page.getByTestId('generic-warning-row');
        await expect(genericRow).toBeVisible();
        await expect(genericRow).not.toHaveClass(/yellow/);
    });

    // Tier gating — specRef V5 (subscription guard)
    test('Import button is disabled for restricted subscription', async ({ page }) => {
        await setupBaseMocks(page, { subscriptionStatus: 'limited_access' });

        await gotoAccount(page);

        const importBtn = page.getByTestId('btn-import-profiles');
        await expect(importBtn).toBeVisible();
        await expect(importBtn).toBeDisabled();

        // Export button remains enabled (GDPR Art. 20)
        const exportBtn = page.getByTestId('btn-export-profiles');
        await expect(exportBtn).toBeEnabled();
    });

    // specRef: I2 — 401 on import shows inline error, dialog stays open
    test('401 on import shows inline reauth error and keeps dialog open', async ({ page }) => {
        await setupBaseMocks(page);

        await page.route('**/api/v1/profiles/import', route =>
            route.fulfill({
                status: 401,
                contentType: 'application/json',
                body: JSON.stringify({ error: 'authentication required' }),
            })
        );

        await gotoAccount(page);

        await page.getByTestId('btn-import-profiles').click();
        await uploadJsonToDropzone(page, makeExportPayload(['Home']));

        await expect(page.getByTestId('import-submit-btn')).toBeVisible({ timeout: 5_000 });
        await page.getByLabel('Current password').fill('wrong-password');
        await page.getByTestId('import-submit-btn').click();

        // Dialog stays open at step 2 — specRef I2
        await expect(page.getByTestId('import-dialog')).toBeVisible();
        await expect(page.getByTestId('import-results-step')).not.toBeVisible();

        // Inline error — specRef I2
        const reauthError = page.getByTestId('reauth-error');
        await expect(reauthError).toBeVisible();
        await expect(reauthError).toContainText(/reauth/i);
    });
});
