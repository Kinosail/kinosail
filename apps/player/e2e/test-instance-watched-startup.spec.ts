import {expect, test} from "@playwright/test";
import {configureTestInstance, firstPlayable, login} from "./test-instance-helpers";
import {startPlaying} from "./checkpoint-setup-cases";

configureTestInstance();
test.use({serviceWorkers: "block"});

for (const watched of [true, false]) {
for (const cancelledAgain of [false, true]) {
test(`accepted Mark ${watched ? "watched" : "unwatched"} survives late native media callbacks from the departing page${cancelledAgain ? " after another submission is cancelled" : ""}`, {tag: "@smoke"}, async ({page}, info) => {
  await page.addInitScript(() => {
    const add = EventTarget.prototype.addEventListener;
    const pending: Array<() => void> = [];
    const control = {pending, release() {for (const notify of pending.splice(0)) notify();}};
    Object.assign(window, {watchedStartup: control});
    EventTarget.prototype.addEventListener = function(type, listener, options) {
      if (this instanceof HTMLVideoElement && type === "playing" && listener) {
        const target = this;
        return add.call(this, type, event => pending.push(() => {
          if (typeof listener === "function") listener.call(target, event);
          else listener.handleEvent(event);
        }), options);
      }
      return add.call(this, type, listener, options);
    };
  });
  await login(page);
  const watch = await firstPlayable(page), id = watch.split("/").at(-1)!;
  await page.goto(watch);
  if (await page.getByRole("button", {name: "Mark unwatched", exact: true}).isVisible()) {
    await page.getByRole("button", {name: "Mark unwatched", exact: true}).click();
    await expect(page.getByRole("button", {name: "Mark watched", exact: true})).toBeVisible();
  }
  if (!watched) {
    await page.getByRole("button", {name: "Mark watched", exact: true}).click();
    await expect(page.getByRole("button", {name: "Mark unwatched", exact: true})).toBeVisible();
  }
  const media = page.locator("video");
  await media.evaluate((video: HTMLVideoElement) => {video.muted = true;});
  await startPlaying(media);
  await expect.poll(() => page.evaluate(() => (window as unknown as {watchedStartup: {pending: unknown[]}}).watchedStartup.pending.length)).toBeGreaterThan(0);
  const progress = async () => {
    const response = await page.request.get(`/api/v1/items/${id}`);
    expect(response.status()).toBe(200);
    const stored = (await response.json()).item.progress;
    return {...stored, seconds: stored.seconds ?? 0, watched: stored.watched ?? false};
  };
  let release!: () => void, committed = false;
  const pending = new Promise<void>(resolve => {release = resolve;});
  const latePositions: string[] = [];
  const origin = new URL(page.url()).origin;
  const relayReady = page.waitForEvent("popup");
  await page.evaluate(() => Object.assign(window, {qaWatchedRelay: window.open("about:blank", "qa-watched-relay")}));
  const relay = await relayReady;
  await relay.evaluate(origin => {
    addEventListener("message", event => {
      if (event.source === opener && event.origin === origin && event.data?.kind === "qa-watched-paused") Object.assign(window, {qaWatchedObserved: event.data});
    });
  }, origin);
  await page.evaluate(({watched, cancelledAgain}) => {
    const relay = (window as unknown as {qaWatchedRelay: Window}).qaWatchedRelay;
    addEventListener("message", event => {
      if (event.source !== relay || event.origin !== location.origin || event.data !== "qa-watched-commit") return;
      const video = document.querySelector("video")!;
      if (cancelledAgain) {
        const form = document.querySelector<HTMLFormElement>('form[action^="/watched/"]')!;
        document.addEventListener("submit", event => event.preventDefault(), {capture: true, once: true});
        if (form.dispatchEvent(new SubmitEvent("submit", {bubbles: true, cancelable: true, submitter: form.querySelector("button")}))) throw new Error("Second submission was not cancelled");
      }
      (window as unknown as {watchedStartup: {release(): void}}).watchedStartup.release();
      const report = (event: Event) => relay.postMessage({kind: "qa-watched-paused", event: event.type, paused: video.paused,
        seconds: video.currentTime, frames: video.getVideoPlaybackQuality().totalVideoFrames}, location.origin);
      if (watched) {video.addEventListener("pause", report, {once: true}); return video.pause();}
      video.addEventListener("ended", report, {once: true});
      video.currentTime = video.duration - 0.1;
      void video.play().catch(() => {});
    });
  }, {watched, cancelledAgain});
  page.on("request", request => {
    if (committed && new URL(request.url()).pathname === `/progress/${id}`) latePositions.push(request.postData() || "");
  });
  await page.route(`**/watched/${id}`, async route => {
    // Playwright's API replay omits the native transport's Sec-Fetch-Site.
    // Retain the real form, cookies and CSRF while replaying this same-origin POST.
    const response = await route.fetch({maxRedirects: 0, headers: {...await route.request().allHeaders(), "Sec-Fetch-Site": "same-origin"}});
    expect(response.status()).toBe(303);
    committed = true;
    await pending;
    await route.fulfill({response});
  });
  try {
    await page.getByRole("button", {name: `Mark ${watched ? "watched" : "unwatched"}`, exact: true}).click({noWaitAfter: true});
    await expect.poll(() => committed).toBe(true);
    expect(await progress()).toMatchObject({watched, seconds: 0});
    await relay.evaluate(origin => opener!.postMessage("qa-watched-commit", origin), origin);
    const observation = () => relay.evaluate(() => (window as unknown as {qaWatchedObserved?: {event: string; paused: boolean; seconds: number; frames: number}}).qaWatchedObserved);
    await expect.poll(() => observation().then(value => value?.paused)).toBe(true);
    const paused = (await observation())!;
    expect(paused.event).toBe(watched ? "pause" : "ended");
    expect(paused!.seconds).toBeGreaterThan(0.2);
    expect(paused!.frames).toBeGreaterThan(2);
    // Keep the accepted real form's response pending while the native pause and
    // page-owned progress callbacks finish; media time and Server state stay real.
    await page.waitForTimeout(500);
    expect(await progress()).toMatchObject({watched, seconds: 0});
    expect(latePositions).toEqual([]);
    release();
    await expect(page.getByRole("button", {name: `Mark ${watched ? "unwatched" : "watched"}`, exact: true})).toBeVisible();
    await page.getByRole("link", {name: "Library", exact: true}).click();
    await expect(page).toHaveURL("/");
    const stored = await progress();
    expect(stored).toMatchObject({watched, seconds: 0});
    await info.attach("accepted-watched-late-native-notification", {body: JSON.stringify({revision: process.env.KINOSAIL_TEST_REVISION,
      browser: info.project.name, cancelledAgain, stored, paused, latePositions, result: "passed",
      data: "Real Go Server and moving decoded media; queued actual native playing notifications; held real watched 303 response"}), contentType: "application/json"});
  } finally {release(); await page.unrouteAll({behavior: "wait"}); await relay.close();}
});
}
}
