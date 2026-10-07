import { expect, test } from "@playwright/test";
import { readFile } from "node:fs/promises";
import { installPlayerExperienceFixture } from "./player-experience-fixture";

installPlayerExperienceFixture();

const frame = '<svg xmlns="http://www.w3.org/2000/svg" width="320" height="180"><rect width="320" height="180" fill="green"/></svg>';

test.beforeEach(async ({ page }) => {
  await page.addStyleTag({ content: await readFile("../../../packages/webassets/static/player-stage.css", "utf8") });
});

for (const completion of ["canplay", "advancing timeupdate"]) test(`@smoke the timeline remains usable throughout seeking and resumes idle hiding after ${completion}`, async ({ page }, testInfo) => {
  await page.clock.install();
  await page.locator("video").evaluate(video => video.play());
  await page.locator("video").dispatchEvent("seeking");
  const seek = page.getByRole("slider", { name: "Seek", exact: true });
  for (const viewport of [{ width: 390, height: 844 }, { width: 1440, height: 900 }, { width: 1920, height: 1080 }]) {
    await page.setViewportSize(viewport);
    await expect(seek).toBeVisible();
    await expect(page.locator("[data-player-message]")).toHaveText("Seeking…");
    await page.locator(".media-stage").screenshot({ path: testInfo.outputPath(`seeking-${viewport.width}.png`) });
  }
  await page.clock.runFor(3000);
  await expect(page.locator("[data-player-controls]")).toHaveCSS("opacity", "1");
  await page.locator("video").evaluate(video => {
    (window as Window & { setReadyState: (value: number) => void }).setReadyState(2);
    video.dispatchEvent(new Event("seeked"));
  });
  await page.clock.runFor(3000);
  await expect(seek).toBeVisible();
  await expect(page.locator("[data-player-controls]")).toHaveCSS("opacity", "1");
  await page.evaluate(() => (window as Window & { setReadyState: (value: number) => void }).setReadyState(4));
  if (completion === "canplay") await page.locator("video").dispatchEvent("canplay");
  else await page.locator("video").evaluate(video => {
    video.currentTime += 1;
    video.dispatchEvent(new Event("timeupdate"));
  });
  await page.clock.runFor(5000);
  await expect(page.locator("[data-player-controls]")).toHaveCSS("opacity", "0");
});

test("holding a scrub keeps the timeline visible without changing playback until release", async ({ page }) => {
  await page.route("**/trickplay/movie/*", route => route.fulfill({ contentType: "image/svg+xml", body: frame }));
  await page.clock.install();
  await page.locator("video").evaluate(video => video.play());
  const seek = page.locator("[data-player-seek]");
  await seek.evaluate((input: HTMLInputElement) => {
    input.dispatchEvent(new PointerEvent("pointerdown", { bubbles: true }));
    input.value = "55";
    input.dispatchEvent(new Event("input", { bubbles: true }));
  });
  await page.clock.runFor(5000);
  await expect(page.locator("[data-player-controls]")).toHaveCSS("opacity", "1");
  await expect(seek).toHaveValue("55");
  await expect(page.locator("video")).toHaveJSProperty("currentTime", 20);
  await seek.dispatchEvent("pointercancel");
  await page.clock.runFor(2500);
  await expect(seek).toHaveValue("20");
  await expect(page.locator("[data-player-controls]")).toHaveCSS("opacity", "0");
});

