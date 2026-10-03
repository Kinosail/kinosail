import {writeFile} from "node:fs/promises";
import {expect, test} from "@playwright/test";
import {startHappyPath} from "./happy-path-setup";

test("real Server media supports Apple launch, seek, pause, captions and repeat loads", async ({page}, info) => {
  test.skip(process.env.KINOSAIL_APPLE_LOCAL_E2E !== "1", "Requires the disposable local runner");
  test.setTimeout(120_000);
  await page.addInitScript(() => {
    const fullscreen = new WeakSet<HTMLVideoElement>();
    Object.defineProperty(navigator, "userAgent", {value: "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 Mobile/15E148"});
    Object.defineProperties(HTMLVideoElement.prototype, {
      webkitDisplayingFullscreen: {get() { return fullscreen.has(this); }},
      webkitEnterFullscreen: {value() { fullscreen.add(this); this.dispatchEvent(new Event("webkitbeginfullscreen")); }, configurable: true},
      webkitExitFullscreen: {value() { fullscreen.delete(this); this.dispatchEvent(new Event("webkitendfullscreen")); }},
    });
  });
  const state = await startHappyPath(page, info);
  const library = await page.request.get("/api/v1/library");
  expect(library.ok()).toBe(true);
  const data = await library.json();
  const item = data.items.find((candidate: {title: string}) => candidate.title === "Direct Retry Control");
  expect(item).toBeTruthy();
  const startupMs: number[] = [];
  for (let load = 0; load < 2; load++) {
    await page.setViewportSize({width: load ? 844 : 390, height: load ? 390 : 844});
    await page.goto(`/watch/${item.id}?playback=direct`);
    const video = page.locator("video");
    await expect(video).toHaveJSProperty("paused", true);
    await expect(video).toHaveJSProperty("playsInline", false);
    await expect(page.getByRole("button", {name: "Play", exact: true})).toBeVisible();
    await expect(page.locator(".player-control-dock")).toBeHidden();
    await expect.poll(() => video.evaluate(media => media.readyState)).toBeGreaterThan(0);
    await page.screenshot({path: info.outputPath(`launch-${load}-ready.png`)});
    await page.getByRole("button", {name: "Settings", exact: true}).click();
    await page.locator("[data-subtitles]").selectOption("0");
    await page.locator("[data-player-settings-close]").click();
    if (!load) {
      await video.evaluate(media => Object.defineProperty(media, "webkitEnterFullscreen", {value: () => { throw new DOMException("synthetic rejection", "NotAllowedError"); }, configurable: true}));
      await page.getByRole("button", {name: "Play", exact: true}).click();
      await expect(video).toHaveJSProperty("paused", true);
      await expect(page.locator(".player-control-feedback")).toBeVisible();
      await page.screenshot({path: info.outputPath("launch-failure.png")});
      await video.evaluate(media => { delete (media as HTMLVideoElement & {webkitEnterFullscreen?: () => void}).webkitEnterFullscreen; });
    }
    const started = Date.now();
    await page.getByRole("button", {name: "Play", exact: true}).click();
    await expect.poll(() => video.evaluate(media => media.currentTime)).toBeGreaterThan(0.5);
    startupMs.push(Date.now() - started);
    expect(startupMs.at(-1)).toBeLessThan(5_000);
    expect(await video.evaluate(media => media.error?.code || 0)).toBe(0);
    await video.evaluate(media => { media.pause(); media.currentTime = 7; });
    await expect(video).toHaveJSProperty("paused", true);
    await expect.poll(() => video.evaluate(media => media.currentTime)).toBeGreaterThanOrEqual(7);
    await video.evaluate(media => media.play());
    await expect.poll(() => video.evaluate(media => media.currentTime)).toBeGreaterThan(7.25);
    expect(await video.evaluate(media => [...media.textTracks].some(track => track.mode === "showing"))).toBe(true);
    await video.evaluate(media => (media as HTMLVideoElement & {webkitExitFullscreen: () => void}).webkitExitFullscreen());
    await expect(video).toHaveJSProperty("paused", true);
    await expect(page.getByRole("button", {name: "Play", exact: true})).toBeVisible();
    await page.getByRole("button", {name: "Play", exact: true}).click();
    await expect(video).toHaveJSProperty("paused", false);
  }
  const receipt = info.outputPath("real-server-apple-launch.json");
  expect(state.errors).toEqual([]);
  await writeFile(receipt, JSON.stringify({revision: process.env.KINOSAIL_TEST_REVISION,
    command: "python3 apps/player/scripts/test-apple-playback-local.py", result: "passed", startupMs,
    environment: "macOS ARM64 Chrome; native Go Server on loopback HTTP with disposable state",
    data: "Generated MP4/AAC and English sidecar; real HTTP, decode, seek, captions and persistence",
    boundary: "Apple fullscreen API is simulated; no physical iPhone, production media, TLS or deployment proof"}, null, 2));
  await info.attach("real-server-apple-launch", {path: receipt, contentType: "application/json"});
});
