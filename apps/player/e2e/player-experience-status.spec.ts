import { expect, test } from "@playwright/test";
import { installPlayerExperienceFixture } from "./player-experience-fixture";

installPlayerExperienceFixture();

test("traces a rejected Safari play request", async ({ page }) => {
  const events: string[] = [];
  page.on("request", (request) => {
    if (request.url().includes("/playback-events")) events.push(request.postDataJSON().event + ":" + request.postDataJSON().detail);
  });
  await page.evaluate(() => (window as Window & { setPlayFailure: (value: string) => void }).setPlayFailure("NotAllowedError"));
  await page.locator(".player-center-control[data-player-toggle]").click();
  await expect.poll(() => events).toEqual(expect.arrayContaining(["play-request:control", "play-rejected:control:NotAllowedError"]));
  await expect(page.locator("video")).toHaveJSProperty("paused", true);
});

test("reports honest loading and buffering progress", async ({ page }) => {
  const status = page.locator("[data-player-status]");
  const buffered = page.locator("[data-buffered]");
  await page.locator("video").evaluate((element) => element.play());
  await page.locator("video").dispatchEvent("loadstart");
  await expect(status).toContainText("Loading video…");
  await expect(buffered).toBeHidden();
  await expect(buffered).toHaveAttribute("aria-valuetext", "60% buffered");

  await page.locator("video").dispatchEvent("waiting");
  await page.waitForTimeout(600);
  await expect(status).toContainText("Buffering · 60% buffered");
  await expect(buffered).toBeVisible();
  await expect(page.locator(".media-stage")).toHaveClass(/is-busy/);
  await page.evaluate(() => (window as Window & { setBufferedEnd: (value: number) => void }).setBufferedEnd(78));
  await page.locator("video").dispatchEvent("progress");
  await expect(status).toContainText("Buffering · 78% buffered");
  await expect(buffered).toHaveAttribute("aria-valuetext", "78% buffered");

  await page.locator("video").dispatchEvent("playing");
  await expect(status).toBeHidden();
  await expect(page.locator(".media-stage")).not.toHaveClass(/is-busy/);
  await page.locator("video").dispatchEvent("seeking");
  await expect(status).toContainText("Seeking…");
  await expect(buffered).toBeHidden();
  await page.locator("video").dispatchEvent("seeked");
  await expect(status).toBeHidden();
});

test("records one private trace across playback events", async ({ page }) => {
  const events: Record<string, object | string | number | boolean | null>[] = [];
  page.on("request", (request) => {
    if (request.url().endsWith("/api/v1/items/movie/playback-events")) events.push(request.postDataJSON());
  });
  const video = page.locator("video");
  await video.evaluate((element) => element.play());
  await video.dispatchEvent("waiting");
  await video.dispatchEvent("stalled");
  await video.dispatchEvent("playing");
  await expect.poll(() => events.length).toBeGreaterThanOrEqual(4);
  expect(events.map((event) => event.event)).toEqual(expect.arrayContaining(["capability", "play", "waiting", "stalled", "playing"]));
  expect(new Set(events.map((event) => event.session))).toEqual(new Set(["trace-session"]));
  for (const event of events) {
    expect(event).not.toHaveProperty("title");
    expect(event).not.toHaveProperty("source");
    expect(event).not.toHaveProperty("url");
  }
});

test("retries requested autoplay when a browser only reaches canplay", async ({ page }) => {
  const video = page.locator("video");
  await video.dispatchEvent("canplay");
  await expect.poll(() => video.evaluate((element: HTMLVideoElement) => element.paused)).toBe(false);
});

test("resumed autoplay does not wait for a missing seeked event", async ({ page }) => {
  const video = page.locator("video");
  await expect.poll(() => video.evaluate((element: HTMLVideoElement) => element.paused)).toBe(false);
  await expect.poll(() => video.evaluate((element: HTMLVideoElement) => element.currentTime)).toBe(20);
});

test("canplay clears seeking when WebKit omits seeked", async ({ page }) => {
  const video = page.locator("video");
  const status = page.locator("[data-player-status]");
  await video.dispatchEvent("seeking");
  await expect(status).toContainText("Seeking…");
  await video.dispatchEvent("canplay");
  await expect(status).toBeHidden();
});

test("advancing video hides controls when WebKit omits playing", async ({ page }) => {
  const video = page.locator("video");
  await page.evaluate(() => (window as Window & { setPaused: (value: boolean) => void }).setPaused(false));
  await video.evaluate((element) => { element.currentTime += 1; });
  await video.dispatchEvent("timeupdate");

  await expect(page.locator("[data-player-controls]")).toHaveClass(/is-idle/);
});

test("does not obstruct playable video when the network stalls", async ({ page }) => {
  const video = page.locator("video");
  const status = page.locator("[data-player-status]");
  await video.dispatchEvent("playing");
  await expect(status).toBeHidden();

  await video.dispatchEvent("stalled");
  await expect(status).toBeHidden();
  await expect(page.locator(".media-stage")).not.toHaveClass(/is-busy/);
});

test("clears a transient buffering signal when playback advances", async ({ page }) => {
  const video = page.locator("video");
  const status = page.locator("[data-player-status]");
  await video.evaluate((element) => element.play());
  await video.dispatchEvent("waiting");
  await video.evaluate((element) => { element.currentTime += 1; });
  await video.dispatchEvent("timeupdate");
  await page.waitForTimeout(600);
  await video.dispatchEvent("stalled");
  await expect(status).toBeHidden();
  await expect(page.locator(".media-stage")).not.toHaveClass(/is-busy/);
});

test("does not call a fully buffered video buffering", async ({ page }) => {
  const video = page.locator("video");
  await video.evaluate((element) => element.play());
  await page.evaluate(() => (window as Window & { setBufferedEnd: (value: number) => void }).setBufferedEnd(100));
  await video.dispatchEvent("waiting");
  await page.waitForTimeout(600);
  await expect(page.locator("[data-player-status]")).toBeHidden();
});

test("keeps status accurate across bandwidth and seek changes", async ({ page }) => {
  const video = page.locator("video");
  const status = page.locator("[data-player-status]");
  await video.dispatchEvent("loadstart");
  await video.dispatchEvent("stalled");
  await expect(status).toContainText("Loading video…");

  await video.evaluate((element) => element.play());
  await page.evaluate(() => (window as Window & { setReadyState: (value: number) => void }).setReadyState(2));
  await video.dispatchEvent("waiting");
  await page.waitForTimeout(600);
  await expect(status).toContainText("Buffering · 60% buffered");

  await video.dispatchEvent("seeking");
  await expect(status).toContainText("Seeking…");
  for (const event of ["waiting", "stalled", "progress", "canplay", "playing"]) {
    await video.dispatchEvent(event);
    await expect(status).toContainText("Seeking…");
  }
  await video.dispatchEvent("seeked");
  await expect(status).toContainText("Buffering · 60% buffered");
  await page.evaluate(() => (window as Window & { setReadyState: (value: number) => void }).setReadyState(4));
  await video.dispatchEvent("playing");
  await expect(status).toBeHidden();
});
