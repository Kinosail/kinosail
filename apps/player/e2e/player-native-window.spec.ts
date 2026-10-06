import { playerSource } from "./static-sources";
import { expect, test } from "@playwright/test";
import type { Page } from "@playwright/test";
import {writeFile} from "node:fs/promises";

// Isolated exception: a populated Chromium decoder cannot deterministically retain
// old native-HLS metadata while a replacement window is selected. Real public
// checkpoint journeys separately prove accepted storage and direct decoded reentry.
async function nativeTimelineWindow(page: Page, duration: number, start: number) {
  const writes: number[] = [];
  await page.route("**/progress/movie", async route => {
    writes.push(Number(new URLSearchParams(route.request().postData() || "").get("seconds")));
    await route.fulfill({status: 204});
  });
  const markup = `<html><head><base href="https://127.0.0.1:38127/"></head><body data-viewer-profile="fixture-viewer">
    <video data-hls="/hls/movie/p/a-a0-s0-none-t0-b0/index.m3u8" data-duration="${duration}" data-start="${start}"
      data-progress="/progress/movie" data-playback-session="fixture-native-session"></video>
    <div data-quality-control hidden><select data-quality></select><span data-quality-state></span></div>
  </body></html>`;
  await page.route("https://127.0.0.1:38127/", route => route.fulfill({contentType: "text/html", body: markup}));
  await page.goto("https://127.0.0.1:38127/");
  await page.evaluate(() => {
    const state = {raw: 0, duration: 70, ready: 0, paused: true, loads: 0, source: "", lastLoad: {position: 0, duration: 0}};
    const video = document.querySelector("video")!;
    Object.defineProperties(HTMLMediaElement.prototype, {
      currentTime: {configurable: true, get: () => state.raw, set: (value: number) => { state.raw = value; }},
      duration: {configurable: true, get: () => state.ready ? state.duration : NaN},
    });
    Object.defineProperties(video, {
      readyState: {configurable: true, get: () => state.ready},
      paused: {configurable: true, get: () => state.paused},
      buffered: {configurable: true, value: {length: 0}},
      seekable: {configurable: true, value: {length: 0}},
      canPlayType: {value: (type: string) => type === "application/vnd.apple.mpegurl" ? "probably" : ""},
      src: {configurable: true, get: () => state.source, set: (value: string) => {
        state.source = new URL(value, document.baseURI).href;
        // A replacement can queue a Pause from the still-visible old decoder.
        if (state.loads) {
          video.dispatchEvent(new Event("pause"));
          video.dispatchEvent(new Event("seeked"));
        }
      }},
      load: {value: () => {
        state.loads++;
        state.lastLoad = {position: video.currentTime, duration: video.duration};
      }},
      play: {value: () => {
        state.paused = false;
        video.dispatchEvent(new Event("play"));
        video.dispatchEvent(new Event("playing"));
        return Promise.resolve();
      }},
      pause: {value: () => { state.paused = true; video.dispatchEvent(new Event("pause")); }},
    });
    Object.assign(window, {nativeTimeline: state});
  });
  await page.addScriptTag({content: playerSource});
  await expect.poll(() => page.evaluate(() => (window as unknown as {nativeTimeline: {loads: number}}).nativeTimeline.loads)).toBe(1);
  return writes;
}

test("native HLS keeps a saved source window when the full duration is unknown @smoke", async ({page}, info) => {
  const writes = await nativeTimelineWindow(page, 0, 22);
  const path = await page.locator("video").evaluate(video => new URL(video.src).pathname);
  await writeFile(info.outputPath("unknown-duration-window.json"), JSON.stringify({path, expectedOffset: 22, fullDuration: "unknown"}));
  expect(path).toMatch(/-o22000\/index\.m3u8$/);
  await page.evaluate(() => {
    const state = (window as unknown as {nativeTimeline: {raw: number; duration: number; ready: number}}).nativeTimeline;
    state.raw = 0; state.duration = 10; state.ready = 2;
    document.querySelector("video")!.dispatchEvent(new Event("loadedmetadata"));
    document.querySelector("video")!.dispatchEvent(new Event("pause"));
  });
  await expect(page.locator("video")).toHaveJSProperty("currentTime", 22);
  await page.waitForTimeout(100);
  expect(writes).toEqual([]);
  await page.locator("video").evaluate(async video => { await video.play(); video.pause(); });
  await expect.poll(() => writes).toEqual([22]);
  await page.locator("video").evaluate(video => video.play());
  await page.evaluate(() => {
    (window as unknown as {nativeTimeline: {raw: number}}).nativeTimeline.raw = 1.25;
    document.querySelector("video")!.dispatchEvent(new Event("timeupdate"));
  });
  await expect(page.locator("video")).toHaveJSProperty("currentTime", 23.25);
  await page.locator("video").evaluate(video => video.pause());
  await expect.poll(() => writes).toEqual([22, 23.25]);
  await writeFile(info.outputPath("unknown-duration-checkpoint.json"), JSON.stringify({beforePlayback: [], afterExplicitPlayback: writes, fullDuration: "unknown", sourceOffset: 22, decoderPosition: 1.25}));
});

test("native HLS replacement checkpoints the requested source position while old decoder metadata remains @smoke", async ({page}, info) => {
  const writes = await nativeTimelineWindow(page, 70, 0);
  await page.evaluate(() => {
    const state = (window as unknown as {nativeTimeline: {raw: number; duration: number; ready: number}}).nativeTimeline;
    state.raw = 12; state.ready = 2;
    const video = document.querySelector("video")!;
    video.dispatchEvent(new Event("loadedmetadata"));
    video.currentTime = 22;
    video.dispatchEvent(new Event("seeking"));
  });
  await expect.poll(() => page.evaluate(() => (window as unknown as {nativeTimeline: {loads: number}}).nativeTimeline.loads)).toBe(2);
  await expect.poll(() => writes.length).toBeGreaterThan(0);
  const snapshot = await page.evaluate(() => (window as unknown as {nativeTimeline: {lastLoad: {position: number; duration: number}}}).nativeTimeline.lastLoad);
  await writeFile(info.outputPath("replacement-window-checkpoint.json"), JSON.stringify({snapshot, writes, expectedPosition: 22, fullDuration: 70}));
  await expect.poll(() => writes).toEqual([22, 22]);
  expect(snapshot).toEqual({position: 22, duration: 70});
  await page.evaluate(() => {
    const state = (window as unknown as {nativeTimeline: {raw: number; duration: number}}).nativeTimeline;
    state.raw = 1.25; state.duration = 10;
    const video = document.querySelector("video")!;
    video.dispatchEvent(new Event("loadedmetadata"));
    video.dispatchEvent(new Event("pause"));
  });
  await expect(page.locator("video")).toHaveJSProperty("currentTime", 23.25);
  await expect(page.locator("video")).toHaveJSProperty("duration", 70);
  await expect.poll(() => writes).toEqual([22, 22, 23.25]);
});

test("native HLS bounds source offsets when duration is unknown @smoke", async ({page}) => {
  for (const start of [-1, NaN, Infinity, 604800.1, 604801]) {
    await nativeTimelineWindow(page, 0, start);
    const path = await page.locator("video").evaluate(video => new URL(video.src).pathname);
    expect(path).toBe("/hls/movie/p/a-a0-s0-none-t0-b0/index.m3u8");
  }
  await nativeTimelineWindow(page, 0, 604800);
  const path = await page.locator("video").evaluate(video => new URL(video.src).pathname);
  expect(path).toMatch(/-o604800000\/index\.m3u8$/);
});
