import { test, expect } from "@playwright/test";
import { collectTapTargetViolations } from "../utils/layoutAssertions";

const STRICT = process.env.STRICT_MOBILE === "1";
const MIN_SIZE = 40;

const protectedRoutes = [
  { path: "/settings", name: "Settings" },
  { path: "/blocklists", name: "Blocklists" },
  { path: "/custom-rules", name: "Custom Rules" },
  { path: "/account-preferences", name: "Account Preferences" },
  { path: "/mobileconfig", name: "Mobileconfig" },
  { path: "/query-logs", name: "Query Logs" },
  { path: "/faq", name: "FAQ" },
];

test.describe("Tap targets – protected pages", { tag: "@mobile" }, () => {
  for (const route of protectedRoutes) {
    test(`${route.name} has adequate tap targets`, async ({ page }) => {
      await page.goto(route.path);
      await page.waitForLoadState("networkidle");

      const violations = await collectTapTargetViolations(page, MIN_SIZE);

      if (STRICT) {
        expect(violations).toHaveLength(0);
      } else {
        // Soft mode: log but don't fail
        if (violations.length > 0) {
          console.warn(
            `${route.name}: ${violations.length} tap target violations found`
          );
        }
      }
    });
  }
});
