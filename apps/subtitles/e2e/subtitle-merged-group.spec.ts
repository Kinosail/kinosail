import { readFile } from "node:fs/promises";
import { execFileSync } from "node:child_process";
import { expect, test } from "@playwright/test";

const fixtureDir = process.env.KINOSAIL_SUBTITLE_MERGED_FIXTURE_DIR;
const revision = process.env.KINOSAIL_TEST_REVISION ?? execFileSync("git", ["rev-parse", "HEAD"], { encoding: "utf8" }).trim();
test.skip(!fixtureDir, "requires the public 10,000-cue merged preview fixture");

for (const width of [390, 1440]) {
  test(`long merged comparison bounds rendering and reaches the last source at ${width}px`, { tag: "@smoke" }, async ({ page }, testInfo) => {
    const current = JSON.parse(await readFile(`${fixtureDir}/subtitle-pairing.json`, "utf8"));
    const proposed = JSON.parse(await readFile(`${fixtureDir}/subtitle-pairing-cleanup.json`, "utf8"));
    expect(current.current.cues).toHaveLength(10000);
    expect(proposed.comparison[0].current).toHaveLength(10000);
    const writes: string[] = [];
    await page.setViewportSize({ width, height: 900 });
    await page.route("http://merged.test/**", async route => {
      const path = new URL(route.request().url()).pathname;
      if (path.endsWith("/inspect")) return route.fulfill({ json: current });
      if (path.endsWith("/draft")) return route.fulfill({ json: { state: "none", words: [] } });
      if (path.endsWith("/preview")) { writes.push(path); return route.fulfill({ json: proposed }); }
      expect(route.request().method()).toBe("GET");
      if (path.startsWith("/media/")) return route.abort();
      const types: Record<string, string> = { "theme.js": "text/javascript", "app.css": "text/css", "subtitle-inspector.css": "text/css", "subtitle-inspector.js": "text/javascript", "icon.svg": "image/svg+xml", "manrope.woff2": "font/woff2" };
      const name = path.split("/").pop() || "";
      if (types[name]) return route.fulfill({ contentType: types[name], body: await readFile(`${fixtureDir}/${name}`) });
      if (path.startsWith("/static/")) return route.fulfill({ body: "" });
      return route.fulfill({ contentType: "text/html", body: await readFile(`${fixtureDir}/subtitle-pairing.html`) });
    });
    await page.goto("http://merged.test/subtitles/inspect/fixture");
    await expect(page.locator("#inspector-status")).toContainText("Current subtitle loaded");
    await page.locator('input[name="automaticSync"]').uncheck();
    await page.getByRole("button", { name: "Preview changes", exact: true }).click();
    await expect(page.locator("#inspector-status")).toContainText("Preview ready");
    await expect(page.locator(".subtitle-cue-row")).toHaveCount(1);
    const metrics = await page.locator("#subtitle-cues").evaluate(root => ({ seekButtons: root.querySelectorAll('button[aria-label^="Seek "]').length, elements: root.querySelectorAll("*").length, textBytes: root.textContent?.length || 0 }));
    await testInfo.attach("long-group-initial-render", { contentType: "application/json", body: JSON.stringify({ revision, width, metrics, sourceCount: 10000, boundary: "isolated renderer; actual public Go 10,000-cue inspect/preview/assets; media aborted; no browser performance timing claim" }) });
    expect(metrics.seekButtons).toBeLessThanOrEqual(80);
    expect(metrics.elements).toBeLessThanOrEqual(1000);
    expect(metrics.textBytes).toBeLessThanOrEqual(4000);
    await page.getByRole("button", { name: "Review 10000 Current source cues", exact: true }).click();
    const sourcePage = page.getByRole("spinbutton", { name: "Current source page", exact: true });
    await sourcePage.fill(await sourcePage.getAttribute("max") || "");
    await sourcePage.press("Enter");
    await page.getByRole("button", { name: "Seek Current cue 10000: 166:39.000 to 166:40.000", exact: true }).click();
    await expect(page.locator('input[name="preview-track"][value="current"]')).toBeChecked();
    await expect.poll(() => page.locator("video").evaluate(video => video.currentTime)).toBe(9998);
    await expect(page.locator("video")).toBeFocused();
    expect(await page.locator('#subtitle-cues button[aria-label^="Seek "]').count()).toBeLessThanOrEqual(80);
    await page.getByRole("button", { name: "Close Current source cues", exact: true }).click();
    await expect(page.getByRole("button", { name: "Review 10000 Current source cues", exact: true })).toBeFocused();
    expect(writes).toHaveLength(1);
    expect(await page.evaluate(() => document.documentElement.scrollWidth - innerWidth)).toBeLessThanOrEqual(1);
    await page.screenshot({ path: testInfo.outputPath("long-group-bounded.png") });
  });
}
