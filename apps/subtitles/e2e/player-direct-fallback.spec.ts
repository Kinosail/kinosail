import { playerSource } from "./static-sources";
import { expect, Page, test } from "@playwright/test";

async function startDirectPlayer(page: Page, options: { preloadHls?: boolean; directType?: string; directSupported?: boolean; safari?: boolean; compatibleMode?: string; compatibleLabel?: string; compatibleReason?: string; playbackPolicy?: string; playbackOverride?: boolean; initialSource?: boolean; savedPolicy?: string } = {}) {
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
      destroy() {}
    }
    Object.assign(window, { ...(preload ? { Hls: FakeHls } : {}), FakeHls });
  }, { preload: preloadHls, directSupported, safari });
  await page.addScriptTag({ content: playerSource });
}

async function failDirect(page: Page, code: number) {
  await page.locator("video").evaluate((media, errorCode) => {
    Object.defineProperty(media, "error", { configurable: true, value: { code: errorCode } });
    media.dispatchEvent(new Event("error"));
  }, code);
}

test("Direct First keeps direct play through buffering episodes", async ({ page }) => {
  await startDirectPlayer(page);

  await expect.poll(() => page.evaluate(() => (window as Window & { FakeHls: { instances: number } }).FakeHls.instances)).toBe(0);
  await page.locator("video").dispatchEvent("playing");
  await page.locator("video").dispatchEvent("seeking");
  await page.locator("video").dispatchEvent("waiting");
  await expect.poll(() => page.evaluate(() => (window as Window & { FakeHls: { instances: number } }).FakeHls.instances)).toBe(0);
  await page.locator("video").dispatchEvent("playing");
  await page.locator("video").dispatchEvent("waiting");
  await expect.poll(() => page.evaluate(() => (window as Window & { FakeHls: { instances: number } }).FakeHls.instances)).toBe(0);

  await page.locator("video").dispatchEvent("playing");
  await page.locator("video").dispatchEvent("waiting");
  await expect.poll(() => page.evaluate(() => (window as Window & { FakeHls: { instances: number } }).FakeHls.instances)).toBe(0);
  await expect(page.locator("[data-playback-mode-status]")).toHaveText("Direct Play");
  await expect(page.locator("[data-playback-reason]")).toContainText("No conversion");
  await expect(page.locator("video")).toHaveJSProperty("currentTime", 42);
});

test("Automatic does not transcode only because direct startup is slow", async ({ page }) => {
  await startDirectPlayer(page);
  await page.locator("video").dispatchEvent("play");
  await expect.poll(() => page.evaluate(() => (window as Window & { FakeHls: { instances: number } }).FakeHls.instances)).toBe(0);
  await page.waitForTimeout(100);
  await expect(page.getByText("Starting Direct Play", { exact: true }).first()).toBeVisible();
});

test("Automatic tries the original when its codec hint is unavailable", async ({ page }) => {
  await startDirectPlayer(page, { directType: "", directSupported: false });

  await expect.poll(() => page.evaluate(() => (window as Window & { FakeHls: { instances: number } }).FakeHls.instances)).toBe(0);
  await expect(page.locator("video")).toHaveAttribute("src", "/movie.mp4");
  await expect(page.locator("[data-playback-mode-status]")).toHaveText("Starting Direct Play");
});

test("Direct First starts minimal compatibility when Safari rejects the original container", async ({ page }) => {
  await startDirectPlayer(page, { directType: "video/x-matroska", directSupported: false, safari: true, compatibleMode: "audio-transcode", compatibleLabel: "Transcoding audio" });

  await expect.poll(() => page.evaluate(() => (window as Window & { FakeHls: { instances: number } }).FakeHls.instances)).toBe(1);
  await expect(page.locator("[data-playback-mode-status]")).toHaveText("Starting Transcoding audio");
  await expect(page.locator('[data-playback-mode] input[value="direct-first"]')).toBeChecked();
  await page.locator("video").dispatchEvent("canplay");
  await expect(page.locator("[data-playback-mode-status]")).toHaveText("Transcoding audio");
});

test("Direct First does not trust Safari claiming Matroska support", async ({ page }) => {
  await startDirectPlayer(page, { directType: "video/x-matroska", directSupported: true, safari: true, compatibleMode: "audio-transcode", compatibleLabel: "Transcoding audio" });

  await expect.poll(() => page.evaluate(() => (window as Window & { FakeHls: { instances: number } }).FakeHls.instances)).toBe(1);
  await expect(page.locator("[data-playback-mode-status]")).toHaveText("Starting Transcoding audio");
});

test("legacy Direct Play preference does not start unsupported media before Direct First", async ({ page }) => {
  let directRequests = 0;
  await page.route("https://direct.test/movie.mp4", (route) => {
    directRequests++;
    return route.fulfill({ status: 206, contentType: "video/x-matroska", headers: { "Content-Range": "bytes 0-0/1" }, body: "x" });
  });
  await startDirectPlayer(page, { directType: "video/x-matroska", directSupported: false, safari: true, compatibleMode: "audio-transcode", compatibleLabel: "Transcoding audio", initialSource: false, savedPolicy: "direct-only" });

  await expect.poll(() => page.evaluate(() => (window as Window & { FakeHls: { instances: number } }).FakeHls.instances)).toBe(1);
  await expect(page.locator('[data-playback-mode] input[value="direct-first"]')).toBeChecked();
  expect(directRequests).toBe(0);
});

test("Original quality applies only to the current playback", async ({ page }) => {
  await startDirectPlayer(page);
  await page.locator("[data-quality]").evaluate((select) => {
    select.add(new Option("Original", "original"));
    select.value = "original";
    select.dispatchEvent(new Event("change"));
  });

  await expect(page.locator('[data-playback-mode] input[value="direct-only"]')).toBeChecked();
  await expect.poll(() => page.evaluate(() => localStorage.getItem("kinosail.playback-policy-v2"))).toBeNull();
});

