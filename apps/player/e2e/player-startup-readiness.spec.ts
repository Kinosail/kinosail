import { expect, test } from "@playwright/test";
import { readFile } from "node:fs/promises";
import { installPlayerExperienceFixture } from "./player-experience-fixture";

installPlayerExperienceFixture();

for (const scenario of [
  { name: "a leading frame timestamp", start: 0.021402, end: 3, ready: 3, playable: true },
  { name: "a large unbuffered gap", start: 0.5, end: 3, ready: 3, playable: false },
  { name: "an insufficient buffer", start: 0.021402, end: 0.1, ready: 3, playable: false },
  { name: "metadata without future data", start: 0.021402, end: 3, ready: 1, playable: false },
]) test(`Safari startup handles ${scenario.name}`, async ({ page }) => {
  await page.evaluate(({ start, end, ready }) => {
    const context = window as Window & { setBufferedStart: (value: number) => void; setBufferedEnd: (value: number) => void; setReadyState: (value: number) => void };
    const video = document.querySelector("video")!;
    video.currentTime = 0;
    video.dataset.start = "0";
    context.setBufferedStart(start);
    context.setBufferedEnd(end);
    context.setReadyState(ready);
  }, scenario);
  await page.locator("video").dispatchEvent("loadstart");
  await page.locator("video").dispatchEvent("canplay");
  await expect(page.locator("[data-player-status]")).toBeVisible({ visible: !scenario.playable, timeout: 1000 });
  await expect(page.locator(".player-center-control[data-player-toggle]")).toBeVisible({ visible: scenario.playable });
});

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

