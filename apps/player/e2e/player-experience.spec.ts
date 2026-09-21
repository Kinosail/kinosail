import { expect, test } from "@playwright/test";
import { installPlayerExperienceFixture } from "./player-experience-fixture";

installPlayerExperienceFixture();

test("offline source swap preserves active playback position and state", async ({ page }) => {
  const video = page.locator("video");
  await page.evaluate(() => {
    const context = window as Window & { resolveOfflineSource: () => void; setPaused: (value: boolean) => void };
    const player = document.querySelector("video")!;
    player.currentTime = 42;
    context.setPaused(false);
    context.resolveOfflineSource();
  });
  await expect(video).toHaveJSProperty("src", "/offline-media/profile/movie-job");
  await expect(page.locator("[data-playback-mode-status]")).toHaveText("Offline copy…");
  await video.dispatchEvent("loadedmetadata");
  await video.dispatchEvent("loadeddata");
  await expect.poll(() => video.evaluate((element: HTMLVideoElement) => element.currentTime)).toBe(42);
  await expect.poll(() => video.evaluate((element: HTMLVideoElement) => element.paused)).toBe(false);
  await expect(page.locator("[data-playback-mode-status]")).toHaveText("Offline copy");
  await expect(page.locator("[data-playback-method-detail]")).toHaveText("Offline copy");
});

test("localized offline source swap restores saved progress before metadata", async ({ page }) => {
  const video = page.locator("video");
  await page.evaluate(() => (window as Window & { resolveOfflineSource: () => void }).resolveOfflineSource());
  await expect(video).toHaveJSProperty("src", "/offline-media/profile/movie-job");
  await expect(page.locator("[data-playback-mode-status]")).toHaveText("Copia local…");
  await video.dispatchEvent("loadedmetadata");
  await video.dispatchEvent("loadeddata");
  await expect.poll(() => video.evaluate((element: HTMLVideoElement) => element.currentTime)).toBe(20);
  await expect.poll(() => video.evaluate((element: HTMLVideoElement) => element.paused)).toBe(true);
  await expect(page.locator("[data-playback-reason]")).toHaveText("Copia local · Archivo verificado en este dispositivo.");
  await expect(page.locator("[data-playback-mode-status]")).toHaveAttribute("aria-label", "Método de reproducción: Copia local. Abrir ajustes.");
});

test("offline source swap removes an unavailable copy and restores the network source", async ({ page }) => {
  const video = page.locator("video");
  await page.evaluate(() => {
    const context = window as Window & { resolveOfflineSource: () => void; setOfflineProbeStatus: (status: number) => void };
    context.setOfflineProbeStatus(404);
    context.resolveOfflineSource();
  });
  await expect(video).toHaveJSProperty("src", "/offline-media/profile/movie-job");
  await expect(page.locator("[data-playback-mode-status]")).toHaveText("Offline copy…");
  await video.dispatchEvent("error");
  await expect(video).toHaveJSProperty("src", "/media/movie");
  await video.dispatchEvent("loadedmetadata");
  await expect.poll(() => video.evaluate((element: HTMLVideoElement) => element.currentTime)).toBe(20);
  await expect(page.locator("[data-playback-mode-status]")).toHaveText("Direct Play");
  await expect(page.locator("[data-player-message]")).toHaveText("Offline copy is unavailable.");
  await expect(page.locator("[data-playback-recovery]")).toBeHidden();
  await expect.poll(() => page.evaluate(() => (window as Window & { removedOffline: string[] }).removedOffline)).toEqual(["movie-job"]);
  expect(await video.getAttribute("data-offline")).toBeNull();
});

test("offline source swap restores Direct Play instead of a stale HLS fallback blob", async ({ page }) => {
  const video = page.locator("video");
  await page.evaluate(() => {
    const context = window as Window & { resolveOfflineSource: () => void; setOfflineProbeStatus: (status: number) => void };
    context.setOfflineProbeStatus(404);
    context.resolveOfflineSource();
  });
  await expect(video).toHaveJSProperty("src", "/offline-media/profile/movie-job");
  await video.dispatchEvent("error");
  await expect(video).toHaveJSProperty("src", "/media/direct");
  await expect(page.locator("[data-playback-recovery]")).toBeHidden();
  await expect.poll(() => page.evaluate(() => (window as Window & { removedOffline: string[] }).removedOffline)).toEqual(["movie-job"]);
});

