import { expect, test } from "@playwright/test";
import { readStaticSource } from "./static-sources";

const source = await readStaticSource([
  "../../../packages/webassets/static/supporter.js",
  "../internal/server/static/supporter.js",
]);

type Collection = { badges: { rank: number; edition: string; family: string; name: string }[]; display: string };
type SupporterWindow = Window & {
  requests: string[];
  signals: (AbortSignal | undefined)[];
  finish: ((collection: Collection) => void)[];
};
const hidden: Collection = { badges: [], display: "hidden" };
const visible: Collection = { badges: [{ rank: 2, edition: "monthly", family: "living-standard", name: "Crew" }], display: "automatic" };

test.beforeEach(async ({ page }) => {
  await page.setContent('<body><header><a class="header-supporter" href="/supporter">Support Kinosail</a></header><main class="library-shell"></main></body>');
  await page.evaluate(() => {
    const context = window as SupporterWindow;
    context.requests = [];
    context.signals = [];
    context.finish = [];
    window.fetch = (async (input: string, options: RequestInit) => {
      context.requests.push(input);
      context.signals.push(options.signal as AbortSignal | undefined);
      return { ok: true, json: () => new Promise((resolve) => context.finish.push(resolve)) } as Response;
    }) as typeof fetch;
  });
});

test("supporter recognition applies the display preference from the current collection", async ({ page }) => {
  await page.addScriptTag({ content: source });
  await page.evaluate((collection) => (window as SupporterWindow).finish[0](collection), hidden);
  await expect(page.locator(".header-supporter")).toBeHidden();
  expect(await page.evaluate(() => (window as SupporterWindow).requests)).toEqual(["/api/v1/supporter/collection"]);
});

test("leaving aborts collection recognition and back restoration ignores the old response", async ({ page }) => {
  await page.addScriptTag({ content: source });
  await page.evaluate(() => window.dispatchEvent(new PageTransitionEvent("pagehide", { persisted: true })));
  expect(await page.evaluate(() => (window as SupporterWindow).signals[0]?.aborted ?? false)).toBe(true);
  await page.evaluate(() => window.dispatchEvent(new PageTransitionEvent("pageshow", { persisted: true })));
  await expect.poll(() => page.evaluate(() => (window as SupporterWindow).finish.length)).toBe(2);
  await page.evaluate((collection) => (window as SupporterWindow).finish[1](collection), hidden);
  await expect(page.locator(".header-supporter")).toBeHidden();
  await page.evaluate((collection) => (window as SupporterWindow).finish[0](collection), visible);
  await expect(page.locator(".header-supporter")).toBeHidden();
  await expect(page.locator(".header-supporter img")).toHaveCount(0);
});

test("a later collection refresh wins when the older response finishes last", async ({ page }) => {
  await page.addScriptTag({ content: source });
  await page.evaluate(() => document.dispatchEvent(new Event("htmx:after:swap")));
  await expect.poll(() => page.evaluate(() => (window as SupporterWindow).finish.length)).toBe(2);
  expect(await page.evaluate(() => (window as SupporterWindow).signals[0]?.aborted ?? false)).toBe(true);
  await page.evaluate((collection) => (window as SupporterWindow).finish[1](collection), visible);
  await expect(page.locator(".header-supporter img")).toHaveAttribute("alt", "Crew · monthly");
  await page.evaluate((collection) => (window as SupporterWindow).finish[0](collection), hidden);
  await expect(page.locator(".header-supporter")).toBeVisible();
  await expect(page.locator(".header-supporter img")).toHaveAttribute("alt", "Crew · monthly");
});

test("authentication pages never fetch supporter recognition", async ({ page }) => {
  await page.locator("body").evaluate((body) => body.classList.add("auth"));
  await page.addScriptTag({ content: source });
  expect(await page.evaluate(() => (window as SupporterWindow).requests)).toEqual([]);
});
