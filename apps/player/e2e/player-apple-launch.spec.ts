import {writeFile} from "node:fs/promises";
import {expect, test} from "@playwright/test";
import {installPlayerExperienceFixture} from "./player-experience-fixture";

test.describe("Apple launch policy @smoke", () => {
  test.use({hasTouch: true, viewport: {width: 390, height: 844}, ignoreHTTPSErrors: false});
  installPlayerExperienceFixture(false, true);
  test.afterEach(async ({browserName}, info) => {
    const path = info.outputPath("apple-launch-receipt.json");
    await writeFile(path, JSON.stringify({revision: process.env.KINOSAIL_TEST_REVISION || process.env.GITHUB_SHA,
      command: "node node_modules/@playwright/test/cli.js test player-apple-launch.spec.ts --workers=1",
      environment: browserName, data: "Isolated media state; simulated Apple fullscreen API",
      result: info.status, boundary: "Does not prove physical iPhone fullscreen or media transport"}, null, 2));
    await info.attach("apple-launch-receipt", {path, contentType: "application/json"});
  });

  test("cold metadata never hides Play or starts muted preparation", async ({page}) => {
    const video = page.locator("video");
    await page.evaluate(() => {
      const state = window as Window & {setReadyState: (value: number) => void; setBufferedEnd: (value: number) => void};
      state.setReadyState(0); state.setBufferedEnd(20.1);
    });
    await video.dispatchEvent("loadstart");
    await video.dispatchEvent("suspend");
    await expect(video).toHaveJSProperty("paused", true);
    await expect(video).toHaveJSProperty("muted", false);
    await expect(video).toHaveJSProperty("playsInline", false);
    await expect(page.locator("[data-player-status]")).toBeHidden();
    await expect(page.getByRole("button", {name: "Play", exact: true})).toHaveCount(1);
    await expect(page.getByRole("button", {name: "Play", exact: true})).toBeVisible();
    await expect(page.locator(".player-control-dock")).toBeHidden();
    await video.evaluate(media => Object.defineProperty(media, "webkitEnterFullscreen", {value: () => { throw new Error("cold fullscreen must use Safari's play presentation"); }}));
    await page.getByRole("button", {name: "Play", exact: true}).tap();
    await expect(video).toHaveJSProperty("paused", false);
    await expect(video).toHaveJSProperty("controls", true);
    await expect(page.locator(".player-control-feedback")).toBeHidden();
  });

  test("ready Play opens Apple, dismissal pauses and next Play reenters", async ({page}) => {
    const video = page.locator("video");
    await expect(page.getByRole("button", {name: "Settings", exact: true})).toBeVisible();
    await page.getByRole("button", {name: "Play", exact: true}).tap();
    await expect(video).toHaveJSProperty("webkitDisplayingFullscreen", true);
    await expect(video).toHaveJSProperty("paused", false);
    await video.evaluate(media => { media.pause(); media.currentTime = 35; void media.play(); });
    await expect(video).toHaveJSProperty("currentTime", 35);
    await video.evaluate(media => (media as HTMLVideoElement & {webkitExitFullscreen: () => void}).webkitExitFullscreen());
    await expect(video).toHaveJSProperty("paused", true);
    await expect(video).toHaveJSProperty("controls", false);
    await expect(page.getByRole("button", {name: "Play", exact: true})).toBeVisible();
    await page.getByRole("button", {name: "Settings", exact: true}).tap();
    await page.locator("[data-subtitles]").selectOption("0");
    await page.locator("[data-player-settings-close]").tap();
    await page.getByRole("button", {name: "Play", exact: true}).tap();
    await expect(video).toHaveJSProperty("webkitDisplayingFullscreen", true);
    await expect(page.locator("[data-subtitles]")).toHaveValue("0");
  });

  test("limited in-band Apple playback emits no custom theater errors", async ({page}) => {
    const errors: string[] = [];
    page.on("pageerror", error => errors.push(error.message));
    await page.getByRole("button", {name: "Play", exact: true}).tap();
    await page.locator("video").evaluate(media => media.pause());
    await page.getByRole("button", {name: "Settings", exact: true}).tap();
    expect(errors).toEqual([]);
  });

  test("metadata-only resume seek keeps Apple Play accessible", async ({page}) => {
    const video = page.locator("video");
    await page.evaluate(() => {
      const state = window as Window & {setReadyState: (value: number) => void; setBufferedEnd: (value: number) => void};
      state.setReadyState(1); state.setBufferedEnd(20.1);
      document.querySelector("video")!.currentTime = 37;
    });
    await video.dispatchEvent("seeking");
    await video.dispatchEvent("seeked");
    await expect(video).toHaveJSProperty("paused", true);
    await expect(page.locator("[data-player-status]")).toBeHidden();
    await expect(page.getByRole("button", {name: "Play", exact: true})).toBeVisible();
  });

  for (const failure of ["fullscreen", "play"]) test(`${failure} rejection restores Play without inline fallback`, async ({page}) => {
    const video = page.locator("video");
    if (failure === "fullscreen") await video.evaluate(media => Object.defineProperty(media, "webkitEnterFullscreen", {value: () => { throw new DOMException("private media detail", "InvalidStateError"); }}));
    else await page.evaluate(() => (window as Window & {setPlayFailure: (value: string) => void}).setPlayFailure("NotAllowedError"));
    await page.getByRole("button", {name: "Play", exact: true}).tap();
    await expect(video).toHaveJSProperty("paused", true);
    await expect(video).toHaveJSProperty("webkitDisplayingFullscreen", false);
    await expect(page.getByRole("button", {name: "Play", exact: true})).toBeVisible();
    await expect(page.locator(".player-control-feedback")).toBeVisible();
    await expect(page.locator(".player-control-feedback")).not.toContainText("private media detail");
  });
});
