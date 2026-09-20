import { expect, test } from "@playwright/test";
import { login } from "./layout-audit-helpers";

test("settings search crosses levels and preserves unsaved preferences", async ({ page }) => {
 test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated test instance");
 await login(page);
 await page.goto("/settings");
 const search = page.getByRole("searchbox", { name: "Search settings", exact: true });
 await expect(page.locator('[data-settings-levels] [aria-current="page"]')).toHaveText("Basic");
 await expect(page.locator(".settings-compatibility")).not.toHaveAttribute("open");
 await page.locator('[data-settings-nav] a[href="#general"]').click();
 const originalName = await page.getByRole("textbox", { name: "Server name", exact: true }).inputValue();
 await page.getByRole("textbox", { name: "Server name", exact: true }).fill("Unsaved name");
 await page.keyboard.press("Tab");
 await page.keyboard.press("/");
 await expect(search).toBeFocused();
 for (const query of ["remote access", "session timeout", "mfa"]) {
  await search.fill(query);
  await expect(page.locator("[data-settings-search-results] a").first()).toBeVisible();
 }
 await search.fill("remote access");
 await page.locator("[data-settings-search-results] a").first().click();
 await expect(page.locator('[data-settings-levels] [aria-current="page"]')).toHaveText("Advanced");
 await expect(search).toHaveValue("");
 await page.getByRole("navigation", { name: "Settings level" }).getByRole("link", { name: "Basic", exact: true }).click();
 await page.locator('[data-settings-nav] a[href="#general"]').click();
 await expect(page.getByRole("textbox", { name: "Server name", exact: true })).toHaveValue("Unsaved name");
 // Save the original value so the shared test instance is left unchanged.
 await page.getByRole("textbox", { name: "Server name", exact: true }).fill(originalName);
 await page.getByRole("button", { name: "Save Server name", exact: true }).click();
 await expect(page).toHaveURL(/\/settings#general$/);
 await expect(page.locator("#general")).toBeVisible();
 await search.fill("<script>definitely-no-match</script>");
 await expect(page.locator("[data-settings-search-status]")).toContainText("No settings found");
 await search.press("Escape");
 await expect(search).toHaveValue("");
 await page.goto("/settings#unknown-%broken");
 await expect(page.locator("#playback")).toBeVisible();
});
