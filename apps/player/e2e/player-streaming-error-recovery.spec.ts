import { expect, test } from "@playwright/test";
import { failDirect, startDirectPlayer } from "./player-direct-fallback-fixture";

test("a stalled audio-compatible stream retries itself once, then offers recovery", async ({ page }) => {
  await startDirectPlayer(page, { initialSource: false, initialHls: true, compatibleMode: "audio-transcode", compatibleLabel: "Transcoding audio" });
  await page.clock.install();
  const video = page.locator("video");
  await video.evaluate((media: HTMLVideoElement) => { media.currentTime = 42; });
  await video.dispatchEvent("waiting");
  await page.clock.fastForward(8_000);
  await video.dispatchEvent("waiting");
  await page.clock.fastForward(4_100);
  expect(await page.evaluate(() => (window as Window & { FakeHls: { instances: number } }).FakeHls.instances)).toBe(2);
  await expect(video).toHaveJSProperty("currentTime", 42);
  await expect.poll(() => page.evaluate(() => (window as Window & { playAttempts?: number }).playAttempts || 0)).toBeGreaterThan(0);
  await expect(page.locator("[data-player-status] [data-player-fallback]")).toBeHidden();
  await page.clock.fastForward(12_100);
  await expect(page.locator("[data-player-status] [data-player-fallback]")).toHaveText("Retry playback");
  await expect(page.locator("[data-player-message]")).toContainText("Playback has not advanced");
  expect(await page.evaluate(() => (window as Window & { FakeHls: { instances: number } }).FakeHls.instances)).toBe(2);
  await page.locator("[data-player-status] [data-player-fallback]").click();
  await expect.poll(() => page.evaluate(() => (window as Window & { FakeHls: { instances: number } }).FakeHls.instances)).toBe(3);
  await expect(video).toHaveJSProperty("currentTime", 42);
});

test("pausing a waiting stream cancels automatic recovery", async ({ page }) => {
  await startDirectPlayer(page, { initialSource: false, initialHls: true, compatibleMode: "audio-transcode" });
  await page.clock.install();
  const video = page.locator("video");
  await video.dispatchEvent("waiting");
  await video.dispatchEvent("pause");
  await page.clock.fastForward(25_000);
  expect(await page.evaluate(() => (window as Window & { FakeHls: { instances: number } }).FakeHls.instances)).toBe(1);
  await expect(page.locator("[data-player-status] [data-player-fallback]")).toBeHidden();
});

test("an audio-compatible media network error resumes at its saved position", async ({ page }) => {
  await startDirectPlayer(page, { initialSource: false, initialHls: true, compatibleMode: "audio-transcode" });
  await page.clock.install();
  const video = page.locator("video");
  await video.evaluate((media: HTMLVideoElement) => {
    media.currentTime = 42;
    media.dispatchEvent(new Event("timeupdate"));
  });
  await failDirect(page, 2);
  await expect(page.locator("[data-player-status]")).toContainText("Connection interrupted. Reconnecting…");
  await expect(page.locator("[data-player-status]")).toHaveAttribute("aria-busy", "true");
  await page.clock.fastForward(5_000);
  expect(await page.evaluate(() => (window as Window & { FakeHls: { instances: number } }).FakeHls.instances)).toBe(2);
  await expect(video).toHaveJSProperty("currentTime", 42);
  await expect.poll(() => page.evaluate(() => (window as Window & { playAttempts?: number }).playAttempts || 0)).toBeGreaterThan(0);
  await expect(page.locator("[data-player-status] [data-player-fallback]")).toBeHidden();
});

test("an unsupported audio-compatible media source offers a working retry", async ({ page }) => {
  await startDirectPlayer(page, { initialSource: false, initialHls: true, compatibleMode: "audio-transcode" });
  const video = page.locator("video");
  await video.evaluate((media: HTMLVideoElement) => {
    media.currentTime = 42;
    media.dispatchEvent(new Event("timeupdate"));
  });
  await failDirect(page, 4);
  await expect(page.locator("[data-player-status] [data-player-fallback]")).toHaveText("Retry playback");
  await page.locator("[data-player-status] [data-player-fallback]").click();
  await expect.poll(() => page.evaluate(() => (window as Window & { FakeHls: { instances: number } }).FakeHls.instances)).toBe(2);
  await expect(video).toHaveJSProperty("currentTime", 42);
});

test("audio-compatible network retries stop after a bounded budget", async ({ page }) => {
  await startDirectPlayer(page, { initialSource: false, initialHls: true, compatibleMode: "audio-transcode" });
  await page.clock.install();
  const video = page.locator("video");
  await video.evaluate((media: HTMLVideoElement) => {
    media.currentTime = 42;
    media.dispatchEvent(new Event("timeupdate"));
  });
  for (let attempt = 0; attempt < 3; attempt++) {
    await failDirect(page, 2);
    await page.clock.fastForward(5_000);
    expect(await page.evaluate(() => (window as Window & { FakeHls: { instances: number } }).FakeHls.instances)).toBe(attempt + 2);
  }
  await failDirect(page, 2);
  await page.clock.fastForward(5_000);
  expect(await page.evaluate(() => (window as Window & { FakeHls: { instances: number } }).FakeHls.instances)).toBe(4);
  await expect(page.locator("[data-player-status] [data-player-fallback]")).toHaveText("Retry playback");
  await page.locator("[data-player-status] [data-player-fallback]").click();
  await expect.poll(() => page.evaluate(() => (window as Window & { FakeHls: { instances: number } }).FakeHls.instances)).toBe(5);
});

test("audio-compatible decode errors recover once before offering retry", async ({ page }) => {
  await startDirectPlayer(page, { initialSource: false, initialHls: true, compatibleMode: "audio-transcode" });
  await failDirect(page, 3);
  expect(await page.evaluate(() => (window as Window & { mediaRecoveries?: number }).mediaRecoveries)).toBe(1);
  await failDirect(page, 3);
  await expect(page.locator("[data-player-status] [data-player-fallback]")).toHaveText("Retry playback");
  expect(await page.evaluate(() => (window as Window & { mediaRecoveries?: number }).mediaRecoveries)).toBe(1);
});

test("an aborted audio-compatible source switch does not trigger recovery", async ({ page }) => {
  await startDirectPlayer(page, { initialSource: false, initialHls: true, compatibleMode: "audio-transcode" });
  await failDirect(page, 1);
  expect(await page.evaluate(() => (window as Window & { FakeHls: { instances: number } }).FakeHls.instances)).toBe(1);
  await expect(page.locator("[data-player-status] [data-player-fallback]")).toBeHidden();
});
