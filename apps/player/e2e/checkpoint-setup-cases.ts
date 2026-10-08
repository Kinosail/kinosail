import {expect, test, type Locator, type Page, type TestInfo} from "@playwright/test";

type PlaybackAttempt = {finished: boolean; rejected: boolean};
type PlaybackVideo = HTMLVideoElement & {checkpointPlayAttempt?: PlaybackAttempt};

export async function startPlaying(media: Locator) {
  const before = await media.evaluate((video: PlaybackVideo) => {
    const attempt = {finished: false, rejected: false};
    video.checkpointPlayAttempt = attempt;
    const before = {frames: video.getVideoPlaybackQuality().totalVideoFrames, seconds: video.currentTime};
    void video.play().catch(() => {if (!attempt.finished) attempt.rejected = true;});
    return before;
  });
  await expect.poll(() => media.evaluate((video: PlaybackVideo, before) => {
    if (video.checkpointPlayAttempt?.rejected) return "rejected";
    return !video.paused && !video.ended && video.getVideoPlaybackQuality().totalVideoFrames > before.frames + 2
      && video.currentTime > before.seconds + 0.2 ? "moving" : "pending";
  }, before)).toBe("moving");
  await media.evaluate((video: PlaybackVideo) => {
    const attempt = video.checkpointPlayAttempt;
    if (!attempt || attempt.rejected) throw new Error("Native play failed before moving media");
    attempt.finished = true;
  });
}

type Movie = {media: Locator; id: string; session?: string; paused: number; duration: number};
type State = {seconds: number; revision: number; sessionMatches?: boolean};
type Observation = {key: string; iteration: number; testInfo: TestInfo; resumeAt?: number};

export function registerCheckpointSetup(flows: {
  phase: string;
  openMovie: (page: Page, observation?: Observation) => Promise<Movie>;
  checkpoint: (page: Page, id: string, session?: string) => Promise<State>;
}) {
  test("moving checkpoint does not wait for a delayed play acknowledgement", {tag: "@smoke"}, async ({page}) => {
    await page.addInitScript(() => {
      const play = HTMLMediaElement.prototype.play;
      HTMLMediaElement.prototype.play = function() {
        return play.call(this).then(() => new Promise<void>(resolve => {
          this.addEventListener("ended", () => resolve(), {once: true});
        }));
      };
    });
    const {media, paused, duration} = await flows.openMovie(page);
    expect(paused).toBeGreaterThan(0.2);
    expect(paused).toBeLessThan(duration - 10);
    await expect(media).toHaveJSProperty("ended", false);
  });

  test("a completed moving start can resume after late native abort on pause", {tag: "@smoke"}, async ({page}) => {
    await page.addInitScript(() => {
      const play = HTMLMediaElement.prototype.play;
      HTMLMediaElement.prototype.play = function() {
        return play.call(this).then(() => new Promise<void>((_, reject) => {
          this.addEventListener("pause", () => reject(new DOMException("Intentional fixture pause", "AbortError")), {once: true});
        }));
      };
    });
    const movie = await flows.openMovie(page);
    for (let attempt = 0; attempt < 2; attempt++) {
      const before = await movie.media.evaluate((video: HTMLVideoElement) => ({frames: video.getVideoPlaybackQuality().totalVideoFrames, seconds: video.currentTime}));
      await startPlaying(movie.media);
      await expect.poll(() => movie.media.evaluate((video: HTMLVideoElement) => video.getVideoPlaybackQuality().totalVideoFrames)).toBeGreaterThan(before.frames + 2);
      await expect.poll(() => movie.media.evaluate((video: HTMLVideoElement) => video.currentTime)).toBeGreaterThan(before.seconds + 0.2);
      await movie.media.evaluate((video: HTMLVideoElement) => video.pause());
      const paused = await movie.media.evaluate((video: HTMLVideoElement) => video.currentTime);
      await expect.poll(async () => Math.abs((await flows.checkpoint(page, movie.id, movie.session)).seconds - paused)).toBeLessThan(0.1);
      await expect(movie.media).toHaveJSProperty("ended", false);
    }
  });

  test("a rejected native start cannot establish a moving checkpoint", {tag: "@smoke"}, async ({page}) => {
    await page.addInitScript(() => {
      HTMLMediaElement.prototype.play = function() {
        return Promise.reject(new DOMException("Synthetic native play denial", "NotAllowedError"));
      };
    });
    await expect(flows.openMovie(page)).rejects.toThrow();
    await expect(page.locator("video")).toHaveJSProperty("paused", true);
    await expect(page.locator("video")).toHaveJSProperty("currentTime", 0);
    const id = new URL(page.url()).pathname.split("/").at(-1)!;
    expect((await flows.checkpoint(page, id)).seconds).toBe(0);
  });

  test("checkpoint setup preserves stored resume before establishing fresh moving media", {tag: "@smoke"}, async ({page}, info) => {
    test.skip(flows.phase !== "candidate", "historical sources are reserved for original checkpoint reproductions");
    const seed = await flows.openMovie(page);
    const before = await flows.checkpoint(page, seed.id, seed.session);
    const nearEnd = seed.duration - 0.05;
    await seed.media.evaluate((video: HTMLVideoElement, seconds) => {video.pause(); video.currentTime = seconds;}, nearEnd);
    await expect.poll(() => seed.media.evaluate((video: HTMLVideoElement) => video.seeking)).toBe(false);
    await expect.poll(async () => (await flows.checkpoint(page, seed.id, seed.session)).seconds).toBeCloseTo(nearEnd, 1);
    const stored = await flows.checkpoint(page, seed.id, seed.session);
    expect(stored.sessionMatches).toBe(true);
    expect(stored.revision).toBeGreaterThan(before.revision);
    const fresh = await flows.openMovie(page, {key: "kinosail:checkpoint-near-end-setup", iteration: 1, testInfo: info, resumeAt: nearEnd});
    expect(fresh.id).toBe(seed.id);
    expect(Number(await fresh.media.getAttribute("data-start"))).toBeGreaterThanOrEqual(nearEnd - 0.1);
    expect(fresh.paused).toBeGreaterThan(0.2);
    expect(fresh.paused).toBeLessThan(5);
    await expect(fresh.media).toHaveJSProperty("ended", false);
    const accepted = await flows.checkpoint(page, fresh.id, fresh.session);
    expect(accepted.sessionMatches).toBe(true);
    expect(Math.abs(accepted.seconds - fresh.paused)).toBeLessThan(0.1);
    await info.attach("near-end-checkpoint-setup", {body: JSON.stringify({revision: process.env.KINOSAIL_TEST_REVISION,
      browser: info.project.name, nearEnd, before, stored, paused: fresh.paused, accepted,
      data: "Real Server checkpoint and decoded Checkpoint Example; stored near-end position resumes before fixture rewind", result: "passed"}), contentType: "application/json"});
  });
}
