import { expect, test } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { execFileSync } from "node:child_process";
import { expectNoHorizontalOverflow } from "./subtitle-dashboard-helpers";

test.use({ serviceWorkers: "block" });
test.beforeEach(async ({ page }) => {
  const source = await readFile(new URL("../internal/server/static/subtitle-status.js", import.meta.url), "utf8");
  await page.route("**/static/subtitle-status.js?*", route => route.fulfill({ contentType: "text/javascript", body: source }));
});

test.afterEach(async ({ page }, testInfo) => {
  const evidence = {
    revision: process.env.GITHUB_SHA || execFileSync("git", ["rev-parse", "HEAD"], { encoding: "utf8" }).trim(),
    command: process.argv.join(" "),
    testData: "Production history templates: new sidecar, score upgrade, exact hash, embedded track, restore, manual edit, v2 migration, v3 history",
    environment: { platform: process.platform, arch: process.arch, browser: testInfo.project.name, version: page.context().browser()?.version() },
    result: testInfo.status,
  };
  await testInfo.attach("history-evidence", { body: Buffer.from(JSON.stringify(evidence, null, 2)), contentType: "application/json" });
});

const directory = process.env.KINOSAIL_UI_FIXTURE_DIR;
test.skip(!directory, "requires production-template history fixtures");
const fixture = (state: string) => readFile(join(directory!, `subtitle-history-${state}.html`), "utf8");

test("history explains new files, automatic upgrades, manual edits, restores, and older records @smoke", async ({ page }, testInfo) => {
  await page.route(/\/\?view=history$/, async route => route.fulfill({ contentType: "text/html", body: await fixture("populated") }));
  for (const width of [1440, 1024, 390, 320]) {
    await page.setViewportSize({ width, height: width >= 1024 ? 900 : 844 });
    for (const theme of ["light", "dark"]) {
      await page.goto("/?view=history");
      await page.evaluate(value => { document.documentElement.dataset.theme = value; }, theme);
      const rows = page.locator(".subtitle-history-row");
      await expect(rows).toHaveCount(8);
      await expect(rows.nth(0)).toContainText("Added a new subtitle file");
      await expect(rows.nth(1)).toContainText("Upgraded");
      await expect(rows.nth(1)).toContainText("improved by 20 points");
      await expect(rows.nth(1)).toContainText("72 → 92 / 100");
      await expect(rows.nth(1)).toContainText("75% → 100%");
      await expect(rows.nth(1)).toContainText("SubDL → SubSource");
      await expect(rows.nth(2)).toContainText("exact file-hash match");
      await expect(rows.nth(2)).toContainText("no verified match score");
      await expect(rows.nth(3)).toContainText("Copied an embedded text track");
      await expect(rows.nth(4)).toContainText("recovery copy");
      await expect(rows.nth(5)).toContainText("Saved manually");
      await expect(rows.nth(6)).toContainText("Recorded");
      await expect(rows.nth(6)).toContainText("reason for this earlier change was not recorded");
      await expect(rows.nth(7)).toContainText("whether it added or replaced a file was not recorded");
      await expect(page.locator("#subtitle-loading")).toBeHidden();
      await expectNoHorizontalOverflow(page);
      const geometry = await rows.nth(1).evaluate(row => {
        const title = row.querySelector(".subtitle-history-title")!.getBoundingClientRect();
        const change = row.querySelector(".subtitle-history-change")!.getBoundingClientRect();
        return { display: getComputedStyle(row).display, separated: title.right <= change.left + 1 || title.bottom <= change.top + 1 };
      });
      expect(geometry).toEqual({ display: "grid", separated: true });
      expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
      await page.screenshot({ path: testInfo.outputPath(`history-${width}-${theme}.png`), fullPage: true });
    }
  }
});

test("a browser with the previous immutable stylesheet receives the new history layout", async ({ page }) => {
  let oldStylesheetRequests = 0;
  await page.route("**/static/app.css?v=cinema-9", async route => {
    oldStylesheetRequests += 1;
    await route.fulfill({ contentType: "text/css", headers: { "Cache-Control": "public, max-age=31536000, immutable" }, body: "body{color:#fff;background:#111}" });
  });
  await page.route("**/history-cache-seed", route => route.fulfill({ contentType: "text/html", body: '<link rel="stylesheet" href="/static/app.css?v=cinema-9">Previous version' }));
  await page.goto("/history-cache-seed");
  expect(oldStylesheetRequests).toBeGreaterThanOrEqual(1);
  const warmedRequests = oldStylesheetRequests;
  await page.route(/\/\?view=history$/, async route => route.fulfill({ contentType: "text/html", body: await fixture("populated") }));
  await page.goto("/?view=history");
  await expect(page.locator('link[rel="stylesheet"]').first()).not.toHaveAttribute("href", "/static/app.css?v=cinema-9");
  expect(oldStylesheetRequests).toBe(warmedRequests);
  await expect(page.locator(".subtitle-history-row").first()).toHaveCSS("display", "grid");
});

test("history keeps loaded changes during pending and failed navigation and clears them for empty results", async ({ page }, testInfo) => {
  let release!: () => void;
  const hold = new Promise<void>(resolve => { release = resolve; });
  let fail = true;
  await page.route(/\/\?view=history(?:&page=1)?$/, async route => {
    if (new URL(route.request().url()).searchParams.has("page")) {
      await route.fulfill({ contentType: "text/html", body: await fixture("populated") });
    } else if (fail) {
      await hold;
      await route.fulfill({ status: 503, body: "History unavailable" });
    } else {
      await route.fulfill({ contentType: "text/html", body: await fixture("empty") });
    }
  });
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/?view=history&page=1");
  const before = await page.locator(".subtitle-history-row").first().boundingBox();
  await page.getByRole("link", { name: "History", exact: true }).click();
  await expect(page.locator("#main")).toHaveAttribute("aria-busy", "true");
  await expect(page.locator("#subtitle-feedback")).toContainText("Loading history");
  await expect(page.locator(".subtitle-history-row")).toHaveCount(8);
  expect((await page.locator(".subtitle-history-row").first().boundingBox())?.height).toBe(before?.height);
  await page.screenshot({ path: testInfo.outputPath("history-pending.png"), fullPage: true });
  release();
  await expect(page.locator("#subtitle-feedback")).toContainText("Could not load this view");
  await expect(page.locator("#main")).not.toHaveAttribute("aria-busy", "true");
  await expect(page.locator(".subtitle-history-row")).toHaveCount(8);
  await page.screenshot({ path: testInfo.outputPath("history-failed.png"), fullPage: true });
  fail = false;
  await page.getByRole("link", { name: "Reload view" }).click();
  await expect(page.getByRole("heading", { name: "No subtitle changes yet." })).toBeVisible();
  await expect(page.locator(".subtitle-history-row")).toHaveCount(0);
  await expect(page.locator("#subtitle-feedback")).toBeHidden();
  await expect(page.locator("#subtitle-loading")).toBeHidden();
  await page.screenshot({ path: testInfo.outputPath("history-empty.png"), fullPage: true });
  await page.unroute(/\/\?view=history(?:&page=1)?$/);
  await page.route(/\/\?view=history$/, async route => route.fulfill({ contentType: "text/html", body: await fixture("failed") }));
  await page.goto("/?view=history");
  await expect(page.getByRole("alert")).toContainText("Subtitle history is unavailable");
  await expect(page.locator("#subtitle-loading")).toBeHidden();
});
