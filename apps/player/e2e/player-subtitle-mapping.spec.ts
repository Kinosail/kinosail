import {expect, test} from "@playwright/test";
import {installPlayerExperienceFixture} from "./player-experience-fixture";

installPlayerExperienceFixture();

test("limited in-band tracks never replace the selected English subtitle", async ({page}) => {
  const select = page.locator("[data-subtitles]");
  const video = page.locator("video");
  await expect(select.locator("option")).toHaveText(["Off", "English"]);
  await video.evaluate((element) => { element.textTracks[0].mode = "showing"; });
  await expect.poll(() => video.evaluate((element) => element.textTracks[0].mode)).toBe("disabled");
  await page.getByRole("button", {name: "Settings"}).click();
  await select.selectOption("0");
  expect(await video.evaluate((element) => [...element.textTracks].map((track) => track.mode))).toEqual(["disabled", "showing"]);
  await video.evaluate((element) => { element.textTracks[0].mode = "showing"; });
  await expect.poll(() => video.evaluate((element) => [...element.textTracks].map((track) => track.mode))).toEqual(["disabled", "showing"]);
  await page.getByRole("button", {name: "Subtitles"}).click();
  await expect(select).toHaveValue("off");
  await page.getByRole("button", {name: "Subtitles"}).click();
  await expect(select).toHaveValue("0");
  expect(await video.evaluate((element) => [...element.textTracks].map((track) => track.mode))).toEqual(["disabled", "showing"]);
});
