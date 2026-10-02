import { expect, test } from "@playwright/test";
import { playerSource } from "./static-sources";

for (const scenario of [
  { name: "first frame after a paused far seek @smoke", mode: "native", start: 0.083, position: 0, count: 1 },
  { name: "a later first keyframe", mode: "native", start: 2, position: 0, count: 1 },
  { name: "a fractional frame after an hour-long seek @smoke", mode: "native", start: 0.083, position: 0, count: 1, target: 5005.9 },
  { name: "a position already inside the buffer", mode: "native", start: 0, position: 0.5, count: 1 },
  { name: "no buffered frames yet", mode: "native", start: 0, position: 0, count: 0 },
  { name: "direct playback", mode: "direct", start: 0.083, position: 0, count: 1 },
  { name: "MediaSource playback", mode: "mse", start: 0.083, position: 0, count: 1 },
]) test(`native seek readiness preserves pause with ${scenario.name}`, async ({ page }) => {
  const target = "target" in scenario ? scenario.target! : 10.3;
  await page.setContent(`<html><head><base href="https://127.0.0.1:38127/"></head><body>
    <div class="media-stage"><video data-duration="7200" data-start="120" ${scenario.mode === "direct" ? 'data-direct="/media/movie"' : 'data-hls="/hls/movie/p/a-a0-s0-none-t0-b0/index.m3u8"'}></video>
      <div data-player-status><span class="buffer-skeleton"></span><span data-player-message>Loading video…</span><progress data-buffered max="100"></progress></div>
    </div><div data-quality-control hidden><select data-quality></select><span data-quality-state></span></div>
  </body></html>`);
  await page.evaluate((mode) => {
    const video = document.querySelector("video")!;
    let time = 0, ready = 4, paused = true, source = "", start = 0, count = 1;
    // Keep the native timeline adapter in use while controlling decoder events.
    Object.defineProperty(HTMLMediaElement.prototype, "currentTime", {
      configurable: true, get: () => time, set: (value: number) => {
        const next = Math.max(0, value);
        if (time === next) return;
        time = next;
        if (ready) queueMicrotask(() => { video.dispatchEvent(new Event("seeking")); video.dispatchEvent(new Event("seeked")); });
      },
    });
    Object.defineProperties(video, {
      duration: { configurable: true, get: () => 7200 },
      readyState: { get: () => ready },
      networkState: { get: () => 2 },
      paused: { get: () => paused },
      error: { value: null },
      buffered: { get: () => ({ length: count, start: () => start, end: () => 5 }) },
      seekable: { value: { length: 0 } },
      canPlayType: { value: (type: string) => type === "application/vnd.apple.mpegurl" ? "probably" : "" },
      src: { get: () => source, set: (value: string) => { source = value; time = 0; } },
      currentSrc: { get: () => source },
      load: { value: () => { video.dataset.loadCalls = String(Number(video.dataset.loadCalls || 0) + 1); } },
      play: { value: async () => { paused = false; video.dispatchEvent(new Event("play")); video.dispatchEvent(new Event("playing")); } },
      pause: { value: () => { paused = true; video.dispatchEvent(new Event("pause")); } },
    });
    Object.assign(window, {
      setDecoder: (state: { time?: number; ready?: number; start?: number; count?: number }) => {
        time = state.time ?? time; ready = state.ready ?? ready; start = state.start ?? start; count = state.count ?? count;
      },
    });
    if (mode === "mse") {
      class FakeHls {
        static isSupported = () => true;
        static Events = { MANIFEST_PARSED: "manifest", LEVEL_SWITCHED: "level", ERROR: "error" };
        static ErrorTypes = { NETWORK_ERROR: "network", MEDIA_ERROR: "media" };
        autoLevelEnabled = true; levels = [];
        on() {} loadSource() {} attachMedia() {} destroy() {}
      }
      Object.assign(window, { Hls: FakeHls });
    }
  }, scenario.mode);
  await page.addScriptTag({ content: playerSource });
  const video = page.locator("video"), status = page.locator("[data-player-status]");
  if (scenario.mode === "native") await expect(video).toHaveAttribute("data-load-calls", "1");
  await video.dispatchEvent("canplay");
  await video.evaluate(async (media, target) => {
    await media.play();
    media.dispatchEvent(new CustomEvent("kinosail:playback-intent", { detail: { playing: false } }));
    media.pause();
    (window as Window & {setDecoder: (state: {ready: number}) => void}).setDecoder({ready: 0});
    media.currentTime = target;
    media.dispatchEvent(new Event("seeking"));
  }, target);
  if (scenario.mode === "native") await expect(video).toHaveAttribute("data-load-calls", "2");
  const source = await video.evaluate((media) => media.src);
  await video.evaluate((media, state) => {
    (window as Window & {setDecoder: (state: {time: number; start: number; count: number; ready: number}) => void}).setDecoder({ ...state, ready: 4 });
    media.dispatchEvent(new Event("loadstart"));
    media.dispatchEvent(new Event("loadedmetadata"));
  }, { time: scenario.position, start: scenario.start, count: scenario.count });
  if ("target" in scenario) {
    // A real gap must remain pending until native readiness moves into the range.
    await video.evaluate((media) => {
      (window as Window & {setDecoder: (state: {time: number}) => void}).setDecoder({time: 0.083 - 0.000001});
      media.dispatchEvent(new Event("progress"));
    });
    await expect(status).toBeVisible();
    await video.evaluate(() => (window as Window & {setDecoder: (state: {time: number}) => void}).setDecoder({time: 0}));
  }
  await video.dispatchEvent("canplay");

  const nativeReady = scenario.mode === "native" && scenario.count > 0;
  if (nativeReady) await expect(status).toBeHidden();
  else await expect(status).toBeVisible();
  await expect(video).toHaveJSProperty("paused", true);
  await expect.poll(() => video.evaluate((media) => media.currentTime)).toBeCloseTo(
    scenario.mode === "native" ? target + Math.max(scenario.position, scenario.count ? scenario.start : 0) : 0,
    6,
  );
  await expect.poll(() => video.evaluate((media) => media.src)).toBe(source);
  if (scenario.mode === "native") await expect(video).toHaveAttribute("data-load-calls", "2");
  if (nativeReady) {
    await video.evaluate((media) => media.play());
    await expect(video).toHaveJSProperty("paused", false);
    await expect(status).toBeHidden();
  }
  await page.screenshot({ path: test.info().outputPath("paused-seek-readiness.png") });
});
