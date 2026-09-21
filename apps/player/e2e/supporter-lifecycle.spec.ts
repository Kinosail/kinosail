import { expect, test } from "@playwright/test";
import { readStaticSource } from "./static-sources";

const source = await readStaticSource([
  "../../../packages/webassets/static/supporter.js",
  "../internal/server/static/supporter.js",
]);

type SupporterWindow = Window & {
  requests: string[];
  signals: AbortSignal[];
  finishStatus: () => void;
  finishPreference: () => void;
};

test.beforeEach(async ({ page }) => {
  await page.setContent('<body><main class="library-shell"></main></body>');
  await page.evaluate(() => {
    const context = window as SupporterWindow;
    context.requests = [];
    context.signals = [];
    window.fetch = (async (input: string, options: RequestInit) => {
      context.requests.push(input);
      context.signals.push(options.signal as AbortSignal);
      return {
        ok: true,
        json: () => new Promise((resolve) => {
          if (input.endsWith("/display")) context.finishPreference = () => resolve({ display: "hidden" });
          else context.finishStatus = () => resolve({});
        }),
      } as Response;
    }) as typeof fetch;
  });
});

test("supporter recognition reads and applies the current display preference", async ({ page }) => {
  await page.addScriptTag({ content: source });
  await page.evaluate(() => (window as SupporterWindow).finishStatus());
  await expect.poll(() => page.evaluate(() => (window as SupporterWindow).requests)).toEqual(["/api/v1/supporter", "/api/v1/supporter/display"]);
  await page.evaluate(() => (window as SupporterWindow).finishPreference());
  await expect(page.locator(".supporter-signature")).toHaveJSProperty("hidden", true);
  expect(await page.evaluate(() => (window as SupporterWindow).signals.every((signal) => !signal.aborted))).toBe(true);
});

test("leaving during the status response prevents the follow-up request and supports back restoration", async ({ page }) => {
  await page.addScriptTag({ content: source });
  await page.evaluate(() => {
    window.dispatchEvent(new PageTransitionEvent("pagehide", { persisted: true }));
    (window as SupporterWindow).finishStatus();
  });
  await expect(page.locator("[data-supporter-pending]")).toHaveCount(0);
  expect(await page.evaluate(() => (window as SupporterWindow).requests)).toEqual(["/api/v1/supporter"]);
  expect(await page.evaluate(() => (window as SupporterWindow).signals[0].aborted)).toBe(true);
  await page.evaluate(() => window.dispatchEvent(new PageTransitionEvent("pageshow", { persisted: true })));
  await expect.poll(() => page.evaluate(() => (window as SupporterWindow).requests.length)).toBe(2);
  await page.evaluate(() => (window as SupporterWindow).finishStatus());
  await expect.poll(() => page.evaluate(() => (window as SupporterWindow).requests.length)).toBe(3);
  await page.evaluate(() => (window as SupporterWindow).finishPreference());
  await expect(page.locator(".supporter-signature")).toHaveJSProperty("hidden", true);
  expect(await page.evaluate(() => (window as SupporterWindow).signals.slice(1).every((signal) => !signal.aborted))).toBe(true);
});

test("leaving during the preference response prevents stale display changes", async ({ page }) => {
  await page.addScriptTag({ content: source });
  await page.evaluate(() => (window as SupporterWindow).finishStatus());
  await expect.poll(() => page.evaluate(() => (window as SupporterWindow).requests.length)).toBe(2);
  await page.evaluate(() => {
    window.dispatchEvent(new PageTransitionEvent("pagehide"));
    (window as SupporterWindow).finishPreference();
  });
  await expect(page.locator("[data-supporter-pending]")).toHaveCount(0);
  await expect(page.locator(".supporter-signature")).toBeVisible();
  expect(await page.evaluate(() => (window as SupporterWindow).signals.every((signal) => signal.aborted))).toBe(true);
});

test("authentication pages never fetch supporter recognition", async ({ page }) => {
  await page.locator("body").evaluate((body) => body.classList.add("auth"));
  await page.addScriptTag({ content: source });
  expect(await page.evaluate(() => (window as SupporterWindow).requests)).toEqual([]);
});
