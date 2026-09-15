import { playerSource } from "./static-sources";
import { expect, test } from "@playwright/test";

test("compatible playback resumes and seeks from the requested HLS window", async ({ page }) => {
  await page.route("https://127.0.0.1:38128/", (route) => route.fulfill({ contentType: "text/html", body: `
    <body>
      <video data-hls="/hls/movie/p/a-a0-s0-none-t0-b0/index.m3u8" data-duration="7200" data-start="2940" data-progress="/progress/movie"></video>
      <div data-quality-control hidden><select data-quality></select><span data-quality-state></span></div>
    </body>
  ` }));
  await page.setContent(`<html><head><base href="https://127.0.0.1:38128/"></head><body>
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
  })).toEqual({ count: 2, config: expect.objectContaining({ startPosition: 23, timelineOffset: 5100 }), source: "/hls/movie/p/a-a0-s0-none-t0-b0-o5100000/index.m3u8" });
});

test("seeking to the start before initial metadata does not restore the resume position", async ({ page }) => {
  await page.route("https://127.0.0.1:38128/", (route) => route.fulfill({ contentType: "text/html", body: `
    <body>
      <video data-hls="/hls/movie/p/a-a0-s0-none-t0-b0/index.m3u8" data-duration="7200" data-start="2940"></video>
      <div data-quality-control hidden><select data-quality></select><span data-quality-state></span></div>
    </body>
  ` }));
  await page.setContent(`<html><head><base href="https://127.0.0.1:38128/"></head><body>
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
