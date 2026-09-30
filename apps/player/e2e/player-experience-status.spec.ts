import { expect, test } from "@playwright/test";
import { readFile } from "node:fs/promises";
import { installPlayerExperienceFixture } from "./player-experience-fixture";

installPlayerExperienceFixture();

test("blocked autoplay reveals Play without a second action", async ({ page }, testInfo) => {
  await page.addStyleTag({ content: await readFile("../../../packages/webassets/static/player-stage.css", "utf8") });
  await page.addStyleTag({ content: await readFile("../internal/server/static/home.css", "utf8") });
  await page.setViewportSize({ width: 390, height: 844 });
  await page.locator("video").dispatchEvent("kinosail:play-needs-gesture");
  await expect(page.locator("[data-player-status]")).toBeHidden();
  const play = page.locator(".player-center-control[data-player-toggle]");
  await expect(play).toBeVisible();
  await expect(page.locator(".player-control-row [data-player-toggle]")).toBeHidden();
  await expect(page.locator("[data-player-controls]")).toHaveCSS("opacity", "1");
  await page.screenshot({ path: testInfo.outputPath("390-one-play-button.png") });
  await play.click();
  await expect(page.locator("video")).toHaveJSProperty("paused", false);
});

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
  await page.addStyleTag({ content: await readFile("../../../packages/webassets/static/player-app.css", "utf8") });
  await page.addStyleTag({ content: await readFile("../../../packages/webassets/static/player-stage.css", "utf8") });
  await page.addStyleTag({ content: await readFile("../internal/server/static/home.css", "utf8") });
  const status = page.locator("[data-player-status]");
  const spinner = status.locator(".buffer-skeleton");
  const buffered = page.locator("[data-buffered]");
  await page.locator("video").evaluate((element) => element.play());
  await page.locator("video").dispatchEvent("loadstart");
  await expect(status).toContainText("Loading video…");
  await expect(spinner).toBeVisible();
  await expect(spinner).toHaveCSS("border-top-style", "solid");
  await expect(spinner).toHaveCSS("border-radius", "50%");
  await expect(spinner).toHaveCSS("animation-name", "player-buffer-spin");
  await expect(buffered).toBeHidden();
  await expect(buffered).toHaveAttribute("aria-valuetext", "60% buffered");

  await page.locator("video").dispatchEvent("waiting");
  await page.waitForTimeout(600);
  await expect(status).toContainText("Buffering · 60% buffered");
  await expect(spinner).toBeVisible();
  await expect(buffered).toBeHidden();
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
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.locator("video").dispatchEvent("loadstart");
  await expect(spinner).toBeVisible();
  await expect(spinner).toHaveCSS("animation-name", "none");
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

