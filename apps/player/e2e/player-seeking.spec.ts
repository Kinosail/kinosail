import { playerSource } from "./static-sources";
import { expect, test } from "@playwright/test";
import type { Page } from "@playwright/test";
import {writeFile} from "node:fs/promises";

test("compatible playback resumes and seeks from the requested HLS window", async ({ page }) => {
  await page.route("https://127.0.0.1:38127/", (route) => route.fulfill({ contentType: "text/html", body: `
    <body>
      <video data-hls="/hls/movie/p/a-a0-s0-none-t0-b0/index.m3u8" data-duration="7200" data-start="2940" data-progress="/progress/movie"></video>
      <div data-quality-control hidden><select data-quality></select><span data-quality-state></span></div>
    </body>
  ` }));
  await page.setContent(`<html><head><base href="https://127.0.0.1:38127/"></head><body>
    <video data-hls="/hls/movie/p/a-a0-s0-none-t0-b0/index.m3u8" data-duration="7200" data-start="2940" data-progress="/progress/movie"></video>
    <div data-quality-control hidden><select data-quality></select><span data-quality-state></span></div>
  </body></html>`);
  await page.evaluate(() => {
    try { Object.defineProperty(window, "localStorage", { value: { getItem: () => null, setItem: () => {} } }); } catch {}
    Object.defineProperties(document.querySelector("video"), {
      currentTime: { value: 0, writable: true },
      duration: { value: 7200 },
      paused: { value: false },
      load: { value() {} },
      play: { value: async () => {} },
    });
    class FakeHls {
      static Events = { MANIFEST_PARSED: "manifest", LEVEL_SWITCHED: "switched", ERROR: "error" };
      static ErrorTypes = { NETWORK_ERROR: "network", MEDIA_ERROR: "media" };
      static isSupported = () => true;
      static instances: FakeHls[] = [];
      handlers = new Map<string, (...args: (string | object)[]) => void>();
      autoLevelEnabled = true;
      levels = [];
      latestLevelDetails = { fragments: [{ start: 2880, duration: 60 }], edge: 2940 };
      source = "";
      constructor(public config: { startPosition?: number; timelineOffset?: number }) { FakeHls.instances.push(this); }
      on(event: string, handler: (...args: (string | object)[]) => void) { this.handlers.set(event, handler); }
      loadSource(source: string) { this.source = source; }
      attachMedia() {}
      destroy() {}
    }
    Object.assign(window, { Hls: FakeHls, FakeHls });
  });
  await page.addScriptTag({ content: playerSource });

  await expect.poll(() => page.evaluate(() => {
    const instance = (window as Window & { FakeHls: { instances: Array<{ config: { startPosition?: number; timelineOffset?: number }; source: string }> } }).FakeHls.instances[0];
    return { config: instance?.config, source: instance?.source };
  })).toEqual({ config: expect.objectContaining({ startPosition: 0, timelineOffset: 2940 }), source: "/hls/movie/p/a-a0-s0-none-t0-b0-o2940000/index.m3u8" });

  await page.locator("video").evaluate((video) => { video.currentTime = 5123; });
  await page.locator("video").dispatchEvent("seeking");
  await expect.poll(() => page.evaluate(() => {
    const instances = (window as Window & { FakeHls: { instances: Array<{ config: { startPosition?: number; timelineOffset?: number }; source: string }> } }).FakeHls.instances;
    const instance = instances.at(-1);
    return { count: instances.length, config: instance?.config, source: instance?.source };
  })).toEqual({ count: 2, config: expect.objectContaining({ startPosition: 0, timelineOffset: 5123 }), source: "/hls/movie/p/a-a0-s0-none-t0-b0-o5123000/index.m3u8" });
});

