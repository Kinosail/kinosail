import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { expectNoOverflow, login, owner, password, resetBoard, screenshot } from "./dashboard-helpers";

export function registerDashboardAuthTests() {
  test("sets up the first Owner with clear validation", async ({ page }, testInfo) => {
    await page.goto("/");
    if (!page.url().endsWith("/setup")) {
      await login(page);
      await resetBoard(page);
    } else {
      await expect(page.getByRole("heading", { name: "Make this dashboard yours." })).toBeVisible();
      await expect(page.getByLabel("Setup progress")).toContainText("Your Owner account");
      for (const viewport of [
        { width: 1440, height: 900 }, { width: 1024, height: 768 }, { width: 720, height: 450 },
        { width: 390, height: 844 }, { width: 320, height: 700 },
      ]) {
        await page.setViewportSize(viewport);
        await expectNoOverflow(page);
        const controls = page.locator("#auth-form input, #auth-form button");
        for (let index = 0; index < await controls.count(); index++) {
          const box = await controls.nth(index).boundingBox();
          expect(box?.height).toBeGreaterThanOrEqual(44);
        }
        await screenshot(page, testInfo, `setup-${viewport.width}.png`);
      }
      expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
      await page.getByLabel("Name", { exact: true }).fill(owner);
      await page.getByLabel("Password", { exact: true }).fill(password);
      await page.getByLabel("Confirm password").fill("another secure password");
      await page.getByRole("button", { name: "Create Owner & continue" }).click();
      await expect(page.getByRole("alert")).toHaveText("Passwords do not match.");
      await page.getByLabel("Confirm password").fill(password);
      await page.getByRole("button", { name: "Create Owner & continue" }).click();
      await expect(page.getByRole("dialog", { name: "Make the next sign-in easier" })).toBeVisible();
      await page.getByRole("button", { name: "Not now" }).click();
    }
    await expect(page).toHaveURL(/\/$/);
    await expect(page.getByText("No services configured yet.")).toBeVisible();
    await screenshot(page, testInfo, "setup-complete-empty-board.png");
  });

  test("keeps login controls usable at 720 by 450", async ({ page }, testInfo) => {
    await page.setViewportSize({ width: 720, height: 450 });
    await page.goto("/login");
    await expect(page.getByRole("heading", { name: "Kinosail Dashboard" })).toBeVisible();
    await expect(page.getByRole("button", { name: "Sign in with passkey" })).toBeVisible();
    await expect(page.locator(".login-shell .auth-form")).toBeVisible();
    const controls = page.locator("#auth-form input, #auth-form button");
    for (let index = 0; index < await controls.count(); index++) {
      const box = await controls.nth(index).boundingBox();
      expect(box?.height).toBeGreaterThanOrEqual(44);
    }
    await expectNoOverflow(page);
    await screenshot(page, testInfo, "login-720x450.png");
  });

  test("keeps passkey setup available in board settings", async ({ page }, testInfo) => {
    await page.setViewportSize({ width: 320, height: 700 });
    await login(page);
    await page.getByRole("button", { name: "Open board settings" }).click();
    const settings = page.getByRole("dialog", { name: "Settings and backup" });
    const passkeysHeading = settings.getByRole("heading", { name: "Passkeys" });
    await expect(passkeysHeading).toBeVisible();
    await expect(settings.getByRole("button", { name: "Add a passkey" })).toBeVisible();
    await expectNoOverflow(page);
    await passkeysHeading.scrollIntoViewIfNeeded();
    await screenshot(page, testInfo, "passkeys-settings-320.png");
  });

  test("offers a passkey after password sign-in at desktop and compact widths", async ({ page }, testInfo) => {
    await login(page, password, false);
    const settings = page.getByRole("dialog", { name: "Make the next sign-in easier" });
    for (const viewport of [{ width: 1440, height: 900 }, { width: 1024, height: 768 }, { width: 720, height: 450 }, { width: 390, height: 844 }, { width: 320, height: 700 }]) {
      await page.setViewportSize(viewport);
      await expect(settings.getByRole("heading", { name: "Passkeys" })).toBeVisible();
      await expect(settings.getByRole("button", { name: "Add a passkey" })).toBeFocused();
      await expect(settings.getByRole("button", { name: "Not now" })).toBeVisible();
      await expect(settings.getByLabel("Board name")).toBeHidden();
      await expect(settings.getByRole("heading", { name: "Owner password" })).toBeHidden();
      await expect(settings.getByRole("button", { name: "Save board" })).toBeHidden();
      expect(await settings.evaluate(dialog => dialog.scrollHeight <= dialog.clientHeight + 1)).toBe(true);
      await expectNoOverflow(page);
      expect((await new AxeBuilder({ page }).include("#settings-dialog").analyze()).violations).toEqual([]);
      await screenshot(page, testInfo, `passkey-offer-${viewport.width}.png`);
    }
    await settings.getByRole("button", { name: "Not now" }).click();
    await expect(page).toHaveURL(/\/$/);
    await page.getByRole("button", { name: "Open board settings" }).click();
    await expect(page.getByLabel("Board name")).toBeVisible();
    await expect(page.getByRole("heading", { name: "Owner password" })).toBeVisible();
    await expect(page.getByRole("button", { name: "Not now" })).toBeHidden();
  });

  test("registers and signs in with a passkey", { tag: "@chromium" }, async ({ page }) => {
    const client = await page.context().newCDPSession(page);
    await client.send("WebAuthn.enable");
    const { authenticatorId } = await client.send("WebAuthn.addVirtualAuthenticator", {
      options: { protocol: "ctap2", transport: "internal", hasResidentKey: true, hasUserVerification: true, isUserVerified: true },
    });
    try {
      await login(page);
      await page.getByRole("button", { name: "Open board settings" }).click();
      await page.getByRole("button", { name: "Add a passkey" }).click();
      await expect(page.getByText("Passkey added.")).toBeVisible();
      await expect.poll(() => page.evaluate(() => localStorage.getItem("kinosail-dashboard-passkey"))).toBe("1");
      const passkeyLogin = page.waitForResponse(response => response.url().endsWith("/api/v1/passkeys/login/finish") && response.status() === 204);
      await page.getByRole("button", { name: "Sign out" }).click();
      await passkeyLogin;
      await expect(page).toHaveURL(/\/$/);
      await expect(page.getByRole("heading", { name: "Home" })).toBeVisible();
    } finally {
      await client.send("WebAuthn.removeVirtualAuthenticator", { authenticatorId }).catch(() => {});
      await client.send("WebAuthn.disable").catch(() => {});
    }
  });
}
