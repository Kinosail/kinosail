import { expect, test } from "@playwright/test";
import { readFile } from "node:fs/promises";
import { installPlayerExperienceFixture } from "./player-experience-fixture";

installPlayerExperienceFixture();

test("Safari startup preserves a pause requested before playback starts", async ({ page }) => {
  const video = page.locator("video");
  await page.evaluate(() => (window as Window & {setNetworkState: (value: number) => void}).setNetworkState(1));
  await video.dispatchEvent("loadstart");
  await video.evaluate((element) => element.dispatchEvent(new CustomEvent("kinosail:playback-intent", {detail: {playing: false}})));
  await page.clock.runFor(2_000);
  await expect(video).toHaveJSProperty("paused", true);
});

test("Safari startup accepts the remaining buffer near the end of a video", async ({ page }) => {
  await page.evaluate(() => {
    const context = window as Window & {setBufferedEnd: (value: number) => void; setReadyState: (value: number) => void};
    document.querySelector("video")!.currentTime = 99.5;
    context.setBufferedEnd(100);
    context.setReadyState(3);
  });
  await page.locator("video").dispatchEvent("loadstart");
  await page.locator("video").dispatchEvent("progress");
  await expect(page.locator("[data-player-status]")).toBeHidden();
});

for (const trigger of ["timer", "blocked autoplay", "canplay", "seeked"]) test(`Safari startup waits for a playable buffer after ${trigger}`, async ({ page }, testInfo) => {
  await page.addStyleTag({ content: await readFile("../../../packages/webassets/static/player-stage.css", "utf8") });
  await page.addStyleTag({ content: await readFile("../internal/server/static/home.css", "utf8") });
  await page.setViewportSize({ width: 390, height: 844 });
  const video = page.locator("video");
  const status = page.locator("[data-player-status]");
  const play = page.locator(".player-center-control[data-player-toggle]");
  await page.evaluate(() => {
    const context = window as Window & {setBufferedEnd: (value: number) => void; setReadyState: (value: number) => void};
    context.setBufferedEnd(20.1);
    context.setReadyState(1);
  });
  await video.dispatchEvent("loadstart");
  if (trigger === "blocked autoplay") await video.dispatchEvent("kinosail:play-needs-gesture");
  if (trigger === "canplay" || trigger === "seeked") {
    await page.evaluate(() => (window as Window & {setReadyState: (value: number) => void}).setReadyState(3));
    if (trigger === "seeked") await video.dispatchEvent("seeking");
    await video.dispatchEvent(trigger);
  }
  await page.clock.runFor(2_000);
  await expect(status).toBeVisible();
  await expect(status).toHaveAttribute("aria-busy", "true");
  await expect(play).toBeHidden();
  await page.screenshot({path: testInfo.outputPath("390-pending.png")});

  await page.evaluate(() => {
    const context = window as Window & {setBufferedEnd: (value: number) => void; setReadyState: (value: number) => void};
    context.setBufferedEnd(23);
    context.setReadyState(3);
  });
  await video.dispatchEvent("progress");
  await expect(status).toBeHidden();
  await expect(play).toBeVisible();
  await page.screenshot({path: testInfo.outputPath("390-ready.png")});
  await play.click();
  await expect(video).toHaveJSProperty("paused", false);
  await expect(status).toBeHidden();
});

for (const readyState of [1, 3]) test(`Safari startup keeps the required gesture usable when preloading stops at readyState ${readyState}`, async ({ page }, testInfo) => {
  await page.addStyleTag({ content: await readFile("../../../packages/webassets/static/player-stage.css", "utf8") });
  await page.addStyleTag({ content: await readFile("../internal/server/static/home.css", "utf8") });
  await page.setViewportSize({width: 390, height: 844});
  const video = page.locator("video");
  const status = page.locator("[data-player-status]");
  const play = page.locator(".player-center-control[data-player-toggle]");
  await page.evaluate((state) => {
    const context = window as Window & {setBufferedEnd: (value: number) => void; setReadyState: (value: number) => void; setNetworkState: (value: number) => void; setPlayFailure: (value: string) => void};
    context.setBufferedEnd(20.1);
    context.setReadyState(state);
    context.setNetworkState(1);
    context.setPlayFailure("NotAllowedError");
  }, readyState);
  await video.dispatchEvent("loadstart");
  await page.clock.runFor(2_000);
  await expect(play).toBeVisible();
  await expect(status).toBeHidden();
  await page.evaluate(() => {
    const context = window as Window & {setNetworkState: (value: number) => void; setPlayFailure: (value: string) => void; setPlayPending: (value: boolean) => void};
    context.setNetworkState(2);
    context.setPlayFailure("");
    context.setPlayPending(true);
  });
  await play.click();
  await expect(status).toBeVisible();
  await expect(play).toBeHidden();
  await page.clock.runFor(2_000);
  await expect(status).toBeVisible();
  await page.screenshot({path: testInfo.outputPath("390-gesture-loading.png")});
  await page.evaluate(() => (window as Window & {finishPlay: () => void}).finishPlay());
  await expect(status).toBeHidden();
  await expect(video).toHaveJSProperty("paused", false);
});

