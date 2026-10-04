import { expect, test } from "@playwright/test";
import { captionPeer, isolated, openCaptionPlayer, serverOrigin } from "./player-subtitles-recovery-fixture";

test.skip(!serverOrigin && !isolated, "requires the opt-in disposable Server runner or explicit isolated transport control");
test.use({ serviceWorkers: "block" });

for (const stalledBoundary of ["headers", "body"] as const) {
  test(`${serverOrigin ? "real Server" : "isolated transport"}: stalled caption ${stalledBoundary} reaches a deadline and keyboard Retry`, async ({ page }, info) => {
    const peer = await captionPeer(page, stalledBoundary);
    try {
      await openCaptionPlayer(page, peer.origin);
      await expect(page.locator("[data-subtitle-status]")).toHaveText("Loading subtitles…");
      await expect.poll(async () => (await peer.stats()).calls).toBe(1);
      const before = await page.locator("video").evaluate((video: HTMLVideoElement) => ({source: video.getAttribute("src"), time: video.currentTime, paused: video.paused}));
      await page.clock.fastForward(20_100);
      await page.screenshot({ path: info.outputPath(`${stalledBoundary}-after-deadline.png`) });
      await expect(page.locator("[data-subtitle-status]")).toContainText("Subtitles unavailable", { timeout: 2_000 });
      await expect.poll(async () => (await peer.stats()).closed).toBe(1);
      const retry = page.getByRole("button", { name: "Retry subtitles" });
      await expect(retry).toBeVisible();
      await retry.focus();
      await page.keyboard.press("Enter");
      await expect.poll(() => page.locator('track[label="English"]').evaluate((track: HTMLTrackElement) => (track.track.cues?.[0] as VTTCue | undefined)?.text)).toBe("Recovered captions");
      await expect(page.locator("[data-subtitle-status]")).toBeHidden();
      await expect(retry).toBeHidden();
      await expect(page.locator("[data-subtitles]")).toHaveValue("0");
      expect(await page.locator('track[label="English"]').evaluate((track: HTMLTrackElement) => track.track.mode)).toBe("showing");
      const after = await page.locator("video").evaluate((video: HTMLVideoElement) => ({source: video.getAttribute("src"), time: video.currentTime, paused: video.paused}));
      expect(after.source).toBe(before.source);
      expect(after.paused).toBe(before.paused);
      if (serverOrigin) {
        await expect.poll(() => page.locator("video").evaluate((video: HTMLVideoElement) => video.currentTime)).toBeGreaterThan(before.time + 0.1);
        await page.screenshot({ path: info.outputPath(`${stalledBoundary}-recovered.png`), fullPage: true });
      } else expect(after.time).toBe(before.time);
      expect((await peer.stats()).calls).toBe(2);
    } finally {
      await peer.close();
    }
  });
}
