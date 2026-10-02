import { expect, test } from "@playwright/test";
import { readStaticSource } from "./static-sources";

const source = await readStaticSource([
  "../../../packages/webassets/static/supporter.js",
  "../internal/server/static/supporter.js",
]);
type Collection = { badges: unknown[]; display: string };
type SupporterWindow = Window & {
  requests: string[];
  signals: Array<AbortSignal | undefined>;
  finishHeaders: (index: number) => void;
  finishCollection: (index: number, collection: Collection) => void;
};

test.beforeEach(async ({ page }) => {
  await page.setContent('<body><main class="library-shell"><a class="header-supporter" href="/supporter">Support Kinosail</a><input type="checkbox" data-supporter-visibility disabled></main></body>');
  await page.evaluate(() => {
    const context = window as SupporterWindow;
    context.requests = [];
    context.signals = [];
    const headers: Array<() => void> = [];
    const bodies: Array<(collection: Collection) => void> = [];
    context.finishHeaders = (index) => headers[index]();
    context.finishCollection = (index, collection) => bodies[index](collection);
    window.fetch = (async (input: string, options: RequestInit) => {
      const index = context.requests.length;
      context.requests.push(input);
      context.signals.push(options.signal as AbortSignal | undefined);
      const body = new Promise<Collection>((resolve) => { bodies[index] = resolve; });
      return new Promise<Response>((resolve) => {
        headers[index] = () => resolve({ ok: true, json: () => body } as Response);
      });
    }) as typeof fetch;
  });
});

test("supporter collection applies its display preference in one request", async ({ page }, testInfo) => {
  await page.addScriptTag({ content: source });
  await page.evaluate(() => {
    const context = window as SupporterWindow;
    context.finishHeaders(0);
    context.finishCollection(0, { badges: [], display: "hidden" });
  });
  await expect(page.locator(".header-supporter")).toBeHidden();
  await expect(page.locator("[data-supporter-visibility]")).toBeEnabled();
  expect(await page.evaluate(() => (window as SupporterWindow).requests)).toEqual(["/api/v1/supporter/collection"]);
  await page.screenshot({ path: testInfo.outputPath("collection-loaded.png") });
});

test("leaving during response headers cancels recognition and back restores a fresh collection", async ({ page }, testInfo) => {
  await page.addScriptTag({ content: source });
  await page.evaluate(() => {
    window.dispatchEvent(new PageTransitionEvent("pagehide", { persisted: true }));
    const context = window as SupporterWindow;
    context.finishHeaders(0);
    context.finishCollection(0, { badges: [], display: "hidden" });
  });
  await expect(page.locator(".header-supporter")).toBeVisible();
  expect(await page.evaluate(() => (window as SupporterWindow).signals[0]?.aborted)).toBe(true);
  await page.evaluate(() => window.dispatchEvent(new PageTransitionEvent("pageshow", { persisted: true })));
  await expect.poll(() => page.evaluate(() => (window as SupporterWindow).requests.length)).toBe(2);
  await page.evaluate(() => {
    const context = window as SupporterWindow;
    context.finishHeaders(1);
    context.finishCollection(1, { badges: [], display: "hidden" });
  });
  await expect(page.locator(".header-supporter")).toBeHidden();
  expect(await page.evaluate(() => (window as SupporterWindow).signals[1]?.aborted)).toBe(false);
  await page.screenshot({ path: testInfo.outputPath("back-restored.png") });
});

test("leaving during the collection body prevents stale display changes", async ({ page }, testInfo) => {
  await page.addScriptTag({ content: source });
  await page.evaluate(() => (window as SupporterWindow).finishHeaders(0));
  await page.evaluate(() => {
    window.dispatchEvent(new PageTransitionEvent("pagehide"));
    (window as SupporterWindow).finishCollection(0, { badges: [], display: "hidden" });
  });
  await expect(page.locator(".header-supporter")).toBeVisible();
  await expect(page.locator("[data-supporter-visibility]")).toBeDisabled();
  expect(await page.evaluate(() => (window as SupporterWindow).signals[0]?.aborted)).toBe(true);
  await page.screenshot({ path: testInfo.outputPath("left-before-body.png") });
});

test("an older collection cannot overwrite recognition after a navigation swap", async ({ page }, testInfo) => {
  await page.addScriptTag({ content: source });
  await page.evaluate(() => document.dispatchEvent(new Event("htmx:after:swap")));
  await expect.poll(() => page.evaluate(() => (window as SupporterWindow).requests.length)).toBe(2);
  await page.evaluate(() => {
    const context = window as SupporterWindow;
    context.finishHeaders(1);
    context.finishCollection(1, { badges: [], display: "hidden" });
  });
  await expect(page.locator(".header-supporter")).toBeHidden();
  await page.evaluate(() => {
    const context = window as SupporterWindow;
    context.finishHeaders(0);
    context.finishCollection(0, { badges: [], display: "automatic" });
  });
  await expect(page.locator(".header-supporter")).toBeHidden();
  expect(await page.evaluate(() => (window as SupporterWindow).signals[0]?.aborted)).toBe(true);
  await page.screenshot({ path: testInfo.outputPath("newest-collection.png") });
});

test("newer nonempty recognition survives an older hidden collection", async ({ page }, testInfo) => {
  await page.addScriptTag({ content: source });
  await page.evaluate(() => document.dispatchEvent(new Event("htmx:after:swap")));
  await expect.poll(() => page.evaluate(() => (window as SupporterWindow).requests.length)).toBe(2);
  await page.evaluate(() => {
    const context = window as SupporterWindow;
    context.finishHeaders(1);
    context.finishCollection(1, { badges: [{ rank: 2, edition: "monthly", family: "living-standard", name: "Crew" }], display: "automatic" });
  });
  await expect(page.locator(".header-supporter img")).toHaveAttribute("alt", "Crew · monthly");
  await page.evaluate(() => {
    const context = window as SupporterWindow;
    context.finishHeaders(0);
    context.finishCollection(0, { badges: [], display: "hidden" });
  });
  await expect(page.locator(".header-supporter")).toBeVisible();
  await expect(page.locator(".header-supporter img")).toHaveAttribute("alt", "Crew · monthly");
  expect(await page.evaluate(() => (window as SupporterWindow).signals[0]?.aborted)).toBe(true);
  await page.screenshot({ path: testInfo.outputPath("nonempty-newest-collection.png") });
});

test("authentication pages never fetch supporter recognition", async ({ page }) => {
  await page.locator("body").evaluate((body) => body.classList.add("auth"));
  await page.addScriptTag({ content: source });
  expect(await page.evaluate(() => (window as SupporterWindow).requests)).toEqual([]);
});
