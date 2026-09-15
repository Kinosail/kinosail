import { readFile } from "node:fs/promises";
import { expect, test } from "@playwright/test";
const core = await readFile(new URL("../../../packages/webassets/static/player-core.js", import.meta.url), "utf8");

test("frame telemetry distinguishes a still frame from advancing presentation", async ({ page }) => {
  const events: string[] = [];
  await page.route("https://trace.test/events", route => {
    if (route.request().method() === 'POST') events.push(route.request().postDataJSON().event);
    return route.fulfill({ status: 204, headers: { 'Access-Control-Allow-Origin': '*', 'Access-Control-Allow-Headers': 'Content-Type', 'Access-Control-Allow-Methods': 'POST, OPTIONS' } });
  });
  await page.setContent('<video data-playback-trace="https://trace.test/events" data-playback-session="test-session"></video>');
  await page.evaluate(() => {
    const video = document.querySelector("video")!;
    let next = 0;
    const callbacks = new Map<number, VideoFrameRequestCallback>();
    Object.defineProperties(video, {
      currentTime: { value: 10, writable: true },
      requestVideoFrameCallback: { value: (callback: VideoFrameRequestCallback) => { callbacks.set(++next, callback); return next; } },
      cancelVideoFrameCallback: { value: (id: number) => callbacks.delete(id) },
    });
    Object.assign(window, { present: (mediaTime: number) => {
      const pending = [...callbacks.values()];
      callbacks.clear();
      for (const callback of pending) callback(performance.now(), { mediaTime, presentedFrames: next } as VideoFrameCallbackMetadata);
    } });
  });
  await page.addScriptTag({ content: core });
  await page.evaluate(() => {
    const context = window as Window & { present(time: number): void };
    context.present(10);
    context.present(10);
    document.querySelector("video")!.dispatchEvent(new Event("playing"));
  });
  await expect.poll(() => events.filter(event => event === "first-frame").length).toBe(1);
  expect(events).not.toContain("first-moving-frame");
  await page.evaluate(() => {
    (window as Window & { present(time: number): void }).present(10.04);
    document.querySelector("video")!.dispatchEvent(new Event("playing"));
  });
  await expect.poll(() => events.filter(event => event === "first-moving-frame").length).toBe(1);
  await page.evaluate(() => {
    const video = document.querySelector("video")!;
    video.dispatchEvent(new Event("seeking"));
    video.currentTime = 60;
    video.dispatchEvent(new Event("seeked"));
    (window as Window & { present(time: number): void }).present(60);
    video.dispatchEvent(new Event("playing"));
  });
  await expect.poll(() => events.filter(event => event === "frame-after-seek").length).toBe(1);
  expect(events.filter(event => event === "first-frame")).toHaveLength(1);
});
