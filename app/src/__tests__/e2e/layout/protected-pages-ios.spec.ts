import { test, expect } from "@playwright/test";
import { registerMocks } from "../../mocks/registerMocks";

test.describe("Protected pages – iOS rendering", () => {
  // eslint-disable-next-line no-empty-pattern
  test.beforeEach(async ({}, testInfo) => {
    test.skip(
      testInfo.project.name !== "iphone15pro-dark",
      "iPhone 15 Pro only"
    );
  });

  test("Settings renders on iOS", async ({ page }) => {
    await registerMocks(page, { authenticated: true });
    await page.goto("/settings");
    await page.waitForLoadState("networkidle");
    // Verify key content is visible
    await expect(page.getByText(/settings/i).first()).toBeVisible();
  });

  test("Account Preferences renders on iOS", async ({ page }) => {
    await registerMocks(page, { authenticated: true });
    await page.goto("/account-preferences");
    await page.waitForLoadState("networkidle");
    await expect(page.getByText(/account/i).first()).toBeVisible();
  });

  test("Mobileconfig renders on iOS", async ({ page }) => {
    await registerMocks(page, { authenticated: true });
    await page.goto("/mobileconfig");
    await page.waitForLoadState("networkidle");
    await expect(page.getByText(/apple devices/i).first()).toBeVisible();
  });

  test("FAQ renders on iOS", async ({ page }) => {
    await page.goto("/faq");
    await page.waitForLoadState("networkidle");
    await expect(page.getByText(/faq|frequently/i).first()).toBeVisible();
  });
});
