import {readFile} from "node:fs/promises";
import {expect, test} from "@playwright/test";
import {installPlayerExperienceFixture} from "./player-experience-fixture";

installPlayerExperienceFixture(true, false, "iPhone", async (page, title) => {
  if (title === "rejected fullscreen leaves playback usable") {
    await page.evaluate(() => {
      Object.defineProperty(document.querySelector("video"), "webkitEnterFullscreen", {configurable: true, value: undefined});
      Object.defineProperty(document.querySelector("video"), "requestFullscreen", {configurable: true, value: () => Promise.reject(new DOMException("private-token https://private.invalid/movie", "NotAllowedError"))});
      Object.defineProperty(document, "fullscreenEnabled", {configurable: true, value: true});
    });
    return;
  }
  if (title !== "touch Play enters native fullscreen in the same gesture without pausing") return;
  await page.evaluate(() => {
    Object.defineProperty(navigator, "maxTouchPoints", {configurable: true, value: 1});
    Object.assign(window, {nativeFullscreenCalls: 0});
    Object.defineProperty(document.querySelector("video"), "webkitEnterFullscreen", {value: () => {
      (window as Window & {nativeFullscreenCalls: number}).nativeFullscreenCalls++;
    }});
  });
});

test.use({hasTouch: true});

test("native settings sliders retain their platform appearance @smoke", async ({page}) => {
  await page.addStyleTag({content: await readFile("../../../packages/webassets/static/player-stage.css", "utf8")});
  await page.evaluate(() => document.documentElement.dataset.theme = "light");
  await page.getByRole("button", {name: "Settings", exact: true}).click();
  await page.locator(".player-native-options .player-settings").evaluate(panel => {
    panel.insertAdjacentHTML("beforeend", '<label class="player-scrubber">Full video timeline<input type="range" min="0" max="100" value="20"></label>');
  });
  const seek = page.getByRole("slider", {name: "Full video timeline"});
  await expect(seek).toBeVisible();
  expect(await seek.evaluate(input => getComputedStyle(input).appearance)).not.toBe("none");
  await seek.focus();
  await page.keyboard.press("ArrowRight");
  await expect(seek).toHaveValue("21");
});

for (const viewport of [{width: 390, height: 844}, {width: 844, height: 390}, {width: 1440, height: 900}]) {
  test(`native controls retain playback when the picture is tapped at ${viewport.width}x${viewport.height}`, async ({page}, testInfo) => {
    await page.setViewportSize(viewport);
    const video = page.locator("video");
    await expect(video).toHaveJSProperty("controls", true);
    await page.getByRole("button", {name: "Play", exact: true}).click();
    await expect(video).toHaveJSProperty("paused", false);
    await expect(page.locator("[data-player-controls]")).toBeHidden();
    await video.dispatchEvent("click");
    await expect(video).toHaveJSProperty("paused", false);
    await expect(page.locator("[data-player-status]")).toBeHidden();
    await page.screenshot({path: testInfo.outputPath("native-playing.png"), fullPage: true});
  });
}

test("native settings sit below the picture, follow caption changes, and close with Escape", async ({page}) => {
  const settings = page.getByRole("button", {name: "Settings", exact: true});
  await expect.poll(async () => {
    const picture = await page.locator("video").boundingBox();
    const button = await settings.boundingBox();
    return !!picture && !!button && button.y >= picture.y + picture.height;
  }).toBe(true);
  await settings.click();
  await expect(page.locator(".player-settings")).toBeVisible();
  await expect(page.locator(".player-settings")).not.toContainText("T for Theater");
  await page.locator("video").evaluate(video => { video.textTracks[1].mode = "showing"; });
  await expect(page.locator("[data-subtitles]")).toHaveValue("1");
  await page.getByRole("combobox", {name: "Playback speed"}).selectOption("1.5");
  await expect(page.locator("video")).toHaveJSProperty("playbackRate", 1.5);
  await page.keyboard.press("Escape");
  await expect(page.locator(".player-settings")).toBeHidden();
  await expect(settings).toBeFocused();
});

