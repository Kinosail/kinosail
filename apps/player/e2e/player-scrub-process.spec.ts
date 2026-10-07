import { expect, test } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { writeFile } from "node:fs/promises";
import { login, firstPlayable } from "./test-instance-helpers";

test("generated video previews decode, reuse frames and seek with a visible timeline", async ({ page }, info) => {
  test.skip(process.env.KINOSAIL_SCRUB_PROCESS !== "1", "requires the isolated synthetic scrub runner");
  await login(page);
  await page.goto(await firstPlayable(page));
  const video = page.locator("video");
  const seek = page.getByRole("slider", { name: "Seek", exact: true });
  const preview = page.locator("[data-seek-preview]");
  await expect.poll(() => video.evaluate(media => media.readyState)).toBeGreaterThanOrEqual(2);
  await video.evaluate(media => media.pause());
  const initialPosition = await video.evaluate(media => media.currentTime);
  await expect(seek).toBeVisible();
  const requests: { path: string; status: number }[] = [];
  page.on("response", response => {
    if (new URL(response.url()).pathname.startsWith("/trickplay/")) requests.push({ path: new URL(response.url()).pathname, status: response.status() });
  });
  const measurements: { position: number; cached: boolean; milliseconds: number }[] = [];
  for (const cached of [false, true]) for (const position of [5, 15, 25]) {
    const result = await seek.evaluate(async (input: HTMLInputElement, seconds) => {
      const started = performance.now();
      input.value = String(seconds);
      input.dispatchEvent(new Event("input", { bubbles: true }));
      const frame = document.querySelector<HTMLImageElement>("[data-seek-preview] img");
      if (!frame?.complete || !frame.naturalWidth) await new Promise<void>((resolve, reject) => {
        const deadline = performance.now() + 10_000;
        const inspect = () => {
          const image = document.querySelector<HTMLImageElement>("[data-seek-preview] img");
          if (image?.complete && image.naturalWidth) resolve();
          else if (performance.now() > deadline) reject(new Error("preview did not decode"));
          else requestAnimationFrame(inspect);
        };
        inspect();
      });
      return performance.now() - started;
    }, position);
    measurements.push({ position, cached, milliseconds: result });
    await expect(preview.locator("img")).toBeVisible();
    await expect(preview.locator("img")).toHaveJSProperty("naturalWidth", 320);
    await seek.dispatchEvent("blur");
  }
  expect(requests).toHaveLength(3);
  expect(requests.every(response => response.status === 200)).toBe(true);
  expect(await video.evaluate(media => media.currentTime)).toBe(initialPosition);
  await video.evaluate(media => {
    Object.assign(window, { observedSeeks: [] });
    media.addEventListener("seeking", () => {
      const seek = document.querySelector<HTMLElement>("[data-player-seek]")!;
      const style = getComputedStyle(seek.closest("[data-player-controls]")!);
      (window as Window & { observedSeeks: boolean[] }).observedSeeks.push(style.visibility === "visible" && style.opacity === "1" && seek.getBoundingClientRect().width > 0);
    });
  });
  for (const [index, viewport] of [{ width: 390, height: 844 }, { width: 1440, height: 900 }, { width: 1920, height: 1080 }].entries()) {
    const position = [15, 5, 25][index];
    await page.setViewportSize(viewport);
    await seek.evaluate((input: HTMLInputElement, seconds) => { input.value = String(seconds); input.dispatchEvent(new Event("input", { bubbles: true })); }, position);
    await expect(preview.locator("img")).toBeVisible();
    await page.locator(".media-stage").screenshot({ path: info.outputPath(`loaded-${viewport.width}.png`) });
    await seek.dispatchEvent("change");
    await expect.poll(() => video.evaluate((media, seconds) => !media.seeking && media.readyState >= 2 && Math.abs(media.currentTime - seconds) < .25, position)).toBeTruthy();
  }
  expect(await page.evaluate(() => (window as Window & { observedSeeks: boolean[] }).observedSeeks)).toEqual([true, true, true]);
  const accessibility = await new AxeBuilder({ page }).include(".player-control-dock").analyze();
  expect(accessibility.violations).toEqual([]);
  await writeFile(info.outputPath("preview-measurements.json"), JSON.stringify({ measurements, requests, accessibility: accessibility.violations }, null, 2));
});
