import { expect, test } from "@playwright/test";
import { installPlayerExperienceFixture } from "./player-experience-fixture";

installPlayerExperienceFixture();

for (const pause of [false, true]) test(`offline source swap honors intent during active playback handoff: pause=${pause}`, async ({page}) => {
  const video = page.locator("video");
  await page.evaluate(() => (window as Window & {resolveOfflineSource: () => void}).resolveOfflineSource());
  await expect(video).toHaveJSProperty("src", "/offline-media/profile/movie-job");
  // Loading the new source pauses the old element; this is not a viewer command.
  await video.dispatchEvent("pause");
  if (pause) await video.evaluate(media => media.dispatchEvent(new CustomEvent("kinosail:playback-intent", {detail: {playing: false}})));
  await video.dispatchEvent("loadedmetadata");
  await video.dispatchEvent("loadeddata");
  await expect(video).toHaveJSProperty("paused", pause);
});