for (const ranges of ["seekable", "buffered"]) test(`native HLS keeps seeks inside ${ranges} media and rebuilds outside its window`, async ({ page }) => {
  await page.setContent(`<html><head><base href="https://127.0.0.1:38127/"></head><body>
    <video data-autoplay data-hls="/hls/movie/p/a-a0-s0-none-t0-b0/index.m3u8?playbackSession=session" data-duration="7200" data-start="271.607" data-progress="/progress/movie"></video>
    <div data-quality-control hidden><select data-quality></select><span data-quality-state></span></div>
  </body></html>`);
  await page.locator("video").evaluate((video) => Object.defineProperties(video, {
    canPlayType: { value: (type: string) => type === "application/vnd.apple.mpegurl" ? "probably" : "" },
    play: { value: () => { video.dataset.playCalls = String(Number(video.dataset.playCalls || 0) + 1); return Promise.resolve(); } },
  }));
  await page.addScriptTag({ content: playerSource });

  await expect.poll(() => page.locator("video").evaluate((video) => {
    const source = new URL(video.src);
    return { path: source.pathname, start: source.searchParams.get("start") };
  })).toEqual({ path: "/hls/movie/p/a-a0-s0-none-t0-b0-o271600/index.m3u8", start: null });

  expect(await page.locator("video").evaluate((video) => {
    Object.defineProperty(video, "duration", {configurable: true, value: 7200});
    video.dispatchEvent(new Event("loadedmetadata"));
    const mediaTime = Object.getOwnPropertyDescriptor(HTMLMediaElement.prototype, "currentTime")!;
    return {playCalls: Number(video.dataset.playCalls || 0), timeline: video.currentTime, source: mediaTime.get!.call(video)};
  })).toEqual({playCalls: 0, timeline: 271.6, source: 0});

  await page.locator("video").evaluate((video, name) => Object.defineProperty(video, name, {
    value: { length: 2, start: (index: number) => [0, 10][index], end: (index: number) => [5, 30][index] },
  }), ranges);
  const originalSource = await page.locator("video").getAttribute("src");
  for (const seconds of [272.25, 276.6, 281.6, 301.6]) {
    await page.locator("video").evaluate((video, target) => { video.currentTime = target; video.dispatchEvent(new Event("seeking")); video.dispatchEvent(new Event("seeked")); }, seconds);
    await expect(page.locator("video")).toHaveAttribute("src", originalSource!);
  }
  // A gap between loaded ranges also needs a new server window.
  await page.locator("video").evaluate((video) => { video.currentTime = 279.25; video.dispatchEvent(new Event("seeking")); });
  await expect(page.locator("video")).toHaveAttribute("src", /-o279200\/index.m3u8/);
  await page.locator("video").dispatchEvent("loadedmetadata");
  await page.locator("video").evaluate((video) => { video.currentTime = 120.25; video.dispatchEvent(new Event("seeking")); });
  await expect.poll(() => page.locator("video").evaluate((video) => {
    const source = new URL(video.src);
    return { path: source.pathname, start: source.searchParams.get("start") };
  })).toEqual({ path: "/hls/movie/p/a-a0-s0-none-t0-b0-o120200/index.m3u8", start: null });
});

async function slowNativeHLS(page: Page) {
  await page.clock.install();
  await page.setContent(`<html><head><base href="https://127.0.0.1:38127/"></head><body>
    <div class="media-stage"><video data-hls="/hls/movie/p/a-a0-s0-none-t0-b0/index.m3u8" data-duration="7200" data-start="120" data-progress="/progress/movie"></video>
      <div data-player-status><span class="buffer-skeleton"></span><span data-player-message>Loading video…</span><progress data-buffered max="100"></progress></div>
    </div><div data-quality-control hidden><select data-quality></select><span data-quality-state></span></div>
  </body></html>`);
  await page.locator("video").evaluate((video) => Object.defineProperties(video, {
    canPlayType: { value: (type: string) => type === "application/vnd.apple.mpegurl" ? "probably" : "" },
    paused: { configurable: true, value: false },
    readyState: { configurable: true, value: 1 },
    networkState: { configurable: true, value: 2 },
    src: { configurable: true, value: "", writable: true },
    load: { value: () => { video.dataset.loadCalls = String(Number(video.dataset.loadCalls || 0) + 1); } },
  }));
  await page.addScriptTag({ content: playerSource });
  await expect(page.locator("video")).toHaveAttribute("data-load-calls", "1");
}

