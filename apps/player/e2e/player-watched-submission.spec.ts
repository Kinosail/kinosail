import {expect, test} from "@playwright/test";
import {readFile} from "node:fs/promises";
import {progressFixtureHTML} from "./player-progress-fixture";

const source = (await Promise.all(["player-progress.js", "player-progress-navigation.js"].map(name =>
  readFile(new URL(`../../../packages/webassets/static/${name}`, import.meta.url), "utf8")))).join("\n");
const origin = "https://progress.kinosail.test";

for (const playedBefore of [false, true]) {
for (const mode of ["cancelled", "late-cancelled", "form-target", "form-method", "form-action", "submitter-target", "submitter-method", "submitter-action", "submitter-invalid-action"]) {
  test(`${mode} watched submission ${playedBefore ? "after" : "before"} playing preserves subsequent position saves`, {tag: "@smoke"}, async ({page}, info) => {
    const positions: string[] = [];
    const errors: string[] = [];
    page.on("pageerror", error => errors.push(error.name));
    await page.route(`${origin}/`, route => route.fulfill({contentType: "text/html", body: progressFixtureHTML}));
    await page.route(`${origin}/progress/**`, async route => {
      positions.push(route.request().postData()!);
      await route.fulfill({status: 204});
    });
    await page.goto(origin);
    await page.addScriptTag({content: `
      const player = document.querySelector('video'), csrf = 'synthetic-csrf', playbackSession = 'qa-watched-boundary';
      let managedSeek = false, playbackPreparation, preparationSeek, preparationPausePending = 0, playbackRequest = 0;
      let playbackTraceMethod = 'direct', playbackTimelineOffset = 0;
      const isPictureInPicture = () => false, requestPlay = async () => {};
      const requestPause = () => {playbackRequest++; paused = true; player.dispatchEvent(new Event('pause'));};
      const playbackTrace = () => {}, flushPlaybackTrace = () => {}, playerStorage = {get: () => '', set: () => {}};
      const setPlayerTime = seconds => player.currentTime = seconds;
      let paused = false, position = 42;
      Object.defineProperties(player, {currentTime: {get: () => position}, duration: {value: 100}, readyState: {value: 4},
        paused: {get: () => paused}, ended: {value: false}});
      window.pauseFixture = seconds => {position = seconds; paused = true; player.dispatchEvent(new Event('pause'));};
      window.watchedSubmissionReplays = 0;
    ` + source});
    await page.evaluate(({mode, playedBefore}) => {
      const form = document.createElement("form");
      form.action = "/watched/movie"; form.method = "post";
      const button = document.createElement("button"); button.name = "watched"; button.value = "true";
      form.append(button); document.body.append(form);
      if (mode === "form-target") form.target = "_blank";
      if (mode === "form-method") form.method = "get";
      if (mode === "form-action") form.action = "https://other.kinosail.test/watched/movie";
      if (mode === "submitter-target") button.formTarget = "_blank";
      if (mode === "submitter-method") button.formMethod = "get";
      if (mode === "submitter-action") button.formAction = "https://other.kinosail.test/watched/movie";
      if (mode === "submitter-invalid-action") button.setAttribute("formaction", "http://[");
      if (mode === "cancelled") document.addEventListener("submit", event => event.preventDefault(), {capture: true});
      if (mode === "late-cancelled") document.addEventListener("submit", event => event.preventDefault());
      const control = window as Window & {pauseFixture(seconds: number): void; watchedSubmissionReplays: number};
      form.addEventListener("submit", () => {control.watchedSubmissionReplays++;});
      if (playedBefore) document.querySelector("video")!.dispatchEvent(new Event("playing"));
      // Exercise delivered submit listeners without an unrelated browser form
      // navigation. A checkpoint replay still invokes the real requestSubmit.
      const notification = new SubmitEvent("submit", {bubbles: true, cancelable: true, submitter: button});
      Object.defineProperty(notification, "target", {value: form});
      document.dispatchEvent(notification);
      if (!playedBefore) document.querySelector("video")!.dispatchEvent(new Event("playing"));
      control.pauseFixture(42);
    }, {mode, playedBefore});
    await expect.poll(() => positions.length).toBeGreaterThan(0);
    await page.waitForTimeout(150);
    await page.evaluate(() => (window as Window & {pauseFixture(seconds: number): void}).pauseFixture(60));
    await expect.poll(() => new URLSearchParams(positions.at(-1)).get("seconds")).toBe("60");
    expect(new URLSearchParams(positions.at(-1)).get("watched")).toBe("false");
    const replayed = await page.evaluate(() => (window as Window & {watchedSubmissionReplays: number}).watchedSubmissionReplays);
    if (mode === "late-cancelled" && playedBefore) expect(replayed).toBe(1);
    else expect(replayed).toBe(0);
    expect(errors).toEqual([]);
    await info.attach("watched-submission-boundary", {body: JSON.stringify({mode, playedBefore, replayed, positions,
      data: "Isolated DOM ordering with synthetic media state; delivered production progress and navigation scripts"}), contentType: "application/json"});
  });
}
}
