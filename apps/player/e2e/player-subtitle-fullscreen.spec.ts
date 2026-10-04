import {expect, test} from "@playwright/test";
import {installPlayerExperienceFixture} from "./player-experience-fixture";

installPlayerExperienceFixture();

test("limited native fullscreen fallback opens without changing inline subtitle choices", async ({ page }, testInfo) => {
  const fullscreen = page.locator("[data-player-fullscreen]");
  await expect(fullscreen).toBeEnabled();
  await expect(fullscreen).toBeVisible();
  await expect(fullscreen).toHaveAttribute("title", /native player and subtitle menu/);
  await fullscreen.click();
  expect(await page.evaluate(() => (window as Window & {nativeFullscreenCalls?: number}).nativeFullscreenCalls || 0)).toBe(1);
  await expect(page.locator("video")).toHaveAttribute("data-subtitle-picker-limited", "true");
  await expect(page.getByRole("button", { name: "Theater" })).toBeEnabled();
  await page.getByRole("button", { name: "Play" }).first().click();
  await expect(page.locator("[data-player-status]")).toBeHidden();
  for (const width of [390, 1440]) {
    await page.setViewportSize({width, height: 844});
    await page.screenshot({path: testInfo.outputPath(`limited-subtitles-${width}.png`)});
  }
});

test("element fullscreen enters and exits without changing playback", async ({page, browserName}) => {
  test.skip(browserName !== "chromium", "Real element fullscreen is available in this Chromium runner; Safari native UI needs a device.");
  await page.getByRole("button", {name: "Play", exact: true}).first().click();
  await page.getByRole("button", {name: "Enter fullscreen"}).click();
  await expect.poll(() => page.evaluate(() => document.fullscreenElement?.classList.contains("media-stage"))).toBe(true);
  await expect(page.locator("video")).toHaveJSProperty("paused", false);
  await page.getByRole("button", {name: "Exit fullscreen"}).click();
  await expect.poll(() => page.evaluate(() => document.fullscreenElement)).toBeNull();
  await expect(page.locator("video")).toHaveJSProperty("paused", false);
});

test("native fullscreen fallback follows Safari entry and exit events", async ({page}) => {
  await page.locator("video").evaluate(video => {
    let fullscreen = false;
    Object.defineProperties(video, {
      webkitDisplayingFullscreen: {get: () => fullscreen},
      webkitEnterFullscreen: {configurable: true, value: () => {
        fullscreen = true;
        video.dispatchEvent(new Event("webkitbeginfullscreen"));
      }},
      webkitExitFullscreen: {value: () => {
        fullscreen = false;
        video.dispatchEvent(new Event("webkitendfullscreen"));
      }},
    });
  });
  await page.getByRole("button", {name: "Enter fullscreen"}).click();
  await expect(page.getByRole("button", {name: "Exit fullscreen"})).toBeVisible();
  await page.getByRole("button", {name: "Exit fullscreen"}).click();
  await expect(page.getByRole("button", {name: "Enter fullscreen"})).toBeVisible();
  await expect(page.locator("video")).toHaveJSProperty("webkitDisplayingFullscreen", false);
});

test("limited native fullscreen fallback rejection keeps inline playback usable", async ({page}) => {
  await page.locator("video").evaluate(video => Object.defineProperty(video, "webkitEnterFullscreen", {
    configurable: true, value: () => { throw new DOMException("denied", "NotAllowedError"); },
  }));
  await page.getByRole("button", {name: "Play", exact: true}).first().click();
  await page.getByRole("button", {name: "Enter fullscreen"}).click();
  await expect(page.getByRole("status").filter({hasText: /Fullscreen could not open/})).toBeVisible();
  await expect(page.locator("video")).toHaveJSProperty("paused", false);
  await expect(page.locator("video")).toHaveJSProperty("currentTime", 20);
});
