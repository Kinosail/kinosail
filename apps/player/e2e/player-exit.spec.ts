import {readFile} from "node:fs/promises";
import {expect, test} from "@playwright/test";
import {installPlayerExperienceFixture} from "./player-experience-fixture";

installPlayerExperienceFixture();
test.use({hasTouch: true, reducedMotion: "reduce"});
test.beforeEach(async ({page}) => {
  await page.addStyleTag({content: (await Promise.all([
    "../../../packages/webassets/static/player-stage.css",
    "../../../packages/webassets/static/last-light.css",
    "../internal/server/static/home.css",
  ].map(path => readFile(path, "utf8")))).join("\n")});
  await page.locator("video").dispatchEvent("canplay");
  await page.clock.install();
});

for (const viewport of [{width: 390, height: 844}, {width: 844, height: 390}, {width: 1440, height: 900}, {width: 1920, height: 1080}]) {
  test(`theater exit remains available during idle playback at ${viewport.width}x${viewport.height} @smoke`, async ({page}, testInfo) => {
    await page.setViewportSize(viewport);
    const exit = page.locator("[data-player-exit]");
    await expect(exit).toBeHidden();
    await page.getByRole("button", {name: "Theater", exact: true}).tap();
    await page.locator("video").evaluate(video => video.play());
    await page.clock.fastForward(3000);
    await expect(page.locator("[data-player-controls]")).toBeHidden();
    await expect(exit).toBeVisible();
    await expect(exit).toHaveAccessibleName("Exit theater");
    const box = await exit.boundingBox();
    // Firefox can report 44px as 43.999999px.
    expect(box!.width + .001).toBeGreaterThanOrEqual(44);
    expect(box!.height + .001).toBeGreaterThanOrEqual(44);
    expect(await exit.evaluate(button => {
      const box = button.getBoundingClientRect();
      return button.contains(document.elementFromPoint(box.x + box.width / 2, box.y + box.height / 2));
    })).toBe(true);
    await page.screenshot({path: testInfo.outputPath("idle-theater-exit.png")});
    await exit.tap();
    await expect(page.locator("body")).not.toHaveClass(/player-theater/);
    await expect(exit).toBeHidden();
    await expect(page.locator("video")).toHaveJSProperty("paused", false);
    await expect(page.locator("video")).toHaveJSProperty("currentTime", 20);
    await expect(page.locator("[data-theater]")).toBeFocused();
    await expect(page.locator(".chapters")).not.toHaveJSProperty("inert", true);
  });
}

for (const state of ["pending", "paused", "ended", "failed"]) test(`theater exit is available while ${state}`, async ({page}, testInfo) => {
  const video = page.locator("video");
  const exit = page.locator("[data-player-exit]");
  await page.getByRole("button", {name: "Theater", exact: true}).click();
  if (state === "pending") {
    await page.evaluate(() => (window as Window & {setReadyState: (value: number) => void}).setReadyState(1));
    await video.dispatchEvent("loadstart");
    await expect(page.locator("[data-player-status]")).toBeVisible();
  } else if (state === "failed") {
    await video.evaluate(media => Object.defineProperty(media, "error", {value: {code: 0}}));
    await video.dispatchEvent("error");
    await expect(page.getByRole("button", {name: "Retry Direct Play"})).toBeVisible();
  } else await video.dispatchEvent(state === "ended" ? "ended" : "pause");
  await expect(exit).toBeVisible();
  await page.screenshot({path: testInfo.outputPath(`${state}-theater-exit.png`)});
  await exit.click();
  await expect(page.locator("body")).not.toHaveClass(/player-theater/);
});

test("keyboard can reach the persistent exit after the transport fades", async ({page}) => {
  await page.getByRole("button", {name: "Theater", exact: true}).click();
  await page.locator("video").evaluate(video => video.play());
  await page.clock.fastForward(3000);
  await expect(page.locator("[data-player-controls]")).toBeHidden();
  await page.keyboard.press("Tab");
  await expect(page.locator("[data-player-exit]")).toBeFocused();
  await page.keyboard.press("Enter");
  await expect(page.locator("body")).not.toHaveClass(/player-theater/);
});
