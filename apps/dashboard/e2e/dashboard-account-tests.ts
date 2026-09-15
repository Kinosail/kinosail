import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { expectNoOverflow, login, password, screenshot } from "./dashboard-helpers";

export function registerDashboardAccountTests() {
  test("keeps compact cards and edit controls fully interactive", async ({ page }, testInfo) => {
    await page.setViewportSize({ width: 320, height: 700 });
    await login(page);
    const tileLinks = page.locator(".app-tile .app-link");
    for (let index = 0; index < await tileLinks.count(); index++) {
      const tile = await tileLinks.nth(index).locator("..").boundingBox();
      const link = await tileLinks.nth(index).boundingBox();
      expect(Math.abs((tile?.height ?? 0) - (link?.height ?? 0))).toBeLessThanOrEqual(2);
    }
    await page.getByRole("button", { name: "Edit board" }).click();
    await expect(page.locator("#toast")).toBeHidden();
    const tiles = page.locator(".app-tile");
    await expect(tiles.first().locator(".edit-name")).toHaveText("Jellyfin");
    for (let index = 0; index < await tiles.count(); index++) {
      const tile = await tiles.nth(index).boundingBox();
      const controls = tiles.nth(index).locator(".edit-controls button");
      for (let controlIndex = 0; controlIndex < await controls.count(); controlIndex++) {
        const control = await controls.nth(controlIndex).boundingBox();
        expect(Math.min(control?.width ?? 0, control?.height ?? 0)).toBeGreaterThanOrEqual(44);
        expect((control?.x ?? 0) + (control?.width ?? 0)).toBeLessThanOrEqual((tile?.x ?? 0) + (tile?.width ?? 0) + 1);
        expect(control?.x ?? 0).toBeGreaterThanOrEqual((tile?.x ?? 0) - 1);
      }
    }
    await screenshot(page, testInfo, "compact-edit-controls-320.png");
  });

  test("creates and revokes an MCP token through settings", async ({ page }) => {
    await login(page);
    await page.getByRole("button", { name: "Open board settings" }).click();
    const settings = page.locator("#settings-dialog");
    const audit = settings.getByLabel("Recent changes history");
    await audit.focus();
    await expect(audit).toBeFocused();
    await settings.getByRole("button", { name: "Create MCP token" }).click();
    const token = settings.getByLabel("New MCP token");
    await expect(token).toBeVisible();
    expect((await token.inputValue()).length).toBeGreaterThan(40);
    await settings.getByRole("button", { name: "Revoke token" }).click();
    await expect(token).toBeHidden();
    await expect(page.getByText("MCP token revoked")).toBeVisible();
  });

  test("rotates the Owner password without ending the current session", async ({ page }) => {
    const rotatedPassword = "another correct horse battery staple";
    await login(page);
    await page.getByRole("button", { name: "Open board settings" }).click();
    const settings = page.locator("#settings-dialog");
    expect((await new AxeBuilder({ page }).include("#settings-dialog").analyze()).violations).toEqual([]);
    await settings.getByLabel("Current password").fill(password);
    await settings.getByLabel("New password", { exact: true }).fill(rotatedPassword);
    await settings.getByLabel("Confirm new password").fill(rotatedPassword);
    await settings.getByRole("button", { name: "Change password" }).click();
    await expect(page.getByText("Password changed. Other sessions were signed out.")).toBeVisible();
    expect((await page.request.get("/api/v1/board")).ok()).toBeTruthy();

    await settings.getByRole("button", { name: "Sign out" }).click();
    await expect(page).toHaveURL(/\/login$/);
    await login(page, rotatedPassword);
    await page.getByRole("button", { name: "Open board settings" }).click();
    await settings.getByLabel("Current password").fill(rotatedPassword);
    await settings.getByLabel("New password", { exact: true }).fill(password);
    await settings.getByLabel("Confirm new password").fill(password);
    await settings.getByRole("button", { name: "Change password" }).click();
    await expect(page.getByText("Password changed. Other sessions were signed out.")).toBeVisible();
    await settings.getByRole("button", { name: "Sign out" }).click();
    await expect(page).toHaveURL(/\/login$/);
    await login(page);
  });

  test("supports keyboard command selection and visible focus", async ({ page }) => {
    await login(page);
    await page.keyboard.press("Control+k");
    const commandSearch = page.getByLabel("Search commands");
    await expect(commandSearch).toBeFocused();
    await commandSearch.fill("zzzzzzzz");
    await expect(page.getByText("No matching applications or commands. Try another search.")).toBeVisible();
    await commandSearch.press("Enter");
    await expect(commandSearch).toBeFocused();
    await expect(page.locator("#command-results button")).toHaveCount(0);
    await commandSearch.fill("Add application");
    await expect(page.getByText("No matching applications or commands. Try another search.")).toBeHidden();
    await commandSearch.press("ArrowDown");
    await expect(page.locator("#command-results").getByRole("button", { name: /Add application/ })).toBeFocused();
    await page.keyboard.press("Enter");
    await expect(page.getByRole("heading", { name: "Add application" })).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(page.locator("#app-dialog")).not.toHaveAttribute("open", "");

    await page.locator(".skip-link").focus();
    const focus = await page.evaluate(() => {
      const element = document.activeElement;
      const style = element ? getComputedStyle(element) : null;
      return { text: element?.textContent?.trim(), outline: style?.outlineStyle, width: style?.outlineWidth };
    });
    expect(focus.text).toBe("Skip to applications");
    expect(focus.outline).not.toBe("none");
  });

  test("clears the connection warning after a successful refresh", async ({ page }) => {
    await login(page);
    let failNextBoardRead = false;
    await page.route("**/api/v1/board", async route => {
      if (failNextBoardRead && route.request().method() === "GET") {
        failNextBoardRead = false;
        await route.abort("failed");
        return;
      }
      await route.continue();
    });
    await page.getByRole("button", { name: "Open board settings" }).click();
    await page.getByLabel("Board name").fill("Recovered board");
    failNextBoardRead = true;
    await page.getByRole("button", { name: "Save board" }).click();
    await expect(page.locator("#offline-banner")).toBeVisible();
    await page.getByRole("button", { name: "Retry" }).click();
    await expect(page.locator("#offline-banner")).toBeHidden();
    await expect(page.getByRole("heading", { name: "Recovered board" })).toBeVisible();
    await page.getByRole("button", { name: "Open board settings" }).click();
    await page.getByLabel("Board name").fill("Home");
    await page.getByRole("button", { name: "Save board" }).click();
  });
}
