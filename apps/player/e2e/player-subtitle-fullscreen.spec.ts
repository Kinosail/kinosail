import {expect, test} from "@playwright/test";
import {installPlayerExperienceFixture} from "./player-experience-fixture";

installPlayerExperienceFixture();

test("limited native fullscreen fallback cannot expose embedded subtitle choices", async ({ page }, testInfo) => {
  const fullscreen = page.locator("[data-player-fullscreen]");
  await expect(fullscreen).toBeDisabled();
  await expect(fullscreen).toBeHidden();
  await fullscreen.evaluate((button) => button.dispatchEvent(new Event("click")));
  expect(await page.evaluate(() => (window as Window & {nativeFullscreenCalls?: number}).nativeFullscreenCalls || 0)).toBe(0);
  await expect(page.getByRole("button", { name: "Theater" })).toBeEnabled();
  await page.getByRole("button", { name: "Play" }).first().click();
  await expect(page.locator("[data-player-status]")).toBeHidden();
  for (const width of [390, 1440]) {
    await page.setViewportSize({width, height: 844});
    await page.screenshot({path: testInfo.outputPath(`limited-subtitles-${width}.png`)});
  }
});

test("native fullscreen fallback stays available with all subtitle choices", async ({ page }) => {
  const fullscreen = page.getByRole("button", { name: "Enter fullscreen" });
  await expect(fullscreen).toBeEnabled();
  await fullscreen.click();
  expect(await page.evaluate(() => (window as Window & {nativeFullscreenCalls?: number}).nativeFullscreenCalls)).toBe(1);
});