test("offline source swap reports Direct Play after an active HLS blob fails", async ({ page }) => {
  const video = page.locator("video");
  await expect(video).toHaveJSProperty("src", "blob:stream");
  await page.evaluate(() => {
    const context = window as Window & { resolveOfflineSource: () => void; setOfflineProbeStatus: (status: number) => void };
    context.setOfflineProbeStatus(404);
    context.resolveOfflineSource();
  });
  await expect(video).toHaveJSProperty("src", "/offline-media/profile/movie-job");
  await video.dispatchEvent("error");
  await expect(video).toHaveJSProperty("src", "/media/direct");
  await expect(page.locator("[data-playback-mode-status]")).toHaveText("Direct Play");
  await expect(page.locator("[data-quality-state]")).toHaveText("Original");
});

test("offline source swap hides stale active HLS blob quality after local metadata loads", async ({ page }) => {
  const video = page.locator("video");
  await page.evaluate(() => (window as Window & { resolveOfflineSource: () => void }).resolveOfflineSource());
  await expect(video).toHaveJSProperty("src", "/offline-media/profile/movie-job");
  await video.dispatchEvent("loadedmetadata");
  await video.dispatchEvent("loadeddata");
  await expect(page.locator("[data-playback-mode-status]")).toHaveText("Offline copy");
  await expect(page.locator("[data-quality-control]")).toBeHidden();
});

test("offline source swap active HLS blob manual Direct Play cancels local metadata state", async ({ page }) => {
  const video = page.locator("video");
  await page.evaluate(() => (window as Window & { resolveOfflineSource: () => void }).resolveOfflineSource());
  await expect(video).toHaveJSProperty("src", "/offline-media/profile/movie-job");
  await expect(page.locator("[data-quality-control]")).toBeHidden();
  await page.locator('[data-playback-mode] input[value="direct-only"]').check();
  await expect(video).toHaveJSProperty("src", "/media/direct");
  await expect(page.locator("[data-quality-control]")).toBeVisible();
  await expect(page.locator("[data-quality-state]")).toHaveText("Original");
  await video.dispatchEvent("loadedmetadata");
  await expect(page.locator("[data-player-message]")).not.toHaveText("Ready offline on this device");
});

test("offline source swap restarts server-skip active HLS blob without a direct source during active playback", async ({ page }) => {
  const video = page.locator("video");
  await expect(video).toHaveJSProperty("src", "blob:stream");
  await expect.poll(() => page.evaluate(() => (window as Window & { hlsSources: string[] }).hlsSources)).toEqual(["/hls/movie-o20000/index.m3u8"]);
  await page.evaluate(() => {
    const context = window as Window & { resolveOfflineSource: () => void; setOfflineProbeStatus: (status: number) => void; setPaused: (value: boolean) => void };
    const player = document.querySelector("video")!;
    player.currentTime = 42;
    context.setPaused(false);
    context.setOfflineProbeStatus(404);
    context.resolveOfflineSource();
  });
  await expect(video).toHaveJSProperty("src", "/offline-media/profile/movie-job");
  await video.dispatchEvent("error");
  await expect(video).toHaveJSProperty("src", "blob:stream");
  await expect.poll(() => page.evaluate(() => (window as Window & { hlsSources: string[] }).hlsSources)).toEqual([
    "/hls/movie-o20000/index.m3u8",
    "/hls/movie-o42000/index.m3u8",
  ]);
  await video.dispatchEvent("loadedmetadata");
  await expect.poll(() => video.evaluate((element: HTMLVideoElement) => element.currentTime)).toBe(42);
  await expect.poll(() => video.evaluate((element: HTMLVideoElement) => element.paused)).toBe(false);
  await expect(page.locator("[data-playback-mode-status]")).toHaveText("Remux");
});

test("offline source swap keeps advanced time when server-skip active HLS blob fails after metadata", async ({ page }) => {
  const video = page.locator("video");
  await expect(video).toHaveJSProperty("src", "blob:stream");
  await page.evaluate(() => {
    const context = window as Window & { resolveOfflineSource: () => void };
    context.resolveOfflineSource();
  });
  await expect(video).toHaveJSProperty("src", "/offline-media/profile/movie-job");
  await video.dispatchEvent("loadedmetadata");
  await video.dispatchEvent("loadeddata");
  await page.evaluate(() => {
    const context = window as Window & { setOfflineProbeStatus: (status: number) => void; setPaused: (value: boolean) => void };
    const player = document.querySelector("video")!;
    player.currentTime = 55;
    context.setPaused(false);
    context.setOfflineProbeStatus(404);
  });
  await video.dispatchEvent("error");
  await expect(video).toHaveJSProperty("src", "blob:stream");
  await expect.poll(() => page.evaluate(() => (window as Window & { hlsSources: string[] }).hlsSources)).toEqual([
    "/hls/movie-o20000/index.m3u8",
    "/hls/movie-o55000/index.m3u8",
  ]);
  await video.dispatchEvent("loadedmetadata");
  await expect.poll(() => video.evaluate((element: HTMLVideoElement) => element.currentTime)).toBe(55);
  await expect.poll(() => video.evaluate((element: HTMLVideoElement) => element.paused)).toBe(false);
});

