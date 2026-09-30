import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { writeFile } from "node:fs/promises";
import { totp } from "./happy-path-helpers";

// Each browser needs a new isolated Server with no Owner account.
test.describe.configure({ retries: 0 });
test("fresh install protects the Owner and reaches playback and a new sign-in", async ({ page, browserName }, info) => {
  test.skip(process.env.KINOSAIL_FRESH_INSTALL !== "1", "requires a dedicated empty Server");
  test.setTimeout(120_000);
  let secret = "";
  if (browserName === "chromium") {
    await page.addInitScript(() => Object.defineProperty(PublicKeyCredential, "isConditionalMediationAvailable", { value: async () => false }));
    const client = await page.context().newCDPSession(page);
    await client.send("WebAuthn.enable");
    await client.send("WebAuthn.addVirtualAuthenticator", { options: { protocol: "ctap2", transport: "internal", hasResidentKey: true, hasUserVerification: true, isUserVerified: true, automaticPresenceSimulation: true } });
  }
  await page.goto("/setup");
  await expect(page.getByRole("heading", { name: "Set up your Server." })).toBeVisible();
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
  await page.getByLabel("Name").fill("Owner");
  await page.locator("#new-password").fill("test-first-install-password");
  await page.getByLabel(/Add extra sign-in protection now/).uncheck();
  await page.getByRole("button", { name: "Create Owner & continue" }).click();
  await expect(page.getByRole("heading", { name: "Protect the Owner account" })).toBeVisible();
  if (browserName === "chromium") {
    await page.getByRole("button", { name: "Create passkey" }).click();
  } else {
    await page.getByRole("button", { name: "Use an authenticator app instead" }).click();
    secret = (await page.locator("code").first().textContent())!;
    await page.getByLabel("Authentication code").fill(totp(secret));
    await page.getByRole("button", { name: "Turn on extra sign-in protection" }).click();
  }
  await expect(page).toHaveURL("/onboarding/connection");
  await expect(page.getByRole("heading", { name: "Secure local access" })).toBeVisible();
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
  await page.getByRole("link", { name: "Continue to household setup" }).click();
  await page.getByRole("link", { name: "Continue to optional viewing history" }).click();
  await page.getByRole("link", { name: "Finish and open Library" }).click();
  await expect(page).toHaveURL("/");
  await page.getByRole("link", { name: "Movies", exact: true }).click();
  await page.getByRole("link", { name: /Example Movie/ }).click();
  const media = page.locator("video");
  await expect(media).toBeVisible();
  await media.evaluate(async (video: HTMLVideoElement) => { video.muted = true; await video.play(); });
  await expect.poll(() => media.evaluate((video: HTMLVideoElement) => video.currentTime)).toBeGreaterThan(0.5);
  expect(await media.evaluate((video: HTMLVideoElement) => video.error?.message ?? null)).toBeNull();
  await page.goto("/");
  await page.evaluate(() => localStorage.removeItem("kinosail-passkey"));
  await page.setViewportSize({ width: 390, height: 844 });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  const signOut = page.getByRole("button", { name: "Sign out" });
  if (!await signOut.isVisible()) await page.locator(".nav-more > summary").click();
  await signOut.click();
  await expect(page).toHaveURL(/\/login/);
  if (browserName === "chromium") {
    await page.getByRole("button", { name: "Sign in with passkey" }).click();
  } else {
    await page.getByLabel("Name").fill("Owner");
    await page.getByLabel("Password", { exact: true }).fill("test-first-install-password");
    await page.getByLabel("Authentication or recovery code").fill(totp(secret));
    await page.getByRole("button", { name: "Sign in", exact: true }).click();
  }
  await expect(page).not.toHaveURL(/\/login/);
  await expect(page.getByRole("main")).toBeVisible();
  const receipt = info.outputPath("first-install-receipt.json");
  await writeFile(receipt, JSON.stringify({ revision: process.env.KINOSAIL_TEST_REVISION,
    command: "KINOSAIL_FRESH_INSTALL=1 pnpm exec playwright test test-instance-first-install.spec.ts --workers=1",
    environment: `Nox ARM64 isolated production image; ${browserName}`,
    testData: "Fresh Owner, synthetic media, virtual Chromium passkey or real TOTP validation",
    result: "passed" }, null, 2));
  await info.attach("first-install-receipt", { path: receipt, contentType: "application/json" });
});
