import { execFileSync } from "node:child_process";
import AxeBuilder from "@axe-core/playwright";
import { readFile } from "node:fs/promises";
import { expect, test } from "@playwright/test";

const revision = process.env.KINOSAIL_TEST_REVISION ?? execFileSync("git", ["rev-parse", "HEAD"], { encoding: "utf8" }).trim();
const fixtureDir = process.env.KINOSAIL_UI_FIXTURE_DIR;
test.skip(!fixtureDir, "requires TestWriteUIStateFixturesSubtitleInspector output");
const viewports = [{ width: 1920, height: 1080 }, { width: 1440, height: 900 }, { width: 1024, height: 768 }, { width: 720, height: 450 }, { width: 568, height: 320 }, { width: 390, height: 844 }, { width: 320, height: 800 }];

for (const theme of ["dark", "light"]) for (const viewport of viewports) {
  test(`inspector shell and review states fit ${viewport.width}px ${theme}`, { tag: theme === "dark" && [1440, 390].includes(viewport.width) ? "@smoke" : [] }, async ({ page }, testInfo) => {
    const source = await readFile(`${fixtureDir}/subtitle-inspector.html`, "utf8");
    const review = JSON.parse(await readFile(`${fixtureDir}/subtitle-inspector.json`, "utf8"));
    let state = "pending";
    let release!: () => void;
    const pending = new Promise<void>(resolve => { release = resolve; });
    const writes: string[] = [];
    await page.setViewportSize(viewport);
    await page.route("http://inspector.test/**", async route => {
      const url = new URL(route.request().url());
      if (route.request().method() !== "GET") writes.push(url.pathname);
      if (url.pathname.endsWith("/inspect")) {
        if (state === "pending") await pending;
        return state === "failed" ? route.fulfill({ status: 503, json: { error: "Subtitle details are unavailable. Reload and try again." } }) : route.fulfill({ json: state === "empty" ? { ...review, current: null } : review });
      }
      if (url.pathname.endsWith("/draft")) return route.fulfill({ json: { state: "idle", words: [] } });
      if (url.pathname.startsWith("/media/")) return route.abort();
      if (["/static/app.css", "/static/subtitle-inspector.css", "/static/subtitle-inspector.js"].includes(url.pathname)) {
        return route.fulfill({ contentType: url.pathname.endsWith("css") ? "text/css" : "text/javascript", body: await readFile(`${fixtureDir}/${url.pathname.split("/").pop()}`, "utf8") });
      }
      const mediaTypes: Record<string, string> = { "/static/icon.svg": "image/svg+xml", "/static/manrope.woff2": "font/woff2", "/static/cinema-backdrop.jpg": "image/jpeg" };
      if (mediaTypes[url.pathname]) return route.fulfill({ contentType: mediaTypes[url.pathname], body: await readFile(`${fixtureDir}/${url.pathname.split("/").pop()}`) });
      if (url.pathname.startsWith("/static/")) return route.fulfill({ body: "" });
      return route.fulfill({ contentType: "text/html", body: source.replace("<html ", `<html data-theme="${theme}" `) });
    });
    await page.goto("http://inspector.test/subtitles/inspect/fixture");
    const status = page.locator("#inspector-status");
    await expect(status).toHaveAttribute("aria-busy", "true");
    const video = page.locator("#subtitle-preview-video");
    const pendingVideo = await video.boundingBox();
    await page.screenshot({ path: testInfo.outputPath("pending.png") });
    state = "loaded"; release();
    await expect(status).toHaveText("Current subtitle loaded. Preview a change before saving.");
    await expect(status).not.toHaveAttribute("aria-busy", "true");
    await expect(page.locator(".subtitle-cue-row")).toHaveCount(2);
    await expect(page.getByRole("heading", { name: "Arrival", exact: true })).toBeVisible();
    const layout = await page.evaluate(() => {
      const box = (selector: string) => document.querySelector(selector)!.getBoundingClientRect().toJSON();
      return { header: box(".app-header"), title: box(".subtitle-topbar h1"), nav: box(".app-header nav"), main: box(".subtitle-main"), video: box("video"), overflow: document.documentElement.scrollWidth - innerWidth, offenders: [...document.querySelectorAll<HTMLElement>("body *")].filter(element => { const rect = element.getBoundingClientRect(); return rect.height > 0 && (rect.right > innerWidth + 1 || element.scrollWidth > element.clientWidth + 1); }).map(element => ({ tag: element.tagName, class: element.className, right: element.getBoundingClientRect().right, width: getComputedStyle(element).width, minWidth: getComputedStyle(element).minWidth, scrollWidth: element.scrollWidth, clientWidth: element.clientWidth })) };
    });
    expect(layout.header.width).toBeCloseTo(viewport.width, 0);
    expect(layout.header.height).toBeLessThanOrEqual(100);
    expect(layout.title.top).toBeGreaterThanOrEqual(layout.header.bottom);
    expect(layout.overflow, JSON.stringify(layout.offenders)).toBeLessThanOrEqual(1);
    expect(layout.video.height).toBeGreaterThanOrEqual(Math.min(layout.video.width * 9 / 16, viewport.height * .6) - 2);
    expect(layout.video.height).toBeCloseTo(pendingVideo!.height, 0);
    if (viewport.width > 1100) expect(layout.main.left).toBeGreaterThanOrEqual(layout.nav.right);
    if (viewport.width <= 900) expect(layout.nav.bottom).toBeCloseTo(viewport.height, 0);
    await expect(page.getByRole("link", { name: "History", exact: true })).toBeVisible();
    await expect(page.getByRole("link", { name: "Library", exact: true })).toHaveAttribute("aria-current", "page");
    await page.screenshot({ path: testInfo.outputPath("loaded.png") });
    if (viewport.width === 1440 || viewport.width === 390) {
      const results = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21aa"]).analyze();
      expect(results.violations).toEqual([]);
    }
    for (const nextState of ["empty", "failed"]) {
      state = nextState;
      await page.reload();
      await expect(status).toHaveText(nextState === "empty" ? "Choose a subtitle file to begin." : "Subtitle details are unavailable. Reload and try again.");
      await expect(status).not.toHaveAttribute("aria-busy", "true");
      await expect(page.getByRole("button", { name: "Save reviewed subtitle" })).toBeDisabled();
      // A failed refresh keeps the known server-rendered review; an empty result replaces it.
      await expect(page.locator(".subtitle-cue-row")).toHaveCount(nextState === "failed" ? 2 : 0);
      await expect(page.locator(".subtitle-topbar h1")).toBeInViewport();
      expect(await page.evaluate(() => document.documentElement.scrollWidth - innerWidth)).toBeLessThanOrEqual(1);
      await page.screenshot({ path: testInfo.outputPath(`${state}.png`) });
    }
    expect(writes).toEqual([]);
    await testInfo.attach("fixture-context", { body: JSON.stringify({ revision, command: "TestWriteUIStateFixturesSubtitleInspector + playwright test subtitle-inspector-layout.spec.ts", fixture: "Arrival, two installed SRT cues; isolated API pending/empty/503 states", viewport, theme, environment: `${testInfo.project.name}; server-rendered route and embedded production assets` }), contentType: "application/json" });
  });
}
