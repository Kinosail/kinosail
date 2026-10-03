import { execFileSync } from "node:child_process";
import { readFile } from "node:fs/promises";
import { expect, test } from "@playwright/test";

const fixtureDir = process.env.KINOSAIL_UI_FIXTURE_DIR;
const revision = process.env.KINOSAIL_TEST_REVISION ?? execFileSync("git", ["rev-parse", "HEAD"], { encoding: "utf8" }).trim();
test.skip(!fixtureDir, "requires server-rendered subtitle inspector fixtures");
test.use({ viewport: { width: 390, height: 844 }, ignoreHTTPSErrors: false });

for (const failed of [false, true]) {
  test(`known inspector content stays stable during a ${failed ? "failed" : "successful"} refresh`, { tag: "@smoke" }, async ({ page }, testInfo) => {
    const source = await readFile(`${fixtureDir}/subtitle-inspector.html`, "utf8");
    const review = JSON.parse(await readFile(`${fixtureDir}/subtitle-inspector.json`, "utf8"));
    let release!: () => void;
    const pending = new Promise<void>(resolve => { release = resolve; });
    await page.route("http://inspector.test/**", async route => {
      const path = new URL(route.request().url()).pathname;
      if (path.endsWith("/inspect")) {
        await pending;
        return route.fulfill(failed ? { status: 503, json: { error: "Subtitle details are unavailable. Reload and try again." } } : { json: review });
      }
      if (path.endsWith("/draft")) return route.fulfill({ json: { state: "idle", words: [] } });
      if (["/static/app.css", "/static/subtitle-inspector.css", "/static/subtitle-inspector.js"].includes(path)) {
        return route.fulfill({ contentType: path.endsWith("css") ? "text/css" : "text/javascript", body: await readFile(`${fixtureDir}/${path.split("/").pop()}`, "utf8") });
      }
      if (path.startsWith("/static/") || path.startsWith("/media/")) return route.fulfill({ body: "" });
      return route.fulfill({ contentType: "text/html", body: source });
    });
    await page.goto("http://inspector.test/subtitles/inspect/fixture");
    const status = page.locator("#inspector-status"), workspace = page.locator(".subtitle-inspector-workspace");
    const file = page.locator('input[type="file"]'), preview = page.locator('#subtitle-edit-form button[type="submit"]');
    await expect(status).toHaveAttribute("aria-busy", "true");
    await expect(file).toBeDisabled();
    await expect(preview).toBeDisabled();
    await expect(page.locator(".subtitle-cue-row")).toHaveCount(2);
    const before = await workspace.boundingBox();
    await page.screenshot({ path: testInfo.outputPath("pending.png") });
    release();
    await expect(status).not.toHaveAttribute("aria-busy", "true");
    await expect(file).toBeEnabled();
    if (failed) await expect(preview).toBeDisabled(); else await expect(preview).toBeEnabled();
    await expect(page.locator(".subtitle-cue-row")).toHaveCount(2);
    await expect(status).toHaveText(failed ? "Subtitle details are unavailable. Reload and try again." : "Current subtitle loaded. Preview a change before saving.");
    expect(await workspace.boundingBox()).toEqual(before);
    await page.screenshot({ path: testInfo.outputPath("settled.png") });
    await testInfo.attach("verification-context", { contentType: "application/json", body: JSON.stringify({ revision, command: "playwright test subtitle-inspector-loading.spec.ts", data: "Server-rendered Arrival review, two installed cues; synthetic delayed API and labelled 503", environment: testInfo.project.name, failed, before, after: await workspace.boundingBox(), result: testInfo.status }) });
  });
}
