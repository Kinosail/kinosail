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
  await page.evaluate(() => document.documentElement.dataset.theme = "dark");
  await page.locator("video").dispatchEvent("canplay");
  await page.clock.install();
});

for (const viewport of [{width: 320, height: 568}, {width: 390, height: 844}, {width: 430, height: 932}, {width: 844, height: 390}]) {
  test(`custom transport targets do not overlap the timeline at ${viewport.width}x${viewport.height}`, async ({page}, testInfo) => {
    await page.setViewportSize(viewport);
    const play = page.locator(".player-center-control[data-player-toggle]");
    const targets = page.locator(".player-center-control");
    for (const theater of [false, true]) {
      if (theater) await page.getByRole("button", {name: "Theater", exact: true}).tap();
      const occluded = await targets.evaluateAll(buttons => buttons.flatMap(button => {
        const box = button.getBoundingClientRect();
        const points = [[.5, .1], [.1, .5], [.5, .5], [.9, .5], [.5, .9]];
        return points.filter(([x, y]) => !button.contains(document.elementFromPoint(box.x + box.width * x, box.y + box.height * y)))
          .map(point => ({label: button.getAttribute("aria-label"), point}));
      }));
      expect(occluded).toEqual([]);
      const box = await play.boundingBox();
      await page.touchscreen.tap(box!.x + box!.width / 2, box!.y + box!.height * .9);
      await expect(page.locator("video")).toHaveJSProperty("paused", false);
      await page.touchscreen.tap(box!.x + box!.width / 2, box!.y + box!.height * .1);
      await expect(page.locator("video")).toHaveJSProperty("paused", true);
      await expect(page.locator("video")).toHaveJSProperty("currentTime", 20);
      await page.screenshot({path: testInfo.outputPath(`${theater ? "theater" : "inline"}-${viewport.width}.png`)});
    }
  });
}

for (const input of ["touch", "mouse"]) test(`custom controls hide after ${input} Play without requiring blur @smoke`, async ({page}) => {
  await page.setViewportSize({width: 390, height: 844});
  const play = page.locator(".player-center-control[data-player-toggle]");
  if (input === "touch") await play.tap();
  else await play.click();
  await expect(page.locator("video")).toHaveJSProperty("paused", false);
  await page.clock.fastForward(2600);
  await expect(page.locator("[data-player-controls]")).toBeHidden();
});

test("hidden controls cannot seek or pause when a touch reveals them", async ({page}) => {
  await page.setViewportSize({width: 390, height: 844});
  const seekBox = await page.locator("[data-player-seek]").boundingBox();
  await page.locator("video").evaluate(video => video.play());
  await page.clock.fastForward(2600);
  const point = {x: seekBox!.x + seekBox!.width * .8, y: seekBox!.y + seekBox!.height / 2};
  expect(await page.evaluate(({x, y}) => !!document.elementFromPoint(x, y)?.closest("[data-player-controls]"), point)).toBe(false);
  await page.touchscreen.tap(point.x, point.y);
  await expect(page.locator("video")).toHaveJSProperty("paused", false);
  await expect(page.locator("video")).toHaveJSProperty("currentTime", 20);
  await expect(page.locator("[data-player-controls]")).toBeVisible();
  await page.locator(".player-center-control[data-player-toggle]").tap();
  await expect(page.locator("video")).toHaveJSProperty("paused", true);
  await expect(page.locator("video")).toHaveJSProperty("currentTime", 20);
});

test("touch theater controls hide after settings close and reappear on pause", async ({page}, testInfo) => {
  await page.setViewportSize({width: 844, height: 390});
  await page.getByRole("button", {name: "Theater", exact: true}).tap();
  await page.locator(".player-center-control[data-player-toggle]").tap();
  await page.getByRole("button", {name: "Settings", exact: true}).tap();
  await page.clock.fastForward(3000);
  await expect(page.locator(".player-settings")).toBeVisible();
  await page.getByRole("button", {name: "Close", exact: true}).tap();
  await page.clock.fastForward(2600);
  await expect(page.locator("[data-player-controls]")).toBeHidden();
  await page.screenshot({path: testInfo.outputPath("landscape-playing-controls-hidden.png")});
  await page.locator("video").evaluate(video => video.pause());
  await expect(page.locator("[data-player-controls]")).toBeVisible();
});

test("keyboard focus keeps controls available until focus leaves the player", async ({page}) => {
  await page.locator("video").evaluate(video => video.play());
  await page.keyboard.press("Tab");
  await page.locator(".player-center-control[data-player-toggle]").focus();
  await expect(page.locator(".player-center-control[data-player-toggle]")).toBeFocused();
  await page.clock.fastForward(3000);
  await expect(page.locator("[data-player-controls]")).toBeVisible();
  await page.evaluate(() => {
    document.body.insertAdjacentHTML("beforeend", '<button id="outside-player">Outside player</button>');
    document.querySelector<HTMLButtonElement>("#outside-player")!.focus();
  });
  await page.clock.fastForward(2600);
  await expect(page.locator("[data-player-controls]")).toBeHidden();
});