test("offline source swap keeps an immediately ready offline source during delayed negotiation for server-skip active HLS blob", async ({ page }) => {
  const video = page.locator("video");
  await expect(video).toHaveJSProperty("src", "/offline-media/profile/movie-job");
  await page.evaluate(() => (window as Window & { releaseNegotiation: () => void }).releaseNegotiation());
  await page.waitForTimeout(50);
  await expect(video).toHaveJSProperty("src", "/offline-media/profile/movie-job");
  await expect(page.locator("[data-playback-mode-status]")).toHaveText("Offline copy…");
  await expect.poll(() => page.evaluate(() => (window as Window & { hlsSources: string[] }).hlsSources)).toEqual([]);
});

test("offline source swap restarts server-skip adaptive playback after a pending HLS loader fails locally", async ({ page }) => {
  const video = page.locator("video");
  await page.evaluate(() => (window as Window & { resolveOfflineSource: () => void }).resolveOfflineSource());
  await expect(video).toHaveJSProperty("src", "/offline-media/profile/movie-job");
  await video.dispatchEvent("error");
  await page.evaluate(() => (window as Window & { releaseHlsLoader: () => Promise<void> }).releaseHlsLoader());
  await expect(video).toHaveJSProperty("src", "blob:stream");
  await expect.poll(() => page.evaluate(() => (window as Window & { hlsSources: string[] }).hlsSources)).toEqual(["/hls/movie-o20000/index.m3u8"]);
  await expect(page.locator("[data-playback-mode-status]")).toHaveText("Remux");
  await video.dispatchEvent("loadedmetadata");
  await video.dispatchEvent("canplay");
  await expect(page.locator("[data-playback-mode-status]")).toHaveText("Remux");
});

test("offline source swap cannot replace the network source after player teardown", async ({ page }) => {
  const video = page.locator("video");
  await page.evaluate(() => window.dispatchEvent(new PageTransitionEvent("pagehide")));
  await page.evaluate(() => (window as Window & { resolveOfflineSource: () => void }).resolveOfflineSource());
  await page.waitForTimeout(50);
  await expect(video).toHaveJSProperty("src", "/media/movie");
  expect(await video.getAttribute("data-offline")).toBeNull();
});

test("offline source swap ignores a late media error after player teardown", async ({ page }) => {
  const video = page.locator("video");
  await page.evaluate(() => (window as Window & { resolveOfflineSource: () => void }).resolveOfflineSource());
  await expect(video).toHaveJSProperty("src", "/offline-media/profile/movie-job");
  await page.evaluate(() => window.dispatchEvent(new PageTransitionEvent("pagehide")));
  await video.dispatchEvent("error");
  await expect(video).toHaveJSProperty("src", "/offline-media/profile/movie-job");
  await expect.poll(() => page.evaluate(() => (window as Window & { removedOffline: string[] }).removedOffline)).toEqual([]);
});

test("offline source swap keeps the local source after a delayed direct probe resolves", async ({ page }) => {
  const video = page.locator("video");
  await video.dispatchEvent("error");
  await page.evaluate(() => (window as Window & { resolveOfflineSource: () => void }).resolveOfflineSource());
  await expect(video).toHaveJSProperty("src", "/offline-media/profile/movie-job");
  await page.evaluate(() => (window as Window & { releaseDirectProbe: () => void }).releaseDirectProbe());
  await page.waitForTimeout(50);
  await expect(video).toHaveJSProperty("src", "/offline-media/profile/movie-job");
  await expect(page.locator("[data-playback-mode-status]")).toHaveText("Offline copy…");
});

