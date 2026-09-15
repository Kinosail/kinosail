import { playerSource } from "./static-sources";
import { expect, Page, test } from "@playwright/test";

async function startPlayer(page: Page, duration?: string) {
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await page.route("https://127.0.0.1:38128/", (route) => route.fulfill({ contentType: "text/html", body: `
    <body>
      <video data-hls="/movie.m3u8" ${duration === undefined ? "" : `data-duration="${duration}"`} data-progress="/progress/movie"></video>
      <div data-quality-control hidden><select data-quality></select><span data-quality-state></span></div>
    </body>
  ` }));
  await page.setContent(`<html><head><base href="https://127.0.0.1:38128/"></head><body>
    <video data-hls="/movie.m3u8" ${duration === undefined ? "" : `data-duration="${duration}"`} data-progress="/progress/movie"></video>
    <div data-quality-control hidden><select data-quality></select><span data-quality-state></span></div>
  </body></html>`);
  await page.evaluate(() => {
    try { Object.defineProperty(window, "localStorage", { value: { getItem: () => null, setItem: () => {} } }); } catch {}
    class FakeHls {
      static Events = { MANIFEST_PARSED: "manifest", LEVEL_SWITCHED: "switched", LEVEL_UPDATED: "updated", ERROR: "error" };
      static ErrorTypes = { NETWORK_ERROR: "network", MEDIA_ERROR: "media" };
      static isSupported = () => true;
      mediaSource = { duration: 220 };
      handlers = new Map<string, (...args: (string | object)[]) => void>();
      autoLevelEnabled = true;
      levels = [];
      constructor() { Object.assign(window, { hlsInstance: this }); }
      on(event: string, handler: (...args: (string | object)[]) => void) { this.handlers.set(event, handler); }
      loadSource() {}
      attachMedia(input: HTMLMediaElement | { overrides?: { duration?: number } }) {
        if ("overrides" in input && input.overrides?.duration) this.mediaSource.duration = input.overrides.duration;
      }
      trigger(event: string) { this.handlers.get(event)?.(event, {}); }
    }
    Object.assign(window, { Hls: FakeHls });
  });
  await page.addScriptTag({ content: playerSource });
  const actual = await page.evaluate(() => {
    const instance = (window as Window & { hlsInstance?: { trigger(event: string): void; mediaSource: { duration: number } } }).hlsInstance;
    return instance?.mediaSource.duration;
  });
  expect(errors).toEqual([]);
  return actual;
}

test("compatible playback keeps the full movie duration while HLS is still growing", async ({ page }) => {
  expect(await startPlayer(page, "7200")).toBe(7200);
});

for (const [name, duration] of [["missing", undefined], ["malformed", "not-a-duration"], ["negative", "-1"], ["oversized", "31622401"]] as const) {
  test(`compatible playback rejects a ${name} expected duration`, async ({ page }) => {
    expect(await startPlayer(page, duration)).toBe(220);
  });
}
