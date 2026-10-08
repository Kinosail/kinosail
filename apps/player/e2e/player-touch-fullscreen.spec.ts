import {readFile, writeFile} from "node:fs/promises";
import {expect, test} from "@playwright/test";
import {installPlayerExperienceFixture} from "./player-experience-fixture";

test.describe("touch fullscreen @smoke", () => {
test.use({hasTouch: true, ignoreHTTPSErrors: false});
installPlayerExperienceFixture(false, false, "iPhone", async (page, title) => {
  // Firefox touch emulation does not populate maxTouchPoints. Supply the
  // declared hardware capability before the Player selects its control policy.
  await page.evaluate(({limited, touch}) => {
    Object.defineProperty(navigator, "maxTouchPoints", {configurable: true, value: touch ? 1 : 0});
    if (limited) Object.defineProperty(document.querySelector("video"), "webkitEnterFullscreen", {configurable: true, value: () => {}});
  }, {limited: title.includes("limited native fullscreen"), touch: !title.includes("keeps the container")});
});

test.afterEach(async ({browserName}, testInfo) => {
  const receipt = testInfo.outputPath("fullscreen-receipt.json");
  await writeFile(receipt, JSON.stringify({revision: process.env.GITHUB_SHA, command: "pnpm exec playwright test player-touch-fullscreen.spec.ts", browser: browserName, test: testInfo.title, result: testInfo.status, data: "Isolated Player fixture with simulated media state and labeled test poster", deviceBoundary: "Chromium touch emulation; native Safari fullscreen calls are simulated, physical iPhone UI is not verified"}, null, 2));
  await testInfo.attach("fullscreen-receipt", {path: receipt, contentType: "application/json"});
});

test("limited native fullscreen fallback uses touch Play and preserves fullscreen on resume", async ({page}) => {
  await page.locator("video").evaluate(video => {
    let fullscreen = false;
    Object.assign(window, {fullscreenGestures: []});
    Object.defineProperties(video, {
      webkitDisplayingFullscreen: {get: () => fullscreen},
      webkitEnterFullscreen: {configurable: true, value: () => {
        (window as Window & {fullscreenGestures: boolean[]}).fullscreenGestures.push(navigator.userActivation.isActive);
        fullscreen = true;
        video.dispatchEvent(new Event("webkitbeginfullscreen"));
      }},
      webkitExitFullscreen: {value: () => {
        fullscreen = false;
        video.dispatchEvent(new Event("webkitendfullscreen"));
      }},
    });
  });
  const video = page.locator("video");
  await page.evaluate(() => (window as Window & {setPlayPending: (value: boolean) => void}).setPlayPending(true));
  await page.getByRole("button", {name: "Play", exact: true}).first().tap();
  await expect(video).toHaveJSProperty("webkitDisplayingFullscreen", true);
  await expect(video).toHaveJSProperty("paused", false);
  await page.evaluate(() => {
    const state = window as Window & {finishPlay: () => void; setPlayPending: (value: boolean) => void};
    state.finishPlay();
    state.setPlayPending(false);
  });
  await page.getByRole("button", {name: "Pause", exact: true}).first().tap();
  await expect(video).toHaveJSProperty("paused", true);
  await page.getByRole("button", {name: "Play", exact: true}).first().tap();
  await expect(video).toHaveJSProperty("webkitDisplayingFullscreen", true);
  expect(await page.evaluate(() => (window as Window & {fullscreenGestures: boolean[]}).fullscreenGestures)).toEqual([true]);
  await page.getByRole("button", {name: "Exit fullscreen"}).tap();
  await expect(video).toHaveJSProperty("webkitDisplayingFullscreen", false);
  await expect(video).toHaveJSProperty("paused", false);
  await expect(page.locator("[data-subtitles]")).toHaveValue("off");
});

test("limited native fullscreen fallback rejection after touch Play retains inline playback", async ({page}) => {
  await page.locator("video").evaluate(video => Object.defineProperty(video, "webkitEnterFullscreen", {
    configurable: true, value: () => { throw new DOMException("denied", "NotAllowedError"); },
  }));
  await page.getByRole("button", {name: "Play", exact: true}).first().tap();
  await expect(page.getByRole("status").filter({hasText: /Fullscreen could not open/})).toBeVisible();
  await expect(page.locator("video")).toHaveJSProperty("paused", false);
  await expect(page.locator("video")).toHaveJSProperty("currentTime", 20);
  await expect(page.locator("[data-subtitles]")).toHaveValue("off");
  await page.getByRole("button", {name: "Pause", exact: true}).first().tap();
  await expect(page.locator("video")).toHaveJSProperty("paused", true);
});

test("touch custom playback waits for Play despite resumed autoplay", async ({page}) => {
  await page.locator("video").dispatchEvent("canplay");
  await expect(page.locator("video")).toHaveJSProperty("paused", true);
  await page.getByRole("button", {name: "Play", exact: true}).first().tap();
  await expect(page.locator("video")).toHaveJSProperty("paused", false);
});

test("limited native fullscreen preserves limited in-band choices when both fullscreen APIs are available", async ({page}) => {
  await page.locator("video").evaluate(video => {
    let fullscreen = false;
    Object.assign(window, {nativeCalls: [], containerCalls: 0});
    Object.defineProperties(video, {
      webkitDisplayingFullscreen: {get: () => fullscreen},
      webkitEnterFullscreen: {configurable: true, value: () => {
        (window as Window & {nativeCalls: boolean[]}).nativeCalls.push(navigator.userActivation.isActive);
        fullscreen = true;
        video.dispatchEvent(new Event("webkitbeginfullscreen"));
      }},
      webkitExitFullscreen: {value: () => { fullscreen = false; video.dispatchEvent(new Event("webkitendfullscreen")); }},
    });
    Object.defineProperty(video.closest(".media-stage"), "requestFullscreen", {value: async () => {
      (window as Window & {containerCalls: number}).containerCalls++;
    }});
    video.playsInline = true;
  });
  const video = page.locator("video");
  await page.getByRole("button", {name: "Settings", exact: true}).tap();
  await page.locator("[data-subtitles]").selectOption("0");
  await page.locator("[data-player-settings-close]").tap();
  await expect(page.getByRole("button", {name: "Enter fullscreen"})).toHaveAttribute("title", /native player and subtitle menu/);
  await page.getByRole("button", {name: "Play", exact: true}).first().tap();
  await expect(video).toHaveJSProperty("webkitDisplayingFullscreen", true);
  await expect(video).toHaveJSProperty("playsInline", true);
  await page.getByRole("button", {name: "Pause", exact: true}).first().tap();
  await page.getByRole("button", {name: "Play", exact: true}).first().tap();
  expect(await page.evaluate(() => (window as Window & {nativeCalls: boolean[]}).nativeCalls)).toEqual([true]);
  await page.getByRole("button", {name: "Exit fullscreen"}).tap();
  await expect(video).toHaveJSProperty("webkitDisplayingFullscreen", false);
  await expect(video).toHaveJSProperty("paused", false);
  await page.getByRole("button", {name: "Pause", exact: true}).first().tap();
  await page.getByRole("button", {name: "Play", exact: true}).first().tap();
  expect(await page.evaluate(() => (window as Window & {nativeCalls: boolean[]}).nativeCalls)).toEqual([true, true]);
  expect(await page.evaluate(() => (window as Window & {containerCalls: number}).containerCalls)).toBe(0);
  await expect(video).toHaveAttribute("data-subtitle-picker-limited", "true");
  await expect(page.locator("[data-subtitles]")).toHaveValue("0");
  expect(await video.evaluate(media => media.querySelector("track")!.track.mode)).toBe("showing");
  expect(await video.evaluate(media => media.textTracks[0].mode)).toBe("disabled");
});

test.describe("non-touch custom controls", () => {
  test.use({hasTouch: false});
  test("limited native fullscreen keeps the container when both fullscreen APIs are available", async ({page, browserName}) => {
    test.skip(browserName !== "chromium", "The real container fullscreen assertion uses Chromium.");
    await page.getByRole("button", {name: "Play", exact: true}).first().click();
    await page.getByRole("button", {name: "Enter fullscreen"}).click();
    await expect.poll(() => page.evaluate(() => document.fullscreenElement?.classList.contains("media-stage"))).toBe(true);
    expect(await page.evaluate(() => (window as Window & {nativeFullscreenCalls?: number}).nativeFullscreenCalls || 0)).toBe(0);
    await expect(page.locator("video")).toHaveJSProperty("paused", false);
    await page.getByRole("button", {name: "Exit fullscreen"}).click();
    await expect.poll(() => page.evaluate(() => document.fullscreenElement)).toBeNull();
  });
});

test("limited native fullscreen readiness failure with both fullscreen APIs waits for a fresh gesture", async ({page}) => {
  await page.locator("video").evaluate(video => {
    Object.assign(window, {nativeCalls: [], containerCalls: 0});
    Object.defineProperty(video, "webkitEnterFullscreen", {configurable: true, value: () => {
      (window as Window & {nativeCalls: boolean[]}).nativeCalls.push(navigator.userActivation.isActive);
      if (!video.readyState) throw new DOMException("private media detail", "InvalidStateError");
      Object.defineProperty(video, "webkitDisplayingFullscreen", {value: true});
      video.dispatchEvent(new Event("webkitbeginfullscreen"));
    }});
    Object.defineProperty(video.closest(".media-stage"), "requestFullscreen", {value: async () => {
      (window as Window & {containerCalls: number}).containerCalls++;
    }});
  });
  await page.evaluate(() => (window as Window & {setReadyState: (value: number) => void}).setReadyState(0));
  await page.getByRole("button", {name: "Play", exact: true}).first().tap();
  await expect(page.getByRole("status").filter({hasText: /Fullscreen could not open/})).toBeVisible();
  await expect(page.locator("video")).toHaveJSProperty("paused", false);
  await expect(page.locator(".player-control-feedback")).not.toContainText("private media detail");
  await page.evaluate(() => (window as Window & {setReadyState: (value: number) => void}).setReadyState(4));
  await page.locator("video").dispatchEvent("loadedmetadata");
  await page.locator("video").dispatchEvent("canplay");
  expect(await page.evaluate(() => (window as Window & {nativeCalls: boolean[]}).nativeCalls)).toEqual([true]);
  await page.getByRole("button", {name: "Enter fullscreen"}).tap();
  await expect(page.locator("video")).toHaveJSProperty("webkitDisplayingFullscreen", true);
  expect(await page.evaluate(() => (window as Window & {nativeCalls: boolean[]}).nativeCalls)).toEqual([true, true]);
  expect(await page.evaluate(() => (window as Window & {containerCalls: number}).containerCalls)).toBe(0);
});

for (const viewport of [{width: 390, height: 844}, {width: 844, height: 390}, {width: 1440, height: 900}, {width: 1920, height: 1080}]) {
  test(`touch fullscreen fills the viewport and keeps stable media states at ${viewport.width}x${viewport.height}`, async ({page, browserName}, testInfo) => {
    test.skip(browserName !== "chromium", "Real element fullscreen is verified in Chromium; iPhone native UI needs a device.");
    await page.setViewportSize(viewport);
    for (const path of ["../../../packages/webassets/static/last-light.css", "../../../packages/webassets/static/player-app.css", "../internal/server/static/home.css", "../../../packages/webassets/static/player-stage.css"]) {
      await page.addStyleTag({content: await readFile(path, "utf8")});
    }
    const video = page.locator("video");
    await video.evaluate(media => {
      media.poster = `data:image/svg+xml,${encodeURIComponent('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1920 1080"><rect width="1920" height="1080" fill="#142d3d"/><circle cx="960" cy="510" r="260" fill="#30728e"/><text x="960" y="540" text-anchor="middle" fill="white" font-family="sans-serif" font-size="80">FULLSCREEN TEST FRAME</text></svg>')}`;
    });
    await page.getByRole("button", {name: "Play", exact: true}).first().tap();
    await expect.poll(() => page.evaluate(() => document.fullscreenElement?.classList.contains("media-stage"))).toBe(true);
    const stage = page.locator(".media-stage");
    const bounds = await stage.boundingBox();
    expect(bounds).toEqual({x: 0, y: 0, ...viewport});
    expect(await video.boundingBox()).toEqual(bounds);
    await expect(video).toHaveCSS("object-fit", "contain");
    await page.screenshot({path: testInfo.outputPath("fullscreen-loaded.png")});
    await page.getByRole("button", {name: "Settings", exact: true}).tap();
    const settings = page.locator(".player-settings");
    const settingsBounds = await settings.boundingBox();
    expect(settingsBounds).not.toBeNull();
    expect(settingsBounds!.x).toBeGreaterThanOrEqual(0);
    expect(settingsBounds!.y).toBeGreaterThanOrEqual(0);
    expect(settingsBounds!.x + settingsBounds!.width).toBeLessThanOrEqual(viewport.width);
    expect(settingsBounds!.y + settingsBounds!.height).toBeLessThanOrEqual(viewport.height);
    await page.locator("[data-subtitles]").selectOption("1");
    await expect(page.locator("[data-subtitles]")).toHaveValue("1");
    await page.screenshot({path: testInfo.outputPath("fullscreen-settings.png")});
    await page.locator("[data-player-settings-close]").tap();
    await expect(settings).toBeHidden();
    await expect(page.getByRole("button", {name: "Settings", exact: true})).toBeFocused();
    expect(await stage.boundingBox()).toEqual(bounds);
    await page.evaluate(() => (window as Window & {setReadyState: (value: number) => void}).setReadyState(1));
    await video.dispatchEvent("waiting");
    await expect(page.locator("[data-player-status]")).toBeVisible();
    expect(await stage.boundingBox()).toEqual(bounds);
    await page.screenshot({path: testInfo.outputPath("fullscreen-pending.png")});
    await page.evaluate(() => (window as Window & {setReadyState: (value: number) => void}).setReadyState(4));
    await video.dispatchEvent("playing");
    await expect(page.locator("[data-player-status]")).toBeHidden();
    await video.evaluate(media => Object.defineProperty(media, "error", {value: {code: 0}}));
    await video.dispatchEvent("error");
    await expect(page.locator("[data-player-status]")).toContainText("Direct Play only is selected");
    expect(await stage.boundingBox()).toEqual(bounds);
    await page.screenshot({path: testInfo.outputPath("fullscreen-failed.png")});
    await page.getByRole("button", {name: "Exit fullscreen"}).tap();
    await expect.poll(() => page.evaluate(() => document.fullscreenElement)).toBeNull();
  });
}

});