test("offline source swap rapid playback mode change keeps time and play intent", async ({ page }) => {
  const video = page.locator("video");
  await page.evaluate(async () => {
    const context = window as Window & { setPaused: (value: boolean) => void; setReadyState: (value: number) => void };
    const player = document.querySelector("video")!;
    context.setReadyState(4);
    player.currentTime = 50;
    await player.play();
  });
  await page.locator('[data-playback-mode] input[value="compatible"]').check();
  await expect(video).toHaveJSProperty("src", "blob:stream");
  await video.dispatchEvent("loadedmetadata");
  await expect.poll(() => video.evaluate((element: HTMLVideoElement) => element.currentTime)).toBe(50);
  await expect.poll(() => video.evaluate((element: HTMLVideoElement) => element.paused)).toBe(false);
});

test("offline source swap rapid playback mode keeps pending intent through local failure", async ({ page }) => {
  const video = page.locator("video");
  await page.evaluate(async () => {
    const context = window as Window & { setPaused: (value: boolean) => void; setReadyState: (value: number) => void };
    const player = document.querySelector("video")!;
    context.setReadyState(4);
    player.currentTime = 50;
    await player.play();
  });
  await page.locator('[data-playback-mode] input[value="compatible"]').check();
  await expect(video).toHaveJSProperty("src", "blob:stream");
  await page.evaluate(() => (window as Window & { resolveOfflineSource: () => void }).resolveOfflineSource());
  await expect(video).toHaveJSProperty("src", "/offline-media/profile/movie-job");
  await video.dispatchEvent("error");
  await expect(video).toHaveJSProperty("src", "/media/direct");
  await video.dispatchEvent("loadedmetadata");
  await expect.poll(() => video.evaluate((element: HTMLVideoElement) => element.currentTime)).toBe(50);
  await expect.poll(() => video.evaluate((element: HTMLVideoElement) => element.paused)).toBe(false);
});

test("offline source swap restores native HLS mode after local media fails", async ({ page }) => {
  const video = page.locator("video");
  await expect(video).toHaveJSProperty("src", "/hls/movie-o20000/index.m3u8");
  await page.evaluate(() => {
    const context = window as Window & { resolveOfflineSource: () => void; setOfflineProbeStatus: (status: number) => void };
    context.setOfflineProbeStatus(404);
    context.resolveOfflineSource();
  });
  await expect(video).toHaveJSProperty("src", "/offline-media/profile/movie-job");
  await video.dispatchEvent("error");
  await expect(video).toHaveJSProperty("src", "/hls/movie-o20000/index.m3u8");
  await expect(page.locator("[data-playback-mode-status]")).toHaveText("Remux");
  await expect.poll(() => page.evaluate(() => (window as Window & { removedOffline: string[] }).removedOffline)).toEqual(["movie-job"]);
});

test("offline source swap removes a failed copy after a decoder error", async ({ page }) => {
  const video = page.locator("video");
  await page.evaluate(() => (window as Window & { resolveOfflineSource: () => void }).resolveOfflineSource());
  await expect(video).toHaveJSProperty("src", "/offline-media/profile/movie-job");
  await video.dispatchEvent("error");
  await expect(video).toHaveJSProperty("src", "/media/movie");
  await page.waitForTimeout(50);
  await expect.poll(() => page.evaluate(() => (window as Window & { removedOffline: string[] }).removedOffline)).toEqual(["movie-job"]);
});

test("offline source swap recovers when stored media fails after metadata", async ({ page }) => {
  const video = page.locator("video");
  await page.evaluate(() => (window as Window & { resolveOfflineSource: () => void }).resolveOfflineSource());
  await expect(video).toHaveJSProperty("src", "/offline-media/profile/movie-job");
  await video.dispatchEvent("loadedmetadata");
  await video.dispatchEvent("loadeddata");
  await expect(page.locator("[data-playback-mode-status]")).toHaveText("Offline copy");
  await page.evaluate(() => {
    const context = window as Window & { setOfflineProbeStatus: (status: number) => void; setPaused: (value: boolean) => void };
    const player = document.querySelector("video")!;
    player.currentTime = 55;
    context.setPaused(false);
    context.setOfflineProbeStatus(404);
  });
  await video.dispatchEvent("error");
  await expect(video).toHaveJSProperty("src", "/media/movie");
  await video.dispatchEvent("loadedmetadata");
  await expect.poll(() => video.evaluate((element: HTMLVideoElement) => element.currentTime)).toBe(55);
  await expect.poll(() => video.evaluate((element: HTMLVideoElement) => element.paused)).toBe(false);
  await expect.poll(() => page.evaluate(() => (window as Window & { removedOffline: string[] }).removedOffline)).toEqual(["movie-job"]);
});
