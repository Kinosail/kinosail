import { readFile } from "node:fs/promises";
import { expect, test } from "@playwright/test";
import { login } from "./test-instance-helpers";

test.use({ serviceWorkers: "block" });

const origin = "http://localhost:38127";
const assets = new URL("../../../packages/webassets/static/", import.meta.url);
const [appearance, theme, css, pwa] = await Promise.all([
  "appearance.js", "theme.js", "player-app.css", "pwa.js",
].map((name) => readFile(new URL(name, assets), "utf8")));

test("mobile title jump starts with its compact control while navigation loads", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 650 });
  let releaseNavigation!: () => void;
  const navigationHeld = new Promise<void>((resolve) => { releaseNavigation = resolve; });
  await page.route(`${origin}/**`, async (route) => {
    const path = new URL(route.request().url()).pathname;
    if (path === "/static/theme.js") return route.fulfill({ contentType: "text/javascript", body: appearance + theme });
    if (path === "/static/app.css") return route.fulfill({ contentType: "text/css", body: css });
    if (path === "/static/pwa.js") {
      await navigationHeld;
      return route.fulfill({ contentType: "text/javascript", body: pwa });
    }
    if (path === "/service-worker.js") return route.fulfill({ status: 404 });
    return route.fulfill({ contentType: "text/html", body: `<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width"><script src="/static/theme.js"></script><link rel="stylesheet" href="/static/app.css"><script defer src="/static/pwa.js"></script></head><body class="library-page"><main id="main" class="library-shell"><div class="title-jump" data-title-jump><button class="quiet title-jump-open" type="button" data-title-jump-open>Jump to title</button><nav class="letter-jump" data-title-jump-index><a href="/?letter=A" data-title-letter="A">A</a><a href="/?letter=B" data-title-letter="B">B</a><a href="/?letter=C" data-title-letter="C">C</a><a href="/?letter=D" data-title-letter="D">D</a></nav><dialog data-title-jump-dialog><nav><a href="/?letter=A" data-title-letter="A">A</a></nav></dialog><output data-title-jump-preview hidden></output></div><section id="library"></section></main></body></html>` });
  });

  await page.goto(origin, { waitUntil: "commit" });
  await expect(page.getByRole("button", { name: "Jump to title" })).toBeVisible();
  await expect(page.locator("[data-title-jump-index]")).toBeHidden();
  releaseNavigation();
  await expect(page.locator("html")).toHaveClass(/\bjs\b/);
  await page.getByRole("button", { name: "Jump to title" }).click();
  await expect(page.locator("[data-title-jump-dialog]")).toBeVisible();
});

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

  await page.goto("/?view=movies", { waitUntil: "commit" });
  await expect(page.getByRole("button", { name: "Jump to title" })).toBeVisible();
  await expect(page.locator("[data-title-jump-index]")).toBeHidden();
  await page.screenshot({ path: testInfo.outputPath("mobile-title-jump-before-navigation.png") });
  releaseNavigation();
  await page.waitForLoadState("domcontentloaded");
  await page.getByRole("button", { name: "Jump to title" }).click();
  await expect(page.getByRole("dialog", { name: "Jump to title" })).toBeVisible();
  await page.screenshot({ path: testInfo.outputPath("mobile-title-jump-dialog.png") });
});
