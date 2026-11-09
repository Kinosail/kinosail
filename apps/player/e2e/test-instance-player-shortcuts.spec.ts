import { expect, test } from "@playwright/test";
import { configureTestInstance, firstPlayable, login } from "./test-instance-helpers";

configureTestInstance();
test.use({ serviceWorkers: "block" });

test("player keyboard shortcuts seek, change volume and speed, and keep volume after reload", { tag: "@smoke" }, async ({ page }, info) => {
  await login(page);
  const watch = await firstPlayable(page);
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto(watch);
  const video = page.locator("video");
  const stage = page.locator(".media-stage");
  const notice = page.locator(".player-shortcut-feedback");
  const state = () => video.evaluate((media: HTMLVideoElement) => ({ time: media.currentTime, volume: media.volume, muted: media.muted, rate: media.playbackRate }));
  await expect.poll(() => video.evaluate((media: HTMLVideoElement) => media.readyState)).toBeGreaterThan(0);
  const duration = await video.evaluate((media: HTMLVideoElement) => { media.pause(); return media.duration; });
  expect(duration).toBeGreaterThan(10);
  try {
    await stage.focus();
    await expect(notice).toBeHidden();
    await page.keyboard.press("l");
    await expect.poll(async () => (await state()).time).toBeCloseTo(10, 0);
    await page.keyboard.press("j");
    await expect.poll(async () => (await state()).time).toBeCloseTo(0, 0);
    await page.keyboard.press("5");
    await expect.poll(async () => (await state()).time).toBeCloseTo(duration / 2, 0);
    await page.keyboard.press("Home");
    await expect.poll(async () => (await state()).time).toBeCloseTo(0, 0);

    await page.keyboard.press("Control+m");
    expect((await state()).muted).toBe(false);
    await page.keyboard.press("m");
    expect((await state()).muted).toBe(true);
    await expect(notice).toHaveText("Muted");
    await page.keyboard.press("ArrowDown");
    await page.keyboard.press("ArrowUp");
    await page.keyboard.press("ArrowUp");
    await page.keyboard.press("ArrowDown");
    const quiet = await state();
    expect(quiet.muted).toBe(false);
    expect(quiet.volume).toBeCloseTo(0.05, 2);
    await expect(notice).toHaveText("Volume 5%");
    await expect(notice).toHaveAttribute("role", "status");

    await page.keyboard.press("Shift+>");
    expect((await state()).rate).toBe(1.25);
    await expect(notice).toHaveText("Speed 1.25×");
    await page.keyboard.press("Shift+<");
    expect((await state()).rate).toBe(1);
    await expect(notice).toHaveText("Normal speed");
    await expect(notice).toBeHidden({ timeout: 5_000 });

    await page.keyboard.press("f");
    await expect.poll(() => page.evaluate(() => Boolean(document.fullscreenElement))).toBe(true);
    await page.keyboard.press("f");
    await expect.poll(() => page.evaluate(() => Boolean(document.fullscreenElement))).toBe(false);
    await stage.screenshot({ path: info.outputPath("player-shortcuts.png") });

    await page.reload();
    await expect.poll(() => video.evaluate((media: HTMLVideoElement) => media.readyState)).toBeGreaterThan(0);
    const restored = await state();
    expect(restored.volume).toBeCloseTo(0.05, 2);
    expect(restored.muted).toBe(false);
    await info.attach("player-shortcuts", { body: JSON.stringify({ revision: process.env.KINOSAIL_TEST_REVISION, browser: info.project.name, watch, duration,
      keys: ["l", "j", "5", "Home", "Control+m", "m", "ArrowDown", "ArrowUp", "ArrowUp", "ArrowDown", "Shift+>", "Shift+<", "f"], restored,
      data: "Real populated Server movie; keyboard events on the focused player stage", result: "passed" }), contentType: "application/json" });
  } finally {
    await page.evaluate(() => { localStorage.removeItem("kinosail.player.volume"); localStorage.removeItem("kinosail.player.muted"); });
  }
});
