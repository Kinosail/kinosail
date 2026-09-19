import { Page } from "@playwright/test";
import { playerSource } from "./static-sources";

export async function startDirectPlayer(page: Page, options: { preloadHls?: boolean; directType?: string; directSupported?: boolean; safari?: boolean; compatibleMode?: string; compatibleLabel?: string; compatibleReason?: string; playbackPolicy?: string; playbackOverride?: boolean; initialSource?: boolean; savedPolicy?: string } = {}) {
  const { preloadHls = true, directType = "video/mp4", directSupported = true, safari = false, compatibleMode = "remux", compatibleLabel = "Remux", compatibleReason = "Repackages the original video and audio without conversion.", playbackPolicy = "automatic", playbackOverride = false, initialSource = true, savedPolicy = "" } = options;
  if (savedPolicy) await page.addInitScript((policy) => localStorage.setItem("kinosail.playback-policy", policy), savedPolicy);
  await page.route("https://direct.test/", (route) => route.fulfill({ contentType: "text/html", body: `
    <body><div class="media-stage">
      <video ${initialSource ? 'src="/movie.mp4" ' : ""}data-direct="/movie.mp4" data-direct-type="${directType}" data-adaptive="/movie.m3u8" data-fallback="?compatible=1" data-playback-policy="${playbackPolicy}" data-playback-override="${playbackOverride}" data-playback-label="Direct Play" data-playback-description="Original video and audio. No conversion." data-compatibility-mode="${compatibleMode}" data-compatibility-label="${compatibleLabel}" data-compatibility-description="${compatibleReason}" data-progress="/progress/movie"></video>
      <button type="button" data-playback-mode-status>Starting Direct Play</button>
      <fieldset data-playback-mode><label><input type="radio" name="playback-policy" value="direct-first" checked>Direct First</label><label><input type="radio" name="playback-policy" value="direct-only">Direct Play only</label><label><input type="radio" name="playback-policy" value="compatible">Compatibility</label></fieldset>
      <small data-playback-reason></small>
      <div data-quality-control hidden><select data-quality></select><span data-quality-state></span></div>
      <div data-playback-recovery hidden><span data-playback-recovery-message></span><button type="button" data-player-fallback></button></div>
      <div data-player-status hidden><span data-player-message></span><button type="button" data-player-fallback hidden></button></div>
      <span data-playback-method-detail></span>
    </div></body>
  ` }));
  await page.route("https://direct.test/movie.mp4", (route) => route.fulfill({ status: 206, contentType: "video/mp4", headers: { "Content-Range": "bytes 0-0/1" }, body: "x" }));
  await page.goto("https://direct.test/");
  await page.evaluate(({ preload, directSupported, safari }) => {
    if (safari) Object.defineProperty(navigator, "vendor", { configurable: true, value: "Apple Computer, Inc." });
    HTMLMediaElement.prototype.canPlayType = (type) => type === "application/vnd.apple.mpegurl" ? "" : directSupported ? "probably" : "";
    const media = document.querySelector("video")!;
    media.addEventListener("error", (event) => { if (event.isTrusted) event.stopImmediatePropagation(); }, true);
    Object.defineProperties(media, {
      currentTime: { value: 42, writable: true },
      duration: { value: 120 },
      paused: { value: false },
      load: { value() { const state = window as Window & { directLoads?: number }; state.directLoads = (state.directLoads || 0) + 1; } },
      play: { value: async () => {} },
    });
    class FakeHls {
      static Events = { MANIFEST_PARSED: "manifest", LEVEL_SWITCHED: "switched", ERROR: "error" };
      static ErrorTypes = { NETWORK_ERROR: "network", MEDIA_ERROR: "media" };
      static isSupported = () => true;
      static instances = 0;
      handlers = new Map<string, (...args: (string | object)[]) => void>();
      autoLevelEnabled = true;
      levels = [];
      constructor() { FakeHls.instances++; Object.assign(window, { hlsInstance: this }); }
      on(event: string, handler: (...args: (string | object)[]) => void) { this.handlers.set(event, handler); }
      loadSource(source: string) { Object.assign(window, { loadedSource: source }); }
      attachMedia(media: HTMLMediaElement) {
        media.currentTime = 0;
        media.dispatchEvent(new Event("loadedmetadata"));
      }
      startLoad() { const state = window as Window & { hlsReloads?: number }; state.hlsReloads = (state.hlsReloads || 0) + 1; }
      destroy() {}
    }
    Object.assign(window, { ...(preload ? { Hls: FakeHls } : {}), FakeHls });
  }, { preload: preloadHls, directSupported, safari });
  await page.addScriptTag({ content: playerSource });
}

export async function failDirect(page: Page, code: number) {
  await page.locator("video").evaluate((media, errorCode) => {
    Object.defineProperty(media, "error", { configurable: true, value: { code: errorCode } });
    media.dispatchEvent(new Event("error"));
  }, code);
}
