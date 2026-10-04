import { readFile } from "node:fs/promises";
import { execFileSync } from "node:child_process";
import { reviewSubtitlePairing } from "./subtitle-pairing-journey";
import { expect, test } from "@playwright/test";

const fixtureDir = process.env.KINOSAIL_UI_FIXTURE_DIR;
const revision = process.env.KINOSAIL_TEST_REVISION ?? execFileSync("git", ["rev-parse", "HEAD"], { encoding: "utf8" }).trim();
test.skip(!fixtureDir, "requires TestWriteUIStateFixturesSubtitlePairingPreservesDialogueCorrespondenceThroughCleanup artifacts");

for (const width of [390, 1440]) {
  test(`cleanup compares the same dialogue at ${width}px`, { tag: "@smoke" }, async ({ page }, testInfo) => {
    const source = await readFile(`${fixtureDir}/subtitle-pairing.html`, "utf8");
    const current = JSON.parse(await readFile(`${fixtureDir}/subtitle-pairing.json`, "utf8"));
    const cleanup = JSON.parse(await readFile(`${fixtureDir}/subtitle-pairing-cleanup.json`, "utf8"));
    const imported = JSON.parse(await readFile(`${fixtureDir}/subtitle-pairing-import.json`, "utf8"));
    const previews: unknown[] = [];
    await testInfo.attach("verification-context", { contentType: "application/json", body: JSON.stringify({ revision, width, command: "playwright test subtitle-pairing.spec.ts --workers=1", environment: testInfo.project.name, browserVersion: page.context().browser()?.version(), data: "actual rendered inspector and API preview outputs for synthetic credit/repeated/later dialogue and unrelated import", boundary: "isolated browser transport/render fixture; no playable media or populated Server" }) });
    await page.setViewportSize({ width, height: 900 });
    await page.route("http://pairing.test/**", async route => {
      const path = new URL(route.request().url()).pathname;
      if (path.endsWith("/inspect")) return route.fulfill({ json: current });
      if (path.endsWith("/draft")) return route.fulfill({ json: { state: "none", words: [] } });
      if (path.endsWith("/preview")) {
        previews.push(route.request().postDataJSON());
        return route.fulfill({ json: previews.length === 1 ? cleanup : imported });
      }
      expect(route.request().method(), "preview must not save or restore a sidecar").toBe("GET");
      if (path.startsWith("/media/")) return route.abort();
      if (path === "/static/theme.js") {
        // Saved baseline assets predate this shell dependency capture; preserve their empty-script boundary.
        const body = await readFile(`${fixtureDir}/theme.js`).catch(error => { if (error.code === "ENOENT") return ""; throw error; });
        return route.fulfill({ contentType: "text/javascript", body });
      }
      const types: Record<string, string> = { "app.css": "text/css", "subtitle-inspector.css": "text/css", "subtitle-inspector.js": "text/javascript", "icon.svg": "image/svg+xml", "manrope.woff2": "font/woff2" };
      const name = path.split("/").pop() || "";
      if (types[name]) return route.fulfill({ contentType: types[name], body: await readFile(`${fixtureDir}/${name}`) });
      if (path.startsWith("/static/")) return route.fulfill({ body: "" });
      return route.fulfill({ contentType: "text/html", body: source });
    });
    await page.goto("http://pairing.test/subtitles/inspect/fixture");
    await reviewSubtitlePairing(page, testInfo);
    await testInfo.attach("verification-context", { contentType: "application/json", body: JSON.stringify({ revision, width, command: "playwright test subtitle-pairing.spec.ts", environment: testInfo.project.name, data: "actual rendered inspector and API preview outputs for synthetic credit/repeated/later dialogue and unrelated import", boundary: "isolated browser transport/render fixture; no playable media or populated Server", previews }) });
  });
}