test("Direct First still asks before video conversion when Safari rejects the original container", async ({ page }) => {
  await startDirectPlayer(page, { directType: "video/x-matroska", directSupported: false, safari: true, compatibleMode: "transcode", compatibleLabel: "Transcoding video", compatibleReason: "This device cannot decode the original video." });

  await expect.poll(() => page.evaluate(() => (window as Window & { FakeHls: { instances: number } }).FakeHls.instances)).toBe(0);
  await expect(page.locator("[data-playback-recovery]")).toBeVisible();
  await expect(page.locator("[data-player-status] [data-player-fallback]")).toHaveText("Start video transcode");
});

test("Direct Play only still tries a container that Safari rejects", async ({ page }) => {
  await startDirectPlayer(page, { directType: "video/x-matroska", directSupported: false, playbackPolicy: "direct" });

  await expect.poll(() => page.evaluate(() => (window as Window & { FakeHls: { instances: number } }).FakeHls.instances)).toBe(0);
  await expect(page.locator("video")).toHaveAttribute("src", "/movie.mp4");
  await expect(page.locator("[data-playback-mode-status]")).toHaveText("Starting Direct Play");
});

test("Direct First remuxes only after a direct decode error", async ({ page }) => {
  let loads = 0;
  await page.route("https://direct.test/static/hls.min.js?v=1.7.1", (route) => {
    loads++;
    return route.fulfill({ contentType: "text/javascript", body: "window.Hls = window.FakeHls;" });
  });
  await startDirectPlayer(page, { preloadHls: false });

  expect(loads).toBe(0);
  await failDirect(page, 3);
  await expect.poll(() => loads).toBe(1);
  await expect.poll(() => page.evaluate(() => (window as Window & { FakeHls: { instances: number } }).FakeHls.instances)).toBe(1);
  await expect(page.locator("[data-playback-mode-status]")).toHaveText("Starting Remux");
  await expect(page.locator('[data-playback-mode] input[value="direct-first"]')).toBeChecked();
});

test("Direct First asks before video transcoding", async ({ page }) => {
  await startDirectPlayer(page, { compatibleMode: "transcode", compatibleLabel: "Transcoding video", compatibleReason: "This device cannot decode the original video." });

  await failDirect(page, 4);
  await expect.poll(() => page.evaluate(() => (window as Window & { FakeHls: { instances: number } }).FakeHls.instances)).toBe(0);
  await expect(page.locator("[data-playback-recovery]")).toBeVisible();
  await expect(page.locator("[data-player-status] [data-player-fallback]")).toHaveText("Start video transcode");
  await page.locator("[data-player-status] [data-player-fallback]").click();
  await expect.poll(() => page.evaluate(() => (window as Window & { FakeHls: { instances: number } }).FakeHls.instances)).toBe(1);
});

test("network errors never authorize transcoding", async ({ page }) => {
  await startDirectPlayer(page, { compatibleMode: "transcode", compatibleLabel: "Transcoding video" });
  await failDirect(page, 2);

  await expect.poll(() => page.evaluate(() => (window as Window & { FakeHls: { instances: number } }).FakeHls.instances)).toBe(0);
  await expect(page.locator("[data-player-message]")).toHaveText("The connection interrupted playback. Check your connection and try again.");
  await expect(page.locator("[data-player-status]")).toHaveClass(/is-recovery/);
  await expect(page.locator("[data-player-status] [data-player-fallback]")).toHaveText("Retry Direct Play");
  await page.locator("[data-player-status] [data-player-fallback]").click();
  await expect.poll(() => page.evaluate(() => (window as Window & { directLoads?: number }).directLoads)).toBe(1);
  await expect.poll(() => page.evaluate(() => (window as Window & { FakeHls: { instances: number } }).FakeHls.instances)).toBe(0);
});

test("a viewer can persist Direct Play only or choose compatibility", async ({ page }) => {
  await startDirectPlayer(page);

  await page.locator('[data-playback-mode] input[value="compatible"]').check();
  await expect.poll(() => page.evaluate(() => (window as Window & { FakeHls: { instances: number } }).FakeHls.instances)).toBe(1);
  await expect(page.locator("[data-playback-mode-status]")).toHaveText("Starting Remux");
  await expect.poll(() => page.evaluate(() => localStorage.getItem("kinosail.playback-policy-v2"))).toBe("compatible");
  await page.locator('[data-playback-mode] input[value="direct-only"]').check();
  await expect(page.locator("[data-playback-mode-status]")).toHaveText("Starting Direct Play");
  await expect.poll(() => page.evaluate(() => localStorage.getItem("kinosail.playback-policy-v2"))).toBe("direct-only");
  await failDirect(page, 3);
  await expect.poll(() => page.evaluate(() => (window as Window & { FakeHls: { instances: number } }).FakeHls.instances)).toBe(1);
});

test("an explicit direct link overrides the saved browser policy", async ({ page }) => {
  await page.addInitScript(() => localStorage.setItem("kinosail.playback-policy-v2", "compatible"));
  await startDirectPlayer(page, { playbackPolicy: "direct", playbackOverride: true });

  await expect(page.locator('[data-playback-mode] input[value="direct-only"]')).toBeChecked();
  await expect.poll(() => page.evaluate(() => (window as Window & { FakeHls: { instances: number } }).FakeHls.instances)).toBe(0);
  await expect.poll(() => page.evaluate(() => localStorage.getItem("kinosail.playback-policy-v2"))).toBe("direct-only");
});
