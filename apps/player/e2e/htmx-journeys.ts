import { readFile } from "node:fs/promises";
import { execFileSync } from "node:child_process";
import { createHash } from "node:crypto";
import { fileURLToPath } from "node:url";
import type { Page } from "@playwright/test";

export function registerHtmxJourneys(app: "player" | "subtitles", test: typeof import("@playwright/test").test, expect: typeof import("@playwright/test").expect) {
  const origin = "http://localhost:38127";
  const root = new URL("../../../", import.meta.url);
  let config: string, htmx: string, pwa: string, css: string;
  let revision: string, diffHash: string;
  test.use({ serviceWorkers: "block" });
  test.beforeAll(async () => {
    revision = execFileSync("git", ["rev-parse", "HEAD"], { cwd: fileURLToPath(root), encoding: "utf8" }).trim();
    diffHash = createHash("sha256").update(execFileSync("git", ["diff", "HEAD"], { cwd: fileURLToPath(root) })).digest("hex");
    const server = await readFile(new URL(`apps/${app}/internal/server/server.go`, root), "utf8");
    config = server.match(/<meta name="htmx-config" content='([^']+)'/)![1];
    let library: string;
    [htmx, library, pwa, css] = await Promise.all([
      `apps/${app}/internal/server/static/htmx.min.js`,
      "packages/webassets/static/pwa-library.js", "packages/webassets/static/pwa.js", "packages/webassets/static/player-app.css",
    ].map(path => readFile(new URL(path, root), "utf8")));
    pwa = `${library}\n${pwa}`;
  });
  test.beforeEach(async ({ browserName }, info) => {
    await info.attach("verification-context", { contentType: "application/json", body: JSON.stringify({ app, revision, diffHash, browserName, node: process.version, platform: process.platform, command: process.argv, testData: "Local library card fixture, delayed HTML responses, HTTP failures, aborted requests", htmxHash: createHash("sha256").update(htmx).digest("hex") }, null, 2) });
  });

  const content = (title: string) => `<main id="main" class="library-shell"><section id="library" class="grid"><a class="card" href="/item/1"><img class="poster" src="/poster.svg" width="400" height="600" alt=""><h2>${title}</h2></a></section></main>`;
  async function open(page: Page) {
    await page.route(`${origin}/**`, async route => {
      const path = new URL(route.request().url()).pathname;
      if (path === "/htmx.js") return route.fulfill({ contentType: "text/javascript", body: htmx });
      if (path === "/pwa.js") return route.fulfill({ contentType: "text/javascript", body: pwa });
      if (path === "/app.css") return route.fulfill({ contentType: "text/css", body: css });
      if (path === "/poster.svg") return route.fulfill({ contentType: "image/svg+xml", body: '<svg xmlns="http://www.w3.org/2000/svg" width="400" height="600"><rect width="400" height="600" fill="#31523e"/></svg>' });
      return route.fulfill({ contentType: "text/html", headers: { "Content-Security-Policy": "default-src 'self'; script-src 'self'; style-src 'self'" }, body: `<!doctype html><html><head><meta name="viewport" content="width=device-width"><meta name="htmx-config" content='${config}'><link rel="stylesheet" href="/app.css"><script defer src="/htmx.js"></script><script defer src="/pwa.js"></script></head><body class="library-page"><button hx-get="/first" hx-target="#main" hx-select="#main" hx-swap="outerHTML" hx-push-url="true">First</button><button hx-get="/second" hx-target="#main" hx-select="#main" hx-swap="outerHTML" hx-push-url="true">Second</button>${content("Original")}</body></html>` });
    });
    await page.goto(origin);
  }
  async function ready(page: Page) {
    await expect(page.locator("#main")).not.toHaveAttribute("aria-busy", "true");
    await expect(page.locator("#main")).not.toHaveClass(/request-skeleton/);
    expect(await page.locator("#main").evaluate(element => element.inert)).toBe(false);
  }

  test("HTMX delays visual placeholders while keeping pending content inert @smoke", async ({ page }, info) => {
    await open(page);
    await page.clock.install();
    await page.clock.pauseAt(new Date());
    let release!: () => void;
    const held = new Promise<void>(resolve => { release = resolve; });
    await page.route(`${origin}/first`, async route => { await held; await route.fulfill({ contentType: "text/html", body: content("Loaded") }); });
    await page.getByRole("button", { name: "First" }).click();
    await expect(page.locator("#main")).toHaveAttribute("aria-busy", "true");
    expect(await page.locator("#main").evaluate(element => element.inert)).toBe(true);
    await expect(page.locator("#main")).not.toHaveClass(/request-skeleton/);
    await page.route(`${origin}/second`, route => route.fulfill({ status: 500, body: "Failed overlapping request" }));
    const second = page.waitForResponse(`${origin}/second`);
    await page.getByRole("button", { name: "Second" }).click();
    await (await second).finished();
    await page.clock.runFor(119);
    await expect(page.locator("#main")).not.toHaveClass(/request-skeleton/);
    await page.clock.runFor(1);
    await expect(page.locator("#main")).toHaveClass(/request-skeleton/);
    await page.screenshot({ caret: "initial", path: info.outputPath("delayed-pending.png") });
    release();
    await page.clock.resume();
    await expect(page.locator("#main")).toContainText("Original");
    await ready(page);
  });

  for (const status of [200, 500]) test(`HTMX clears a fast ${status} response before delayed placeholders appear @smoke`, async ({ page }) => {
    await open(page);
    await page.clock.install();
    await page.clock.pauseAt(new Date());
    await page.route(`${origin}/first`, route => route.fulfill({ status, contentType: "text/html", body: content("Loaded") }));
    await page.getByRole("button", { name: "First" }).click();
    await ready(page);
    await page.clock.runFor(1000);
    await ready(page);
    await expect(page.locator("#main")).toContainText(status === 200 ? "Loaded" : "Original");
  });

  for (const width of [390, 1440, 1920]) {
    test(`HTMX replaces loaded and empty content after pending work at ${width}px @smoke`, async ({ page }, info) => {
      const errors: string[] = [];
      const collectError = (message: import("@playwright/test").ConsoleMessage) => { if (message.type() === "error") errors.push(message.text()); };
      page.on("console", collectError);
      const capture = async (name: string) => {
        expect(errors).toEqual([]);
        // WebKit reports Playwright's injected screenshot stylesheet under strict CSP.
        page.off("console", collectError);
        await page.screenshot({ caret: "initial", path: info.outputPath(name) });
        page.on("console", collectError);
      };
      await page.setViewportSize({ width, height: 900 });
      await page.emulateMedia({ reducedMotion: "reduce" });
      await open(page);
      expect(await page.evaluate(() => (window as any).htmx.version)).toBe("4.0.0");
      for (const [label, body] of [["Loaded", content("Loaded")], ["Empty", '<main id="main" class="library-shell"><p role="status">No results</p></main>']]) {
        let release!: () => void;
        const held = new Promise<void>(resolve => { release = resolve; });
        await page.route(`${origin}/first`, async route => { await held; await route.fulfill({ contentType: "text/html", body }); });
        const before = await page.locator("#main").boundingBox();
        await page.getByRole("button", { name: "First" }).click();
        await expect(page.locator("#main")).toHaveAttribute("aria-busy", "true");
        await expect(page.locator("#main")).toHaveClass(/request-skeleton/);
        const pending = await page.locator("#main").boundingBox();
        for (const key of ["x", "y", "width", "height"] as const) expect(Math.abs(pending![key] - before![key])).toBeLessThanOrEqual(1);
        await capture(`${width}-${label}-pending.png`);
        release();
        await expect(page.locator("#main")).toContainText(label === "Empty" ? "No results" : label);
        await ready(page);
        await capture(`${width}-${label}.png`);
        await page.unroute(`${origin}/first`);
      }
      expect(errors).toEqual([]);
    });
  }

  for (const width of [390, 1440, 1920]) for (const failure of [400, 500, 204, "network", "abort", "timeout"] as const) {
    test(`HTMX ${failure} leaves loaded content usable at ${width}px @smoke`, async ({ page }, info) => {
      await page.setViewportSize({ width, height: 900 });
      await open(page);
      let release!: () => void;
      const held = new Promise<void>(resolve => { release = resolve; });
      await page.route(`${origin}/first`, async route => {
        await held;
        if (typeof failure === "number") await route.fulfill({ status: failure, contentType: "text/html", body: failure === 204 ? "" : content("Error response") });
        else await route.abort("failed");
      });
      if (failure === "timeout") await page.evaluate(() => { (window as any).htmx.config.defaultTimeout = 1000; });
      await page.getByRole("button", { name: "First" }).click();
      await expect(page.locator("#main")).toHaveAttribute("aria-busy", "true");
      await expect(page.locator("#main")).toHaveClass(/request-skeleton/);
      if (failure === "abort") await page.evaluate(() => { (window as any).htmx.trigger(document.querySelector('button[hx-get="/first"]'), "htmx:abort"); });
      if (failure !== "abort" && failure !== "timeout") release();
      await ready(page);
      await expect(page.locator("#main")).toContainText("Original");
      await page.screenshot({ caret: "initial", path: info.outputPath(`${failure}-recovered.png`) });
      release();
    });
  }

  test("HTMX keeps the newer result when an older request finishes last @smoke", async ({ page }) => {
    await open(page);
    let release!: () => void;
    const held = new Promise<void>(resolve => { release = resolve; });
    await page.route(`${origin}/first`, async route => { await held; await route.fulfill({ contentType: "text/html", body: content("Older result") }); });
    await page.route(`${origin}/second`, route => route.fulfill({ contentType: "text/html", body: content("Newer result") }));
    await page.getByRole("button", { name: "First" }).click();
    await expect(page.locator("#main")).toHaveAttribute("aria-busy", "true");
    await page.getByRole("button", { name: "Second" }).click();
    await expect(page.locator("#main")).toContainText("Newer result");
    const completed = page.waitForResponse(`${origin}/first`);
    release();
    await completed;
    await ready(page);
    await expect(page.locator("#main")).toContainText("Newer result");
    await expect(page).toHaveURL(`${origin}/second`);
  });
}
