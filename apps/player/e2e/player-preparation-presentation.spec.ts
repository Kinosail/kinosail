import { expect, test } from "@playwright/test";
import { readFile } from "node:fs/promises";
import { installPlayerExperienceFixture } from "./player-experience-fixture";

installPlayerExperienceFixture();

for (const width of [390, 1440]) test(`Safari startup preserves pending player geometry and media session state at ${width}px`, async ({ page }, testInfo) => {
  await page.setViewportSize({width, height: 900});
  await page.addStyleTag({ content: await readFile("../../../packages/webassets/static/player-stage.css", "utf8") });
  const video = page.locator("video");
  const stage = page.locator(".media-stage");
  const toolbar = page.locator(".player-stage-toolbar");
  await page.evaluate(() => (window as Window & {setBufferedEnd: (value: number) => void}).setBufferedEnd(20.1));
  const initialHeight = (await stage.boundingBox())!.height;
  await video.dispatchEvent("loadstart");
  await expect(video).toHaveJSProperty("muted", true);
  await page.clock.runFor(3_000);
  await expect(page.locator("[data-player-status]")).toBeVisible();
  await expect(toolbar).toBeVisible();
  await expect(stage).not.toHaveClass(/is-playing/);
  expect(await page.evaluate(() => navigator.mediaSession?.playbackState)).not.toBe("playing");
  expect((await stage.boundingBox())!.height).toBe(initialHeight);
  await page.screenshot({path: testInfo.outputPath(`${width}-pending-layout.png`)});
  await page.evaluate(() => {
    const context = window as Window & {setBufferedEnd: (value: number) => void; setReadyState: (value: number) => void};
    context.setBufferedEnd(23);
    context.setReadyState(3);
  });
  await video.dispatchEvent("progress");
  await expect(video).toHaveJSProperty("paused", true);
  await expect(page.locator("[data-player-status]")).toBeHidden();
  expect((await stage.boundingBox())!.height).toBe(initialHeight);
  await video.dispatchEvent("play");
  await video.dispatchEvent("playing");
  await expect(stage).not.toHaveClass(/is-playing/);
  expect(await page.evaluate(() => navigator.mediaSession?.playbackState)).not.toBe("playing");
  await page.locator(".player-center-control[data-player-toggle]").click();
  await expect(stage).toHaveClass(/is-playing/);
});
