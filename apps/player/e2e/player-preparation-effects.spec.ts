import { expect, test } from "@playwright/test";
import { installPlayerExperienceFixture } from "./player-experience-fixture";

installPlayerExperienceFixture();

test("Safari startup preserves automatic skips with queued seeking events", async ({ page }) => {
  const video = page.locator("video");
  await page.evaluate(() => {
    const context = window as Window & {setBufferedEnd: (value: number) => void; setPlayPending: (value: boolean) => void};
    context.setBufferedEnd(0.1);
    context.setPlayPending(true);
  });
  await video.dispatchEvent("loadstart");
  await page.evaluate(() => {
    const context = window as Window & {setBufferedEnd: (value: number) => void; setReadyState: (value: number) => void; advanceMediaTime: (value: number) => void};
    context.advanceMediaTime(0.3);
    context.setBufferedEnd(3);
    context.setReadyState(3);
  });
  await video.dispatchEvent("progress");
  await expect(video).toHaveJSProperty("currentTime", 0);
  await expect(page.locator("[data-marker]")).not.toHaveAttribute("data-skipped", "true");
  await video.dispatchEvent("timeupdate");
  await expect(video).toHaveJSProperty("currentTime", 10);
});

test("Safari startup keeps watch-room playback under explicit control", async ({ page }) => {
  const video = page.locator("video");
  await video.evaluate((element) => { element.dataset.room = "example-room"; });
  await video.dispatchEvent("loadstart");
  await page.clock.runFor(2_000);
  await expect(video).toHaveJSProperty("muted", false);
  await expect(video).toHaveJSProperty("paused", true);
});

test("Safari startup cannot mark media watched or advance while preparing", async ({ page }) => {
  const video = page.locator("video");
  await page.evaluate(() => (window as Window & {setBufferedEnd: (value: number) => void}).setBufferedEnd(20.1));
  await video.evaluate((element) => { element.dataset.next = "/next-episode"; });
  await video.dispatchEvent("loadstart");
  await video.dispatchEvent("ended");
  await page.clock.runFor(100);
  expect(page.url()).toBe("about:blank");
});
