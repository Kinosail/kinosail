import AxeBuilder from "@axe-core/playwright";
import { createHash } from "node:crypto";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { expect, test } from "@playwright/test";
import { createViewer, firstPlayable, login, loginViewer, newViewerPage, openLibrarySection, removeViewer, type OfflineClient } from "./test-instance-helpers";

export function registerTestInstanceSupporterTests() {
test.describe("Supporter populated fixtures", () => {
test.use({ serviceWorkers: "block" });
test("Supporter populated page shares a social PNG with safe fallbacks", async ({ page }, testInfo) => {
  const directory = process.env.KINOSAIL_UI_FIXTURE_DIR;
  test.skip(!directory, "requires exact production-template fixtures");
  const supporterPage = await readFile(join(directory!, "supporter-populated-active.html"), "utf8");
  const certificatePath = "/api/v1/supporter/certificates/living-standard";
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.route(`**${certificatePath}`, async (route) => {
    await new Promise((resolve) => setTimeout(resolve, 150));
    await route.fulfill({
      status: 200,
      contentType: "image/svg+xml",
      body: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1200 630"><rect width="1200" height="630" fill="#090a08"/><text x="40" y="80" fill="#c8f169">Kinosail Subtitles</text></svg>',
    });
  });
  await login(page);
  await page.route((url) => url.pathname === "/supporter", (route) => route.fulfill({ status: 200, contentType: "text/html", body: supporterPage }));
  await page.goto("/supporter");
  await expect(page.locator(".owned-grant.is-active").first()).toContainText("Rosetta Crown");
  await expect(page.locator(".subtitle-masterwork")).toContainText("Perfect Sync");
  await expect(page.locator(".subtitle-masterwork")).toContainText("Rosetta Ascendant");
  await expect(page.locator(".badge-level.collected")).toHaveCount(17);
  await expect(page.getByLabel("Subscriber service marks").locator("span")).toHaveCount(6);
  await expect(page.getByText("Complete Fleet · Living")).toBeVisible();
  await page.screenshot({ path: testInfo.outputPath("1440-supporter-populated-active.png"), fullPage: true });
  await page.setViewportSize({ width: 320, height: 800 });
  await page.screenshot({ path: testInfo.outputPath("320-supporter-populated-active.png"), fullPage: true });
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.evaluate(() => {
    type ShareWindow = Window & {
      __shareEvidence?: { name: string; type: string; width: number; height: number };
    };
    const shareWindow = window as ShareWindow;
    Object.defineProperty(navigator, "canShare", { configurable: true, value: (data: ShareData) => data.files?.[0]?.type === "image/png" });
    Object.defineProperty(navigator, "share", { configurable: true, value: async (data: ShareData) => {
      const file = data.files?.[0];
      if (!file) throw new Error("missing shared file");
      const bitmap = await createImageBitmap(file);
      shareWindow.__shareEvidence = { name: file.name, type: file.type, width: bitmap.width, height: bitmap.height };
      bitmap.close();
    } });
  });
  const livingGrant = page.locator(".owned-grant").filter({ hasText: "Living Standard" });
  const share = livingGrant.getByRole("button", { name: "Share PNG certificate" });
  const shareStatus = livingGrant.locator(".share-status");
  await share.click();
  await expect(share).toBeDisabled();
  await expect(shareStatus).toHaveText("Preparing share image…");
  await expect.poll(() => page.evaluate(() => (window as Window & { __shareEvidence?: object }).__shareEvidence)).toEqual({
    name: "kinosail-subtitles-living-standard-legacy.png", type: "image/png", width: 1200, height: 630,
  });
  await expect(shareStatus).toHaveText("Certificate shared.");
  await expect(share).toBeEnabled();
  await expect(livingGrant.getByRole("link", { name: "Download SVG certificate" })).toHaveAttribute("href", certificatePath);

  await page.evaluate(() => Object.defineProperty(navigator, "share", { configurable: true, value: async () => { throw new Error("share unavailable"); } }));
  const failedShareDownload = page.waitForEvent("download");
  await share.click();
  expect((await failedShareDownload).suggestedFilename()).toBe("kinosail-subtitles-living-standard-legacy.png");
  await expect(shareStatus).toHaveText("PNG certificate download started.");
  await expect(share).toBeEnabled();

  await page.evaluate(() => Object.defineProperty(navigator, "share", { configurable: true, value: async () => { throw new DOMException("cancelled", "AbortError"); } }));
  await share.click();
  await expect(shareStatus).toHaveText("");
  await expect(share).toBeEnabled();

  await page.evaluate(() => Object.defineProperty(navigator, "share", { configurable: true, value: undefined }));
  const downloadStarted = page.waitForEvent("download");
  await share.click();
  const download = await downloadStarted;
  expect(download.suggestedFilename()).toBe("kinosail-subtitles-living-standard-legacy.png");
  await expect(shareStatus).toHaveText("PNG certificate download started.");
});

test("Supporter populated archive never implies an active entitlement", async ({ page }, testInfo) => {
  const directory = process.env.KINOSAIL_UI_FIXTURE_DIR;
  test.skip(!directory, "requires exact production-template fixtures");
  const supporterPage = await readFile(join(directory!, "supporter-populated-archived.html"), "utf8");
  await login(page);
  await page.route((url) => url.pathname === "/supporter", (route) => route.fulfill({ status: 200, contentType: "text/html", body: supporterPage }));
  for (const viewport of [{ width: 1440, height: 900 }, { width: 320, height: 800 }]) {
    await page.setViewportSize(viewport);
    await page.goto("/supporter");
    const archived = page.locator(".owned-grant.is-archived");
    await expect(archived).toContainText("Active through");
    await expect(page.locator(".family-living-standard .badge-level.current")).toContainText("Archived");
    await expect(page.getByLabel("Subscriber service marks").locator("span")).toHaveCount(4);
    await expect(page.locator(".family-living-standard .badge-level.current")).not.toContainText("Yours");
    await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-supporter-populated-archived.png`), fullPage: true });
  }
});
});

test("a stale login page redirects passkey sign-in to the canonical origin", async ({ page }) => {
  await page.addInitScript(() => Object.defineProperty(navigator, "credentials", { value: { get: async () => { throw new DOMException("no test passkey", "NotAllowedError"); } } }));
  await page.goto("/login");
  await expect(page.locator('meta[name="kinosail-csrf"]')).toHaveCount(0);
  const login = new URL(page.url());
  await page.context().addCookies([{ name: login.protocol === "https:" ? "__Host-kinosail_session" : "kinosail_session", value: "stale-test-session", url: new URL("/", login).toString(), httpOnly: true, secure: login.protocol === "https:", sameSite: "Strict" }]);
  const response = page.waitForResponse((candidate) => candidate.url().endsWith("/api/v1/passkeys/login/begin") && candidate.request().method() === "POST");
  await page.getByRole("button", { name: "Sign in with passkey" }).click();
  const begin = await response;
  expect(begin.status()).toBe(421);
  expect(begin.headers().location).toBe("https://localhost:38128/login");
});
}
