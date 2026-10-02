import { expect, test, type Page } from "@playwright/test";
import { startDirectPlayer } from "./player-direct-fallback-fixture";

const instances = (page: Page) => page.evaluate(() => (window as Window & { FakeHls: { instances: number } }).FakeHls.instances);

test("a direct format error before script initialization prepares video conversion automatically", { tag: "@smoke" }, async ({ page }) => {
  await startDirectPlayer(page, { initialError: 4, compatibleMode: "transcode" });
  const action = page.locator("[data-player-status] [data-player-fallback]");
  await expect.poll(() => instances(page)).toBe(1);
  await expect(action).toBeHidden();
  await expect(page.locator("[data-playback-recovery]")).toBeHidden();
});

for (const code of [3, 4]) {
  test(`a preexisting direct error ${code} starts minimal compatibility`, async ({ page }) => {
    await startDirectPlayer(page, { initialError: code });
    await expect.poll(() => instances(page)).toBe(1);
    await expect(page.locator("[data-playback-mode-status]")).toHaveText("Remux");
    await expect(page.locator("video")).toHaveJSProperty("currentTime", 42);
  });
}

test("a preexisting direct error preserves Direct Play only", async ({ page }) => {
  await startDirectPlayer(page, { initialError: 4, playbackPolicy: "direct" });
  await expect(page.locator("[data-player-status] [data-player-fallback]")).toHaveText("Retry Direct Play");
  expect(await instances(page)).toBe(0);
});

test("a preexisting aborted source does not start conversion", async ({ page }) => {
  await startDirectPlayer(page, { initialError: 1 });
  await expect(page.locator("[data-player-status] [data-player-fallback]")).toBeHidden();
  expect(await instances(page)).toBe(0);
});

test("a preexisting error does not duplicate known audio compatibility", async ({ page }) => {
  await startDirectPlayer(page, { initialError: 4, compatibleMode: "audio-transcode", initialHls: true });
  await expect.poll(() => instances(page)).toBe(1);
  await expect(page.locator("[data-player-status] [data-player-fallback]")).toBeHidden();
});

test("a preexisting network error retries Direct Play without conversion", async ({ page }) => {
  await page.clock.install();
  await startDirectPlayer(page, { initialError: 2 });
  await expect(page.locator("[data-player-status]")).toContainText("Connection interrupted. Reconnecting…");
  expect(await instances(page)).toBe(0);
  await page.clock.fastForward(5_000);
  await expect.poll(() => page.evaluate(() => (window as Window & { directLoads?: number }).directLoads || 0)).toBe(1);
  expect(await instances(page)).toBe(0);
});
