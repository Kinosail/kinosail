import { playerSource } from "./static-sources";
import { expect, test } from "@playwright/test";

test("cropped video uses its conventional quality tier", async ({ page }) => {
  await page.route("https://127.0.0.1:38127/", (route) => route.fulfill({ contentType: "text/html", body: `
    <body>
      <video data-hls="/movie.m3u8" data-progress="/progress/movie"></video>
      <div data-quality-control hidden><select data-quality aria-label="Stream quality"></select><span data-quality-state></span></div>
    </body>
  ` }));
  await page.setContent(`<html><head><base href="https://127.0.0.1:38127/"></head><body>
    <video data-hls="/movie.m3u8" data-progress="/progress/movie"></video>
    <div data-quality-control hidden><select data-quality aria-label="Stream quality"></select><span data-quality-state></span></div>
  </body></html>`);
  await page.evaluate(() => {
    try { Object.defineProperty(window, "localStorage", { value: { getItem: () => null, setItem: () => {} } }); } catch {}
    class FakeHls {
      static Events = { MANIFEST_PARSED: "manifest", LEVEL_SWITCHED: "switched", ERROR: "error" };
      static ErrorTypes = { NETWORK_ERROR: "network", MEDIA_ERROR: "media" };
      static isSupported = () => true;
      handlers = new Map<string, (...args: (string | object)[]) => void>();
      autoLevelEnabled = true;
      levels = [{ width: 1920, height: 804, url: ["https://quality.test/1080p/index.m3u8"] }];
      constructor() { Object.assign(window, { hlsInstance: this }); }
      on(event: string, handler: (...args: (string | object)[]) => void) { this.handlers.set(event, handler); }
      loadSource() {}
      attachMedia() {}
      trigger(event: string, data: object) { this.handlers.get(event)?.(event, data); }
    }
    Object.assign(window, { Hls: FakeHls });
  });
  await page.addScriptTag({ content: playerSource });
  await page.evaluate(() => {
    const hls = (window as Window & { hlsInstance: { levels: object[]; trigger(event: string, data: object): void } }).hlsInstance;
    hls.trigger("manifest", { levels: hls.levels });
    hls.trigger("switched", { level: 0 });
  });

  await expect(page.getByLabel("Stream quality").locator("option")).toHaveText(["Auto", "1080p"]);
  await expect(page.locator("[data-quality-state]")).toHaveText("Auto · 1080p");
});
