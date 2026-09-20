import { expect, test } from "@playwright/test";
import { failDirect, startDirectPlayer } from "./player-direct-fallback-fixture";

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
  await expect(page.getByText("Direct Play", { exact: true }).first()).toBeVisible();
});

test("Automatic tries the original when its codec hint is unavailable", async ({ page }) => {
  await startDirectPlayer(page, { directType: "", directSupported: false });

  await expect.poll(() => page.evaluate(() => (window as Window & { FakeHls: { instances: number } }).FakeHls.instances)).toBe(0);
  await expect(page.locator("video")).toHaveAttribute("src", "/movie.mp4");
  await expect(page.locator("[data-playback-mode-status]")).toHaveText("Direct Play");
});

test("Direct First starts minimal compatibility when Safari rejects the original container", async ({ page }) => {
  await startDirectPlayer(page, { directType: "video/x-matroska", directSupported: false, safari: true, compatibleMode: "audio-transcode", compatibleLabel: "Transcoding audio" });

  await expect.poll(() => page.evaluate(() => (window as Window & { FakeHls: { instances: number } }).FakeHls.instances)).toBe(1);
  await expect(page.locator("[data-playback-mode-status]")).toHaveText("Transcoding audio");
  await expect(page.locator('[data-playback-mode] input[value="direct-first"]')).toBeChecked();
  await page.locator("video").dispatchEvent("canplay");
  await expect(page.locator("[data-playback-mode-status]")).toHaveText("Transcoding audio");
});

test("Direct First does not trust Safari claiming Matroska support", async ({ page }) => {
  await startDirectPlayer(page, { directType: "video/x-matroska", directSupported: true, safari: true, compatibleMode: "audio-transcode", compatibleLabel: "Transcoding audio" });

  await expect.poll(() => page.evaluate(() => (window as Window & { FakeHls: { instances: number } }).FakeHls.instances)).toBe(1);
  await expect(page.locator("[data-playback-mode-status]")).toHaveText("Transcoding audio");
});

