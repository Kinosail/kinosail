import { expect, test } from "@playwright/test";
import { captionPeer, isolated, openCaptionPlayer, serverOrigin } from "./player-subtitles-recovery-fixture";

test.use({ serviceWorkers: "block" });

for (const preference of ["default", "off"] as const) {
  test(`media metadata preserves ${preference} captions after native track initialization @smoke`, async ({page}) => {
    const peer = await captionPeer(page, "headers");
    try {
      await openCaptionPlayer(page, peer.origin);
      await expect(page.locator("[data-subtitle-status]")).toHaveText("Loading subtitles…");
      await expect.poll(async () => (await peer.stats()).calls).toBe(1);
      if (preference === "off") {
        await page.locator("track").evaluate((track: HTMLTrackElement) => { track.track.mode = "disabled"; });
        await page.locator("[data-subtitles]").selectOption("off");
      }
      // WebKit's initial media selection can disable an external default track.
      // Exercise that native transition without feeding captions to the decoder.
      await page.locator("video").evaluate((video: HTMLVideoElement) => {
        video.textTracks[0].mode = "disabled";
        video.dispatchEvent(new Event("loadedmetadata"));
      });
      await expect.poll(() => page.locator("track").evaluate((track: HTMLTrackElement) => track.track.mode))
        .toBe(preference === "default" ? "showing" : "disabled");
      if (preference === "default") await expect(page.locator("[data-subtitle-status]")).toHaveText("Loading subtitles…");
      else await expect(page.locator("[data-subtitle-status]")).toBeHidden();
      await expect(page.locator("track")).not.toHaveAttribute("src", /.+/);
      expect((await peer.stats()).calls).toBe(1);
    } finally { await peer.close(); }
  });
}

test("later metadata preserves a native Off caption choice @smoke", async ({page}) => {
  const peer = await captionPeer(page, "headers");
  try {
    await openCaptionPlayer(page, peer.origin);
    await expect.poll(async () => (await peer.stats()).calls).toBe(1);
    await page.locator("video").evaluate((video: HTMLVideoElement) => {
      video.dispatchEvent(new Event("loadedmetadata"));
      // Native media controls change TextTrack mode without the app selector.
      video.textTracks[0].mode = "disabled";
      video.dispatchEvent(new Event("loadedmetadata"));
    });
    await expect.poll(() => page.locator("track").evaluate((track: HTMLTrackElement) => track.track.mode)).toBe("disabled");
    await expect(page.locator("[data-subtitle-status]")).toBeHidden();
    expect((await peer.stats()).calls).toBe(1);
  } finally { await peer.close(); }
});

test("persisted caption restore cancels the old attempt and reloads the selected language", async ({page}, info) => {
  test.skip(!isolated || Boolean(serverOrigin), "isolated native PageTransitionEvent control for persisted caption lifecycle");
  const peer = await captionPeer(page, "body");
  try {
    await openCaptionPlayer(page, peer.origin);
    await expect(page.locator("[data-subtitle-status]")).toHaveText("Loading subtitles…");
    await expect.poll(async () => (await peer.stats()).calls).toBe(1);
    await page.evaluate(() => dispatchEvent(new PageTransitionEvent("pagehide", {persisted: true})));
    await expect.poll(async () => (await peer.stats()).closed).toBe(1);
    await expect(page.locator("track")).not.toHaveAttribute("src", /.+/);
    await page.evaluate(() => dispatchEvent(new PageTransitionEvent("pageshow", {persisted: true})));
    await info.attach("transport-after-persisted-restore", {body: JSON.stringify(await peer.stats()), contentType: "application/json"});
    await expect.poll(() => page.locator('track[srclang="en"]').evaluate((track: HTMLTrackElement) => (track.track.cues?.[0] as VTTCue | undefined)?.text), {timeout: 2_000}).toBe("Recovered captions");
    await expect(page.locator("[data-subtitle-status]")).toBeHidden();
    await expect(page.locator("[data-subtitle-retry]")).toBeHidden();
    expect(await page.locator("track").evaluate((track: HTMLTrackElement) => track.track.mode)).toBe("showing");
    expect((await peer.stats()).calls).toBe(2);
    await page.clock.fastForward(25_000);
    await expect(page.locator("[data-subtitle-status]")).toBeHidden();
    await page.screenshot({path: info.outputPath("persisted-selected-recovered.png"), fullPage: true});
  } finally {
    await peer.close();
  }
});

test("persisted caption restore keeps Off without starting another request", async ({page}, info) => {
  test.skip(!isolated || Boolean(serverOrigin), "isolated native PageTransitionEvent control for persisted caption lifecycle");
  const peer = await captionPeer(page, "headers");
  try {
    await openCaptionPlayer(page, peer.origin);
    await expect(page.locator("[data-subtitle-status]")).toHaveText("Loading subtitles…");
    await expect.poll(async () => (await peer.stats()).calls).toBe(1);
    // Isolated shell has no settings adapter; operate the native track preference.
    await page.locator("track").evaluate((track: HTMLTrackElement) => { track.track.mode = "disabled"; });
    await page.locator("[data-subtitles]").selectOption("off");
    await expect(page.locator("[data-subtitle-status]")).toBeHidden();
    await page.evaluate(() => dispatchEvent(new PageTransitionEvent("pagehide", {persisted: true})));
    await expect.poll(async () => (await peer.stats()).closed).toBe(1);
    await page.evaluate(() => dispatchEvent(new PageTransitionEvent("pageshow", {persisted: true})));
    await page.clock.fastForward(25_000);
    await expect(page.locator("[data-subtitle-status]")).toBeHidden();
    await expect(page.locator("[data-subtitle-retry]")).toBeHidden();
    await expect(page.locator("[data-subtitles]")).toHaveValue("off");
    expect(await page.locator("track").evaluate((track: HTMLTrackElement) => track.track.mode)).toBe("disabled");
    await expect(page.locator("track")).not.toHaveAttribute("src", /.+/);
    expect((await peer.stats()).calls).toBe(1);
    await page.screenshot({path: info.outputPath("persisted-off.png"), fullPage: true});
  } finally {
    await peer.close();
  }
});