test("touch Play enters native fullscreen in the same gesture without pausing", async ({page}) => {
  await page.getByRole("button", {name: "Play", exact: true}).click();
  expect(await page.evaluate(() => (window as Window & {nativeFullscreenCalls: number}).nativeFullscreenCalls)).toBe(1);
  await expect(page.locator("video")).toHaveJSProperty("paused", false);
});

test("rejected fullscreen leaves playback usable", async ({page}) => {
  const failures: unknown[] = [];
  page.on("request", request => {
    if (request.url().endsWith("/playback-events") && request.method() === "POST" && request.postDataJSON().event === "error") failures.push(request.postDataJSON());
  });
  await page.getByRole("button", {name: "Enter fullscreen"}).click();
  await expect(page.getByRole("status").filter({hasText: /Fullscreen could not open/})).toBeVisible();
  await expect.poll(() => failures.length).toBe(1);
  const diagnostics = JSON.stringify(failures);
  expect(diagnostics).toContain("trace-session");
  expect(diagnostics).toContain("NotAllowedError");
  expect(diagnostics).toContain("playback-retained");
  expect(diagnostics).not.toMatch(/private-token|private.invalid/);
  await page.getByRole("button", {name: "Play", exact: true}).click();
  await expect(page.locator("video")).toHaveJSProperty("paused", false);
});

test("unsupported fullscreen stays disabled without changing playback", async ({page}) => {
  const fullscreen = page.getByRole("button", {name: "Enter fullscreen"});
  await expect(fullscreen).toBeDisabled();
  await fullscreen.dispatchEvent("click");
  await expect(page.locator("video")).toHaveJSProperty("paused", true);
  await page.getByRole("button", {name: "Play", exact: true}).click();
  await expect(page.locator("video")).toHaveJSProperty("paused", false);
});

test("touch native playback waits for Play despite resumed autoplay", async ({page}) => {
  const video = page.locator("video");
  await video.dispatchEvent("canplay");
  await expect(video).toHaveJSProperty("paused", true);
  await page.getByRole("button", {name: "Play", exact: true}).click();
  await expect(video).toHaveJSProperty("paused", false);
});

for (const width of [390, 1440, 1920]) {
  test(`native pending, loaded, and failed states preserve picture geometry at ${width}`, async ({page}, testInfo) => {
    await page.setViewportSize({width, height: 1080});
    const stage = page.locator(".media-stage");
    const video = page.locator("video");
    const status = page.locator("[data-player-status]");
    const loaded = await stage.boundingBox();
    await page.evaluate(() => (window as Window & {setReadyState: (value: number) => void}).setReadyState(1));
    await video.dispatchEvent("loadstart");
    await expect(status).toHaveAttribute("data-state", "loading");
    await expect(status.locator(".buffer-skeleton")).toBeVisible();
    expect(await stage.boundingBox()).toEqual(loaded);
    await page.screenshot({path: testInfo.outputPath("native-pending.png"), fullPage: true});
    await page.evaluate(() => (window as Window & {setReadyState: (value: number) => void}).setReadyState(4));
    await video.dispatchEvent("canplay");
    await expect(status).toBeHidden();
    expect(await stage.boundingBox()).toEqual(loaded);
    await video.evaluate(media => Object.defineProperty(media, "error", {value: {code: 0}}));
    await video.dispatchEvent("error");
    await expect(status).toContainText("Direct Play only is selected");
    await expect(status.getByRole("button", {name: "Retry Direct Play"})).toBeVisible();
    await expect(status.locator(".buffer-skeleton")).toBeHidden();
    await expect(page.locator("[data-player-controls]")).toBeHidden();
    expect(await stage.boundingBox()).toEqual(loaded);
    await page.screenshot({path: testInfo.outputPath("native-failed.png"), fullPage: true});
  });
}