for (const target of [45, 5]) test(`advancing playback clears seeking at ${target}s without completion events`, { tag: "@smoke" }, async ({ page }, testInfo) => {
  await page.emulateMedia({ colorScheme: "dark" });
  for (const path of ["../../../packages/webassets/static/player-app.css", "../../../packages/webassets/static/player-stage.css", "../internal/server/static/home.css"]) {
    await page.addStyleTag({ content: await readFile(path, "utf8") });
  }
  const video = page.locator("video");
  const status = page.locator("[data-player-status]");
  await video.evaluate((element) => element.play());
  await video.evaluate((element, position) => {
    Object.defineProperty(element, "seeking", { configurable: true, writable: true, value: true });
    element.currentTime = position;
    element.dispatchEvent(new Event("seeking"));
    element.dispatchEvent(new Event("timeupdate"));
  }, target);
  await expect(status).toContainText("Seeking…");
  await expect(status).toBeVisible();
  await expect(status).toHaveAttribute("aria-busy", "true");
  for (const viewport of [{ width: 390, height: 844 }, { width: 1440, height: 900 }]) {
    await page.setViewportSize(viewport);
    await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-pending-seek.png`) });
  }

  // A position update during the seek or while paused is not resumed playback.
  await video.evaluate((element) => { element.currentTime += 0.1; element.dispatchEvent(new Event("timeupdate")); });
  await expect(status).toBeVisible();
  await page.evaluate(() => (window as Window & { setPaused: (value: boolean) => void }).setPaused(true));
  await video.evaluate((element) => {
    Object.defineProperty(element, "seeking", { configurable: true, writable: true, value: false });
    element.currentTime += 0.1;
    element.dispatchEvent(new Event("timeupdate"));
  });
  await expect(status).toBeVisible();
  await page.evaluate(() => (window as Window & { setPaused: (value: boolean) => void }).setPaused(false));
  await video.dispatchEvent("timeupdate");
  await expect(status).toBeVisible();

  await video.evaluate((element) => { element.currentTime += 0.25; element.dispatchEvent(new Event("timeupdate")); });
  await expect(status).toBeHidden();
  await expect(status).not.toHaveAttribute("aria-busy");
  await expect(page.locator(".media-stage")).not.toHaveClass(/is-busy/);
  await video.dispatchEvent("seeking");
  await expect(status).toBeVisible();
  await video.evaluate((element) => { element.currentTime += 0.25; element.dispatchEvent(new Event("timeupdate")); });
  await expect(status).toBeHidden();
  for (const viewport of [{ width: 390, height: 844 }, { width: 1440, height: 900 }]) {
    await page.setViewportSize(viewport);
    await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-resumed-seek.png`) });
  }
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
  await video.evaluate((element) => element.play());
  await expect(status).toBeHidden();

  await video.dispatchEvent("stalled");
  await expect(status).toBeHidden();
  await expect(page.locator(".media-stage")).not.toHaveClass(/is-busy/);
});

test("clears a transient buffering signal when playback advances", async ({ page }) => {
  await page.clock.install();
  const video = page.locator("video");
  const status = page.locator("[data-player-status]");
  await video.evaluate((element) => element.play());
  await video.dispatchEvent("waiting");
  await video.evaluate((element) => { element.currentTime += 1; });
  await video.dispatchEvent("timeupdate");
  await page.clock.runFor(600);
  await video.dispatchEvent("stalled");
  await expect(status).toBeHidden();
  await expect(page.locator(".media-stage")).not.toHaveClass(/is-busy/);
  await page.clock.fastForward(25_000);
  await expect(status).toBeHidden();
  await expect(status.locator("[data-player-fallback]")).toBeHidden();
});

test("a stream that never advances offers a retry and clears it on recovery", async ({ page }, testInfo) => {
  await page.addStyleTag({ content: await readFile("../../../packages/webassets/static/player-app.css", "utf8") });
  await page.addStyleTag({ content: await readFile("../../../packages/webassets/static/player-stage.css", "utf8") });
  await page.clock.install();
  const video = page.locator("video");
  const status = page.locator("[data-player-status]");
  await video.evaluate((element) => element.play());
  await video.dispatchEvent("waiting");
  await page.clock.runFor(600);
  await expect(status).toContainText("Buffering · 60% buffered");
  await page.clock.fastForward(12_100);
  await expect(status.locator("[data-player-fallback]")).toBeHidden();
  await page.clock.fastForward(12_100);
  await expect(status).toContainText("Playback has not advanced");
  await expect(status.locator("[data-player-fallback]")).toHaveText("Retry playback");
  await expect(status).toHaveAttribute("aria-busy", "false");
  await expect(page.locator(".media-stage")).not.toHaveClass(/is-busy/);
  for (const viewport of [{ width: 390, height: 844 }, { width: 1440, height: 900 }]) {
    await page.setViewportSize(viewport);
    await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-stalled-recovery.png`) });
  }
  await video.dispatchEvent("playing");
  await expect(status).toBeHidden();
  await page.clock.fastForward(15_000);
  await expect(status).toBeHidden();
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
