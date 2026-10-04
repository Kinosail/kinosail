import { playerSource } from "./static-sources";
import { expect, test } from "@playwright/test";
import type { Page } from "@playwright/test";

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
