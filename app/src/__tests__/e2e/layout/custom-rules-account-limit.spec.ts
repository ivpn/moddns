import { test, expect } from '@playwright/test';
import { registerMocks } from '../../mocks/registerMocks';

function rules(count: number, prefix: string) {
  return Array.from({ length: count }, (_, i) => ({
    id: `${prefix}${i}`, action: 'block', value: `${prefix}${i}.example.com`, order: i,
  }));
}

function profile(id: string, name: string, ruleCount: number) {
  return { id, profile_id: id, name, settings: { custom_rules: rules(ruleCount, id) } };
}

// The custom-rules cap is account-wide, so the usage notice counts every
// profile, not only the one being edited.
test.describe('@layout Custom Rules account limit notice', () => {
  test('near the cap shows account-wide usage', async ({ page }) => {
    await registerMocks(page, {
      authenticated: true,
      customProfiles: [profile('prof1', 'Default', 214), profile('prof2', 'Kids', 9000)],
    });

    await page.goto('/custom-rules');

    const notice = page.getByRole('status').filter({ hasText: 'custom rules used across all profiles' });
    await expect(notice).toContainText('9,214 of 10,000 custom rules used across all profiles.');
    await expect(notice).not.toContainText("won't be accepted");
  });

  test('at the cap warns that new rules are refused', async ({ page }) => {
    await registerMocks(page, {
      authenticated: true,
      customProfiles: [profile('prof1', 'Default', 10), profile('prof2', 'Kids', 9990)],
    });

    await page.goto('/custom-rules');

    const alert = page.getByRole('alert').filter({ hasText: 'custom rules used across all profiles' });
    await expect(alert).toContainText('10,000 of 10,000 custom rules used across all profiles.');
    await expect(alert).toContainText("won't be accepted until you remove some rules from any profile");
  });
});