test("native HLS keeps a slow first play request alive while the stream loads", async ({ page }) => {
  await slowNativeHLS(page);
  await page.locator("video").dispatchEvent("waiting");
  await page.clock.fastForward("00:00:12");
  await expect(page.locator("video")).toHaveAttribute("data-load-calls", "1");
  await expect(page.locator("[data-player-status]")).not.toHaveClass(/is-recovery/);

  await page.clock.fastForward("00:00:53");
  await expect(page.locator("video")).toHaveAttribute("data-load-calls", "1");
  await expect(page.locator("[data-player-status]")).not.toHaveClass(/is-recovery/);

  await page.locator("video").dispatchEvent("playing");
  await expect(page.locator("[data-player-status]")).toBeHidden();
  await page.locator("video").dispatchEvent("waiting");
  await page.clock.fastForward("00:00:12");
  await expect(page.locator("video")).toHaveAttribute("data-load-calls", "2");
});

test("native HLS offers recovery when the first play never becomes ready", async ({ page }) => {
  await slowNativeHLS(page);
  await page.locator("video").dispatchEvent("waiting");
  await page.clock.fastForward("00:01:36");
  await expect(page.locator("video")).toHaveAttribute("data-load-calls", "1");
  await expect(page.locator("[data-player-status]")).toHaveClass(/is-recovery/);
  await expect(page.locator("[data-player-message]")).toHaveText("Playback has not advanced. You can retry from this position.");
});

test("seeking to the start before initial metadata does not restore the resume position", async ({ page }) => {
  await page.route("https://127.0.0.1:38127/", (route) => route.fulfill({ contentType: "text/html", body: `
    <body>
      <video data-hls="/hls/movie/p/a-a0-s0-none-t0-b0/index.m3u8" data-duration="7200" data-start="2940"></video>
      <div data-quality-control hidden><select data-quality></select><span data-quality-state></span></div>
    </body>
  ` }));
  await page.setContent(`<html><head><base href="https://127.0.0.1:38127/"></head><body>
    <video data-hls="/hls/movie/p/a-a0-s0-none-t0-b0/index.m3u8" data-duration="7200" data-start="2940"></video>
    <div data-quality-control hidden><select data-quality></select><span data-quality-state></span></div>
  </body></html>`);
  await page.evaluate(() => {
    try { Object.defineProperty(window, "localStorage", { value: { getItem: () => null, setItem: () => {} } }); } catch {}
    const media = document.querySelector("video")!;
    Object.defineProperties(media, {
      currentTime: { value: 0, writable: true },
      duration: { value: 7200 },
      paused: { value: false },
      load: { value() {} },
      play: { value: async () => {} },
    });
    class FakeHls {
      static Events = { MANIFEST_PARSED: "manifest", LEVEL_SWITCHED: "switched", ERROR: "error" };
      static ErrorTypes = { NETWORK_ERROR: "network", MEDIA_ERROR: "media" };
      static isSupported = () => true;
      latestLevelDetails = { fragments: [{ start: 2880, duration: 60 }], edge: 2940 };
      constructor() {}
      on() {}
      loadSource() {}
      attachMedia() {}
      destroy() {}
    }
    Object.assign(window, { Hls: FakeHls });
  });
  await page.addScriptTag({ content: playerSource });

  await page.locator("video").evaluate((video) => {
    video.currentTime = 0;
    video.dispatchEvent(new Event("seeking"));
    video.dispatchEvent(new Event("loadedmetadata"));
  });

  await expect(page.locator("video")).toHaveJSProperty("currentTime", 0);
});

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
    const state = {raw: 0, duration: 70, ready: 0, loads: 0, source: "", lastLoad: {position: 0, duration: 0}};
    const video = document.querySelector("video")!;
    Object.defineProperties(HTMLMediaElement.prototype, {
      currentTime: {configurable: true, get: () => state.raw, set: (value: number) => { state.raw = value; }},
      duration: {configurable: true, get: () => state.ready ? state.duration : NaN},
    });
    Object.defineProperties(video, {
      readyState: {configurable: true, get: () => state.ready},
      paused: {configurable: true, get: () => true},
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
      play: {value: () => Promise.resolve()},
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
  await expect.poll(() => writes).toEqual([22]);
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
