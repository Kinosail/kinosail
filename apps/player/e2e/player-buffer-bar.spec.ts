import {readFile} from "node:fs/promises";
import {expect, Page, test} from "@playwright/test";
import {installPlayerExperienceFixture} from "./player-experience-fixture";

installPlayerExperienceFixture(false, false, "iPhone", async (page, title) => {
  if (title.includes("native HLS")) await page.locator("video").evaluate(video => {
    video.dataset.hls = "/hls/movie/p/a-a0-s0-none-t0-b0/index.m3u8";
    video.dataset.start = "40";
    let source = "";
    Object.defineProperty(video, "src", {get: () => source, set: (value: string) => { source = value; }});
    Object.defineProperty(video, "canPlayType", {value: () => "probably"});
    document.body.insertAdjacentHTML("beforeend", '<div data-quality-control hidden><select data-quality></select><span data-quality-state></span></div>');
  });
});

test.beforeEach(async ({page}) => {
  await page.addStyleTag({content: await readFile("../../../packages/webassets/static/player-stage.css", "utf8")});
  await page.clock.install();
});

async function ranges(page: Page) {
  return page.locator("[data-player-seek]").evaluate(input =>
    getComputedStyle(input).backgroundImage.split(/\), (?=linear-gradient)/)
      .filter(layer => layer.includes("0.72"))
      .map(layer => [...layer.matchAll(/([\d.]+)%/g)].map(match => Number(match[1])))
      .map(stops => [stops[0], stops[2]]));
}

async function setRanges(page: Page, values: number[][], event = "progress") {
  await page.locator("video").evaluate((video, {values, event}) => {
    Object.defineProperty(video, "buffered", {configurable: true, value: {
      length: values.length, start: (index: number) => values[index][0], end: (index: number) => values[index][1],
    }});
    video.dispatchEvent(new Event(event));
  }, {values, event});
}

test("buffered ranges update behind played progress and preserve seek gaps @smoke", async ({page}, testInfo) => {
  const seek = page.getByRole("slider", {name: "Seek", exact: true});
  await expect.poll(() => ranges(page)).toEqual([[0, 60]]);
  await expect(seek).toHaveAttribute("aria-valuetext", "0:20 of 1:40");
  await setRanges(page, [[0, 30], [50, 80]]);
  await expect.poll(() => ranges(page)).toEqual([[0, 30], [50, 80]]);
  await seek.fill("55");
  await expect(seek).toHaveAttribute("aria-valuetext", "0:55 of 1:40");
  await expect.poll(() => ranges(page)).toEqual([[0, 30], [50, 80]]);
  await setRanges(page, [[50, 90]], "timeupdate");
  await expect.poll(() => ranges(page)).toEqual([[50, 90]]);
  await seek.focus();
  await page.keyboard.press("ArrowRight");
  await expect(seek).toHaveValue("56");
  await expect.poll(() => page.locator("video").evaluate(video => video.currentTime)).toBe(56);
  for (const width of [390, 1440, 1920]) {
    await page.setViewportSize({width, height: width === 390 ? 844 : 1080});
    const box = await seek.boundingBox();
    expect(box!.height).toBeGreaterThanOrEqual(44);
    expect(box!.x).toBeGreaterThanOrEqual(0);
    expect(box!.x + box!.width).toBeLessThanOrEqual(width);
    await page.screenshot({path: testInfo.outputPath(`buffered-${width}.png`)});
  }
});

test("native HLS buffered ranges use the full video timeline", async ({page}) => {
  await setRanges(page, [[0, 10], [20, 30]]);
  await expect.poll(() => ranges(page)).toEqual([[40, 50], [60, 70]]);
});

test("buffered ranges clear while loading, when empty, and after failure", async ({page}, testInfo) => {
  await expect.poll(() => ranges(page)).toEqual([[0, 60]]);
  await page.locator("video").dispatchEvent("loadstart");
  await expect.poll(() => ranges(page)).toEqual([]);
  await page.screenshot({path: testInfo.outputPath("pending.png")});
  await setRanges(page, [[10, 40]]);
  await expect.poll(() => ranges(page)).toEqual([[10, 40]]);
  await setRanges(page, []);
  await expect.poll(() => ranges(page)).toEqual([]);
  await page.screenshot({path: testInfo.outputPath("empty.png")});
  await setRanges(page, [[0, 100]], "durationchange");
  await expect.poll(() => ranges(page)).toEqual([[0, 100]]);
  await page.locator("video").dispatchEvent("emptied");
  await expect.poll(() => ranges(page)).toEqual([]);
  await setRanges(page, [[20, 70]]);
  await page.locator("video").dispatchEvent("error");
  await expect.poll(() => ranges(page)).toEqual([]);
  await page.screenshot({path: testInfo.outputPath("failed.png")});
});

for (const [name, duration] of [["missing", ""], ["malformed", "unknown"], ["negative", "-1"], ["nonfinite", "Infinity"], ["zero", "0"]]) {
  test(`buffered ranges stay empty for a ${name} duration`, async ({page}) => {
    await page.locator("video").evaluate((video, duration) => {
      Object.defineProperty(video, "duration", {value: NaN});
      video.dataset.duration = duration;
    }, duration);
    await setRanges(page, [[0, 60]]);
    await expect.poll(() => ranges(page)).toEqual([]);
    await expect(page.locator("video")).toHaveJSProperty("currentTime", 20);
    await expect(page.locator("video")).toHaveJSProperty("paused", true);
  });
}

test("invalid buffer ranges do not alter playback and valid ranges stay within the timeline", async ({page}) => {
  await setRanges(page, [[-20, 20], [90, 200], [30, 20], [110, 120], [NaN, 40], [40, Infinity]]);
  await expect.poll(() => ranges(page)).toEqual([[0, 20], [90, 100]]);
  await expect(page.locator("video")).toHaveJSProperty("currentTime", 20);
  await expect(page.locator("video")).toHaveJSProperty("paused", true);
});