test("previews request promptly, coalesce slow loads, reuse decoded frames and recover from failure", async ({ page }, testInfo) => {
  const requests: string[] = [];
  let releaseFirst: () => void = () => {};
  const pending = new Promise<void>(resolve => { releaseFirst = resolve; });
  await page.route("**/trickplay/movie/*", async route => {
    const path = new URL(route.request().url()).pathname;
    requests.push(path);
    if (path.endsWith("/30")) await pending;
    await route.fulfill(path.endsWith("/70") ? { status: 503 } : { contentType: "image/svg+xml", body: frame });
  });
  const seek = page.locator("[data-player-seek]");
  await seek.evaluate(input => { input.dataset.trickplay = "https://127.0.0.1:38127/trickplay/movie/{second}"; });
  await page.clock.install({ time: new Date("2026-01-01T00:00:00Z") });
  await page.clock.pauseAt(new Date("2026-01-01T00:00:00Z"));
  const preview = page.locator("[data-seek-preview]");
  const show = async (value: number) => {
    await seek.evaluate((input: HTMLInputElement, position) => { input.value = String(position); input.dispatchEvent(new Event("input")); }, value);
    await page.clock.runFor(32);
  };
  await show(30);
  await expect.poll(() => requests.length).toBe(1);
  const pendingBox = await preview.boundingBox();
  await show(40);
  await show(55);
  expect(requests).toEqual(["/trickplay/movie/30"]);
  releaseFirst();
  await expect(preview.locator("img")).toBeVisible();
  await expect(preview.locator("img")).toHaveAttribute("src", /\/50$/);
  await expect(page.getByRole("slider", { name: "Seek", exact: true })).toBeVisible();
  expect(requests).toEqual(["/trickplay/movie/30", "/trickplay/movie/50"]);
  expect((await preview.boundingBox())!.height).toBe(pendingBox!.height);
  await show(30);
  await expect(preview.locator("img")).toBeVisible();
  await expect(preview.locator("img")).toHaveAttribute("src", /\/30$/);
  expect(requests).toHaveLength(2);
  await show(70);
  await expect(preview.locator("img")).toBeHidden();
  await expect(preview.locator("[data-seek-frame]")).toHaveAttribute("aria-busy", "false");
  await show(30);
  await expect(preview.locator("img")).toBeVisible();
  await seek.dispatchEvent("blur");
  await expect(preview).toBeHidden();
  await testInfo.attach("preview-requests", { body: JSON.stringify(requests), contentType: "application/json" });
});

for (const unavailable of ["missing source", "beyond the preview range"]) test(`reopening a preview with ${unavailable} never displays a stale frame`, async ({ page }) => {
  const requests: string[] = [];
  await page.route("**/trickplay/movie/*", route => {
    requests.push(route.request().url());
    return route.fulfill({ contentType: "image/svg+xml", body: frame });
  });
  const seek = page.locator("[data-player-seek]");
  await seek.evaluate((input: HTMLInputElement) => {
    input.dataset.trickplay = "https://127.0.0.1:38127/trickplay/movie/{second}";
    input.value = "30";
    input.dispatchEvent(new Event("input"));
  });
  const preview = page.locator("[data-seek-preview]");
  await expect(preview.locator("img")).toBeVisible();
  await seek.dispatchEvent("blur");
  await seek.evaluate((input: HTMLInputElement, scenario) => {
    if (scenario === "missing source") delete input.dataset.trickplay;
    else input.max = "50000";
    const bounds = input.getBoundingClientRect();
    input.dispatchEvent(new PointerEvent("pointermove", { clientX: bounds.left + bounds.width * .9 }));
  }, unavailable);
  await expect(preview).toBeVisible();
  await expect(preview.locator("img")).toBeHidden();
  await expect(preview.locator("[data-seek-frame]")).toHaveAttribute("aria-busy", "false");
  expect(requests).toHaveLength(1);
});

test("preview caching retains recent frames and evicts old frames after its bound", async ({ page }) => {
  const requests: string[] = [];
  await page.route("**/trickplay/movie/*", route => {
    requests.push(new URL(route.request().url()).pathname);
    return route.fulfill({ contentType: "image/svg+xml", body: frame });
  });
  const seek = page.locator("[data-player-seek]");
  const show = async (second: number) => {
    await seek.evaluate((input: HTMLInputElement, position) => {
      input.dataset.trickplay = "https://127.0.0.1:38127/trickplay/movie/{second}";
      input.max = "1000";
      const bounds = input.getBoundingClientRect();
      input.dispatchEvent(new PointerEvent("pointermove", { clientX: bounds.left + bounds.width * position / 1000 }));
    }, second + 5);
    await expect(page.locator("[data-seek-preview] img")).toHaveAttribute("src", new RegExp(`/${second}$`));
  };
  await show(0);
  const oldest = await page.locator("[data-seek-preview] img").elementHandle();
  await show(10);
  await show(20);
  const recent = await page.locator("[data-seek-preview] img").elementHandle();
  for (let second = 30; second < 140; second += 10) await show(second);
  expect(requests).toHaveLength(14);
  await show(20);
  expect(requests).toHaveLength(14);
  expect(await page.locator("[data-seek-preview] img").evaluate((image, previous) => image === previous, recent)).toBe(true);
  await show(0);
  expect(await page.locator("[data-seek-preview] img").evaluate((image, previous) => image === previous, oldest)).toBe(false);
});
