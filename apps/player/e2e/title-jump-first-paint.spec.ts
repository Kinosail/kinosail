import { expect, test } from "@playwright/test";
import { login } from "./test-instance-helpers";

test.use({ serviceWorkers: "block" });

test("populated mobile library keeps the compact title jump while navigation loads", async ({ page }, testInfo) => {
  test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
  await login(page);
  await page.setViewportSize({ width: 390, height: 650 });
  let releaseNavigation!: () => void;
  const navigationHeld = new Promise<void>((resolve) => { releaseNavigation = resolve; });
  await page.route("**/static/main.kinosail.bundle.js*", async (route) => {
    await navigationHeld;
    await route.continue();
  });

  try {
    await page.goto("/?view=movies", { waitUntil: "commit" });
    await expect(page.getByRole("button", { name: "Jump to title" })).toBeVisible();
    await expect(page.locator("[data-title-jump-index]")).toBeHidden();
    await testInfo.attach("first-paint-before-navigation", { body: JSON.stringify({
      navigationHeld: true, compactControlVisible: true, expandedIndexHidden: true,
      viewport: page.viewportSize(), revision: process.env.KINOSAIL_TEST_REVISION,
    }), contentType: "application/json" });
  } finally {
    // Screenshots wait for fonts; their readiness depends on deferred scripts.
    releaseNavigation();
  }
  await page.waitForLoadState("domcontentloaded");
  await page.getByRole("button", { name: "Jump to title" }).click();
  await expect(page.getByRole("dialog", { name: "Jump to title" })).toBeVisible();
  await page.screenshot({ path: testInfo.outputPath("mobile-title-jump-dialog.png") });
});