test("Direct First starts known audio compatibility before silent direct playback", async ({ page }) => {
  await startDirectPlayer(page, { compatibleMode: "audio-transcode", compatibleLabel: "Transcoding audio" });

  await expect.poll(() => page.evaluate(() => (window as Window & { FakeHls: { instances: number } }).FakeHls.instances)).toBe(1);
  await expect(page.locator("[data-playback-mode-status]")).toHaveText("Transcoding audio");
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
  await expect(page.locator("[data-playback-mode-status]")).toHaveText("Direct Play");
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
  await expect(page.locator("[data-playback-mode-status]")).toHaveText("Remux");
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

test("network errors retry the original source with retained position and a bounded budget", async ({ page }) => {
  await startDirectPlayer(page, { compatibleMode: "transcode", compatibleLabel: "Transcoding video" });
  await page.clock.install();
  for (let attempt = 0; attempt < 3; attempt++) {
    await failDirect(page, 2);
    await page.clock.fastForward(5000);
    await expect.poll(() => page.evaluate(() => (window as Window & { directLoads?: number }).directLoads)).toBe(attempt + 1);
    await page.locator("video").dispatchEvent("loadedmetadata");
    await expect(page.locator("video")).toHaveJSProperty("currentTime", 42);
  }
  await failDirect(page, 2);
  await page.clock.fastForward(5000);
  await expect(page.locator("[data-player-message]")).toHaveText("Connection interrupted. Your position is retained.");
  expect(await page.evaluate(() => (window as Window & { directLoads?: number }).directLoads)).toBe(3);
  expect(await page.evaluate(() => (window as Window & { FakeHls: { instances: number } }).FakeHls.instances)).toBe(0);
});

test("pause cancels a pending network reopen and seek updates the retained position", async ({ page }) => {
  await startDirectPlayer(page, { playbackPolicy: "direct-only" });
  await page.clock.install();
  await failDirect(page, 2);
  await page.locator("video").evaluate(media => media.dispatchEvent(new CustomEvent("kinosail:playback-intent", {detail: {playing: false}})));
  await page.clock.fastForward(5000);
  expect(await page.evaluate(() => (window as Window & { directLoads?: number }).directLoads || 0)).toBe(0);
  await page.locator("video").evaluate(media => {
    media.dispatchEvent(new CustomEvent("kinosail:playback-intent", {detail: {playing: true}}));
    (media as HTMLVideoElement).currentTime = 73;
    media.dispatchEvent(new Event("seeking"));
  });
  await failDirect(page, 2);
  await page.clock.fastForward(5000);
  await page.locator("video").dispatchEvent("loadedmetadata");
  await expect(page.locator("video")).toHaveJSProperty("currentTime", 73);
  expect(await page.evaluate(() => (window as Window & { FakeHls: { instances: number } }).FakeHls.instances)).toBe(0);
});

test("a viewer can persist Direct Play only or choose compatibility", async ({ page }) => {
  await startDirectPlayer(page);

  await page.locator('[data-playback-mode] input[value="compatible"]').check();
  await expect.poll(() => page.evaluate(() => (window as Window & { FakeHls: { instances: number } }).FakeHls.instances)).toBe(1);
  await expect(page.locator("[data-playback-mode-status]")).toHaveText("Remux");
  await expect.poll(() => page.evaluate(() => localStorage.getItem("kinosail.playback-policy-v2"))).toBe("compatible");
  await page.locator('[data-playback-mode] input[value="direct-only"]').check();
  await expect(page.locator("[data-playback-mode-status]")).toHaveText("Direct Play");
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

test("compatibility loads HLS alongside bounded capability detection", async ({ page }) => {
  let adapterRequests = 0;
  await page.route("https://direct.test/static/hls.min.js?v=1.7.1", route => {
    adapterRequests++;
    return route.fulfill({ contentType: "text/javascript", body: "window.Hls = window.FakeHls;" });
  });
  await startDirectPlayer(page, { preloadHls: false, compatibleMode: "transcode" });
  await page.evaluate(() => {
    document.querySelector("video")!.dataset.playbackApi = "/api/v1/items/movie/playback";
    Object.defineProperty(navigator, "mediaCapabilities", { configurable: true, value: { decodingInfo: () => new Promise(() => {}) } });
  });
  expect(adapterRequests).toBe(0);
  await page.getByLabel("Compatibility", { exact: true }).check();
  await expect.poll(() => adapterRequests, { timeout: 1000 }).toBe(1);
  await expect.poll(() => page.evaluate(() => (window as Window & { FakeHls: { instances: number } }).FakeHls.instances), { timeout: 3000 }).toBe(1);
});

for (const compatibleMode of ["remux", "audio-transcode"]) {
  test(`${compatibleMode} starts without waiting for video capability negotiation`, async ({ page }) => {
    let negotiations = 0;
    await page.route("https://direct.test/api/v1/items/movie/playback?*", route => {
      negotiations++;
      return route.abort();
    });
    // Audio incompatibility is selected before direct startup; keep this case on the
    // original source until the explicit Compatibility action below.
    await startDirectPlayer(page, { compatibleMode: compatibleMode === "audio-transcode" ? "remux" : compatibleMode });
    await page.evaluate((mode) => {
      document.querySelector("video")!.dataset.playbackApi = "/api/v1/items/movie/playback";
      Object.assign(window, { capabilityChecks: 0 });
      Object.defineProperty(navigator, "mediaCapabilities", { configurable: true, value: { decodingInfo: () => {
        const state = window as Window & { capabilityChecks: number };
        state.capabilityChecks++;
        return new Promise(() => {});
      } } });
      if (mode === "audio-transcode") document.querySelector("video")!.dataset.compatibilityMode = mode;
    }, compatibleMode);
    await page.getByLabel("Compatibility", { exact: true }).check();
    await expect.poll(() => page.evaluate(() => (window as Window & { FakeHls: { instances: number } }).FakeHls.instances)).toBe(1);
    expect(await page.evaluate(() => (window as Window & { capabilityChecks: number }).capabilityChecks)).toBe(0);
    expect(negotiations).toBe(0);
    await expect(page.locator("video")).toHaveJSProperty("currentTime", 42);
  });
}

for (const compatible of ["https://other.test/hls/movie/index.m3u8", "/hls/../admin", "/hls/movie/index.m3u8#secret", "/hls/" + "x".repeat(2049)]) {
  test(`invalid negotiated source leaves the approved rendition unchanged: ${compatible.slice(0, 45)}`, async ({ page }) => {
    await startDirectPlayer(page, { compatibleMode: "transcode" });
    await page.route("https://direct.test/api/v1/items/movie/playback?*", route => route.fulfill({ json: { compatible, compatiblePlan: { allowed: true, mode: "remux" } } }));
    await page.evaluate(() => {
      document.querySelector("video")!.dataset.playbackApi = "/api/v1/items/movie/playback";
      Object.defineProperty(navigator, "mediaCapabilities", { configurable: true, value: { decodingInfo: async () => ({ supported: true, smooth: true, powerEfficient: true }) } });
    });
    await page.getByLabel("Compatibility", { exact: true }).check();
    await expect.poll(() => page.evaluate(() => (window as Window & { loadedSource: string }).loadedSource)).toMatch(/^\/movie\.m3u8/);
  });
}


test("HLS retry resumes after pause even when the media element has no native error", async ({page}) => {
  await startDirectPlayer(page, {playbackPolicy: "compatible"});
  await page.clock.install();
  await page.evaluate(() => {
    const media = document.querySelector("video")!;
    Object.defineProperty(media, "error", {configurable: true, value: null});
    const state = window as Window & {hlsInstance: {handlers: Map<string, Function>}};
    state.hlsInstance.handlers.get("error")!("error", {fatal: true, type: "network"});
    media.dispatchEvent(new CustomEvent("kinosail:playback-intent", {detail: {playing: false}}));
    media.dispatchEvent(new CustomEvent("kinosail:playback-intent", {detail: {playing: true}}));
  });
  expect(await page.evaluate(() => (window as Window & {hlsReloads?: number}).hlsReloads)).toBe(1);
  await page.clock.fastForward(5000);
  expect(await page.evaluate(() => (window as Window & {hlsReloads?: number}).hlsReloads)).toBe(1);
});
