import {expect, test} from "@playwright/test";
import {installPlayerExperienceFixture} from "./player-experience-fixture";

installPlayerExperienceFixture();

for (const prefix of ["", "WebKit "]) test(`${prefix}Picture-in-Picture retains its session until playback departs @smoke`, async ({page}, info) => {
  const ended: unknown[] = [];
  page.on("request", request => {
    if (request.url().includes("/playback-events") && request.postDataJSON().event === "session-end") ended.push(request.postDataJSON());
  });
  const video = page.locator("video");
  const pip = page.locator("[data-player-pip]");
  await pip.click();
  await expect(pip).toHaveAttribute("aria-pressed", "true");
  await video.evaluate((media: HTMLVideoElement) => media.play());
  const progress = page.waitForResponse(response => response.url().includes("/playback-events") && response.request().postDataJSON().event === "progress");
  await video.dispatchEvent("progress");
  await page.evaluate(() => dispatchEvent(new Event("pagehide")));
  await progress;
  await expect(video).toHaveJSProperty("paused", false);
  expect(ended).toEqual([]);

  await page.evaluate(async () => {
    const media = document.querySelector("video") as HTMLVideoElement & {webkitPresentationMode?: string; webkitSetPresentationMode?: (mode: string) => void};
    if (media.webkitPresentationMode === "picture-in-picture") media.webkitSetPresentationMode?.("inline");
    else await document.exitPictureInPicture();
  });
  await expect(pip).toHaveAttribute("aria-pressed", "false");

  const departure = page.waitForResponse(response => response.url().includes("/playback-events") && response.request().postDataJSON().event === "session-end");
  await page.evaluate(() => dispatchEvent(new Event("pagehide")));
  await departure;
  expect(ended).toHaveLength(1);
  await expect(video).toHaveJSProperty("paused", true);
  await info.attach("pip-session-receipt.json", {contentType: "application/json", body: Buffer.from(JSON.stringify({
    revision: process.env.KINOSAIL_TEST_REVISION,
    command: "playwright test player-pip-session.spec.ts --workers=1",
    environment: info.project.name,
    data: `${prefix || "Standard "}Picture-in-Picture API simulation with recorded HTTP trace requests`,
    result: "passed",
    boundary: "Does not prove physical Picture-in-Picture or HLS decoding",
  }, null, 2))});
});