test("Safari startup waits after preparation restores a position following blocked autoplay", async ({ page }) => {
  const video = page.locator("video");
  await page.evaluate(() => {
    const context = window as Window & {setBufferedEnd: (value: number) => void; setPlayPending: (value: boolean) => void};
    context.setBufferedEnd(20.1);
    context.setPlayPending(true);
  });
  await video.dispatchEvent("loadstart");
  await video.dispatchEvent("kinosail:play-needs-gesture");
  await page.evaluate(() => {
    const context = window as Window & {setBufferedEnd: (value: number) => void; setReadyState: (value: number) => void};
    context.setBufferedEnd(23);
    context.setReadyState(3);
    const video = document.querySelector("video")!;
    video.currentTime = 20.2;
    video.addEventListener("pause", () => context.setReadyState(2), {once: true});
  });
  await video.dispatchEvent("progress");
  await expect(video).toHaveJSProperty("currentTime", 20);
  await expect(page.locator("[data-player-status]")).toBeVisible();
  // WebKit can deliver the preparation's queued playing event after pause and seek.
  await video.dispatchEvent("playing");
  await expect(video).toHaveJSProperty("paused", true);
  await expect(page.locator("[data-player-status]")).toBeVisible();
  await page.evaluate(() => (window as Window & {setReadyState: (value: number) => void}).setReadyState(3));
  await video.dispatchEvent("canplay");
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
  await video.dispatchEvent("seeking");
  await video.dispatchEvent("seeked");
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


for (const pauseDelivery of ["immediate", "queued pause"]) test(`Safari startup prepares muted media without a loading tap or progress save with ${pauseDelivery}`, async ({ page }, testInfo) => {
  const saves: string[] = [];
  page.on("request", (request) => { if (request.url().includes("/progress/movie")) saves.push(request.postData() || ""); });
  await page.addStyleTag({ content: await readFile("../../../packages/webassets/static/player-stage.css", "utf8") });
  const video = page.locator("video");
  const status = page.locator("[data-player-status]");
  await page.evaluate(() => {
    const context = window as Window & {setBufferedEnd: (value: number) => void; setPlayPending: (value: boolean) => void};
    context.setBufferedEnd(20.1);
    context.setPlayPending(true);
  });
  await video.dispatchEvent("loadstart");
  await expect(video).toHaveJSProperty("muted", true);
  await expect(status).toBeVisible();
  await expect(page.locator(".player-center-control[data-player-toggle]")).toBeHidden();
  await page.clock.runFor(2_000);
  await expect(status).toBeVisible();
  await page.evaluate(() => {
    const context = window as Window & {setBufferedEnd: (value: number) => void; setReadyState: (value: number) => void};
    context.setBufferedEnd(23);
    context.setReadyState(3);
    document.querySelector("video")!.currentTime = 20.2;
  });
  await video.dispatchEvent("progress");
  await expect(video).toHaveJSProperty("paused", true);
  await expect(video).toHaveJSProperty("muted", false);
  await expect(video).toHaveJSProperty("currentTime", 20);
  await expect(status).toBeHidden();
  await page.clock.runFor(1);
  await page.waitForTimeout(100);
  expect(saves).toEqual([]);
  await page.screenshot({path: testInfo.outputPath("390-prepared-play.png")});
  await page.evaluate(() => (window as Window & {setPlayPending: (value: boolean) => void}).setPlayPending(false));
  await page.locator(".player-center-control[data-player-toggle]").click();
  await expect(video).toHaveJSProperty("paused", false);
  await expect(video).toHaveJSProperty("muted", false);
  await expect(status).toBeHidden();
  // A stale preparation promise must not pause the user's playback.
  await page.evaluate(() => (window as Window & {finishPlay: () => void}).finishPlay());
  await expect(video).toHaveJSProperty("paused", false);
  await page.evaluate(() => (window as Window & {advanceMediaTime: (value: number) => void}).advanceMediaTime(25));
  await video.evaluate((element: HTMLVideoElement) => element.pause());
  await expect.poll(() => saves.length).toBe(1);
  expect(new URLSearchParams(saves[0]).get("seconds")).toBe("25");
  expect(new URLSearchParams(saves[0]).get("watched")).toBe("false");
});

for (const networkState of [1, 2]) test(`Safari startup shows Play when even muted preparation is rejected at networkState ${networkState}`, async ({ page }) => {
  await page.addStyleTag({ content: await readFile("../../../packages/webassets/static/player-stage.css", "utf8") });
  const video = page.locator("video");
  await page.evaluate((state) => {
    const context = window as Window & {setBufferedEnd: (value: number) => void; setNetworkState: (value: number) => void; setPlayFailure: (value: string) => void};
    context.setBufferedEnd(20.1);
    context.setNetworkState(state);
    context.setPlayFailure("NotAllowedError");
  }, networkState);
  await video.dispatchEvent("loadstart");
  await page.clock.runFor(2_000);
  await expect(page.locator("[data-player-status]")).toBeHidden();
  await expect(video).toHaveJSProperty("muted", false);
  await expect(page.locator(".player-center-control[data-player-toggle]")).toBeVisible();
  await page.evaluate(() => {
    const context = window as Window & {setPlayFailure: (value: string) => void; setPlayPending: (value: boolean) => void};
    context.setPlayFailure("");
    context.setPlayPending(true);
  });
  await page.locator(".player-center-control[data-player-toggle]").click();
  await expect(page.locator("[data-player-status]")).toBeVisible();
  await page.evaluate(() => (window as Window & {finishPlay: () => void}).finishPlay());
  await expect(page.locator("[data-player-status]")).toBeHidden();
  await expect(video).toHaveJSProperty("paused", false);
});


test("Safari startup preserves a newer seek while preparing media", async ({ page }) => {
  const video = page.locator("video");
  await page.evaluate(() => {
    const context = window as Window & {setBufferedEnd: (value: number) => void; setPlayPending: (value: boolean) => void};
    context.setBufferedEnd(20.1);
    context.setPlayPending(true);
  });
  await video.dispatchEvent("loadstart");
  await video.evaluate((element) => { element.currentTime = 5; });
  await video.dispatchEvent("seeking");
  await page.evaluate(() => {
    const context = window as Window & {setBufferedEnd: (value: number) => void; setReadyState: (value: number) => void};
    context.setBufferedEnd(8);
    context.setReadyState(3);
  });
  await video.dispatchEvent("canplay");
  await expect(video).toHaveJSProperty("paused", true);
  await expect(video).toHaveJSProperty("currentTime", 5);
  await expect(video).toHaveJSProperty("muted", false);
});

test("Safari startup prevents progress saves from overlapping queued pause events", async ({ page }) => {
  const saves: string[] = [];
  page.on("request", (request) => { if (request.url().includes("/progress/movie")) saves.push(request.url()); });
  await page.evaluate(() => {
    const context = window as Window & {setBufferedEnd: (value: number) => void; setReadyState: (value: number) => void};
    const video = document.querySelector("video")!;
    context.setBufferedEnd(20.1);
    video.dispatchEvent(new Event("loadstart"));
    video.dispatchEvent(new Event("loadstart"));
    context.setReadyState(3);
    context.setBufferedEnd(23);
    video.dispatchEvent(new Event("progress"));
  });
  await page.waitForTimeout(100);
  expect(saves).toEqual([]);
  await expect(page.locator("video")).toHaveJSProperty("paused", true);
  await expect(page.locator("video")).toHaveJSProperty("muted", false);
});

for (const firstFrame of [20.021, 21.937]) test(`Safari startup restores playable media when its first frame is at ${firstFrame}`, async ({ page }) => {
  const video = page.locator("video");
  await page.evaluate(() => {
    const context = window as Window & {setBufferedEnd: (value: number) => void; setPlayPending: (value: boolean) => void};
    context.setBufferedEnd(20.1);
    context.setPlayPending(true);
  });
  await video.dispatchEvent("loadstart");
  await page.evaluate((start) => {
    const context = window as Window & {setBufferedStart: (value: number) => void; setBufferedEnd: (value: number) => void; setReadyState: (value: number) => void; advanceMediaTime: (value: number) => void};
    context.setBufferedStart(start);
    context.setBufferedEnd(start + 3);
    context.setReadyState(3);
    context.advanceMediaTime(start + 0.2);
  }, firstFrame);
  await video.dispatchEvent("progress");
  await expect(video).toHaveJSProperty("paused", true);
  await expect(video).toHaveJSProperty("muted", false);
  await expect(video).toHaveJSProperty("currentTime", firstFrame);
  await expect(page.locator("[data-player-status]")).toBeHidden();
  await expect(page.locator(".player-center-control[data-player-toggle]")).toBeVisible();
  await page.evaluate(() => (window as Window & {setPlayPending: (value: boolean) => void}).setPlayPending(false));
  await page.locator(".player-center-control[data-player-toggle]").click();
  await expect(video).toHaveJSProperty("paused", false);
  await expect(page.locator("[data-player-status]")).toBeHidden();
});
