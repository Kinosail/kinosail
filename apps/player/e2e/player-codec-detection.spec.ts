import { expect, test } from "@playwright/test";
import { startDirectPlayer } from "./player-direct-fallback-fixture";

test("native HLS negotiates codecs supported for file playback", async ({ page }) => {
  let requestedCodecs = "";
  await page.route("https://direct.test/api/v1/items/movie/playback?*", route => {
    requestedCodecs = new URL(route.request().url()).searchParams.get("videoCodecs") || "";
    return route.fulfill({ json: {} });
  });
  await startDirectPlayer(page, { preloadHls: false, compatibleMode: "transcode" });
  await page.evaluate(() => {
    const video = document.querySelector("video")!;
    video.dataset.playbackApi = "/api/v1/items/movie/playback";
    video.canPlayType = (type) => type === "application/vnd.apple.mpegurl" ? "probably" : "";
    Object.defineProperty(navigator, "mediaCapabilities", { configurable: true, value: {
      decodingInfo: async ({ type, video: output }: { type: string; video: { contentType: string } }) => ({
        supported: type === "file" && output.contentType.includes("hvc1"), smooth: true, powerEfficient: true,
      }),
    } });
  });
  await page.getByLabel("Compatibility", { exact: true }).check();
  await expect.poll(() => requestedCodecs).toContain("hevc");
});

test("Media Source playback does not trust file-only codec support", async ({ page }) => {
  let negotiations = 0;
  await page.route("https://direct.test/api/v1/items/movie/playback?*", route => {
    negotiations++;
    return route.fulfill({ json: {} });
  });
  await startDirectPlayer(page, { compatibleMode: "transcode" });
  await page.evaluate(() => {
    const video = document.querySelector("video")!;
    video.dataset.playbackApi = "/api/v1/items/movie/playback";
    video.canPlayType = (type) => type === "application/vnd.apple.mpegurl" ? "" : "probably";
    Object.defineProperty(navigator, "mediaCapabilities", { configurable: true, value: undefined });
    Object.defineProperty(MediaSource, "isTypeSupported", { configurable: true, value: () => false });
  });
  await page.getByLabel("Compatibility", { exact: true }).check();
  await expect.poll(() => page.evaluate(() => (window as Window & { FakeHls: { instances: number } }).FakeHls.instances)).toBe(1);
  expect(negotiations).toBe(0);
});

test("codec detection checks the compatible rendition instead of the original 4K file", async ({ page }) => {
  let requestedCodecs = "";
  await page.route("https://direct.test/api/v1/items/movie/playback?*", route => {
    requestedCodecs = new URL(route.request().url()).searchParams.get("videoCodecs") || "";
    return route.fulfill({ json: {} });
  });
  await startDirectPlayer(page, { compatibleMode: "transcode" });
  await page.evaluate(() => {
    const video = document.querySelector("video")!;
    video.dataset.playbackApi = "/api/v1/items/movie/playback";
    video.dataset.mediaWidth = "3840";
    video.dataset.mediaHeight = "2160";
    video.dataset.mediaBitrate = "40000000";
    video.dataset.mediaFramerate = "120";
    Object.defineProperty(navigator, "mediaCapabilities", { configurable: true, value: {
      decodingInfo: async ({ video: output }: { video: { contentType: string; width: number; height: number; bitrate: number; framerate: number } }) => ({
        supported: output.contentType.includes("hvc1") && output.width <= 1920 && output.height <= 1080 && output.bitrate <= 6128000 && output.framerate <= 60,
        smooth: true, powerEfficient: true,
      }),
    } });
  });
  await page.getByLabel("Compatibility", { exact: true }).check();
  await expect.poll(() => requestedCodecs).toContain("hevc");
});
