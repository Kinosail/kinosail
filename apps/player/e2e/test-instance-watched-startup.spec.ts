import {expect, test} from "@playwright/test";
import {configureTestInstance, firstPlayable, login} from "./test-instance-helpers";
import {startPlaying} from "./checkpoint-setup-cases";

configureTestInstance();
test.use({serviceWorkers: "block"});
test.beforeEach(async ({page}) => {
  await page.addInitScript(() => {
    const add = EventTarget.prototype.addEventListener;
    const native: Array<Record<string, unknown>> = [];
    const mediaTime = Object.getOwnPropertyDescriptor(HTMLMediaElement.prototype, "currentTime");
    for (const event of ["loadstart", "loadedmetadata", "canplay", "play", "playing", "pause", "waiting", "stalled", "error", "ended", "seeking", "seeked", "timeupdate"]) {
      add.call(document, event, event => {
        if (!(event.target instanceof HTMLVideoElement)) return;
        const video = event.target;
        native.push({event: event.type, at: performance.now(), seconds: video.currentTime,
          nativeSeconds: mediaTime?.get?.call(video) ?? null, paused: video.paused, seeking: video.seeking,
          readyState: video.readyState, networkState: video.networkState, errorCode: video.error?.code ?? null,
          decodedFrames: video.getVideoPlaybackQuality().totalVideoFrames, pendingNotifications: (window as Window & {watchedStartup?: {pending: unknown[]}}).watchedStartup?.pending.length ?? 0});
        if (native.length > 96) native.shift();
      }, true);
    }
    Object.assign(window, {watchedNativeStartup: native});
  });
});

for (const watched of [true, false]) {
for (const cancelledAgain of [false, true]) {
test(`accepted Mark ${watched ? "watched" : "unwatched"} survives late native media callbacks from the departing page${cancelledAgain ? " after another submission is cancelled" : ""}`, {tag: "@smoke"}, async ({page}, info) => {
  await page.addInitScript(() => {
    const add = EventTarget.prototype.addEventListener;
    const pending: Array<() => void> = [];
    const control = {pending, registered: 0, release() {for (const notify of pending.splice(0)) notify();}};
    Object.assign(window, {watchedStartup: control});
    EventTarget.prototype.addEventListener = function(type, listener, options) {
      if (this instanceof HTMLVideoElement && type === "playing" && listener) {
        control.registered++;
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
  const progress = async () => {
    const response = await page.request.get(`/api/v1/items/${id}`);
    expect(response.status()).toBe(200);
    const stored = (await response.json()).item.progress;
    return {...stored, seconds: stored.seconds ?? 0, watched: stored.watched ?? false};
  };
  // Establish state before the watch page installs the delayed native-callback fixture.
  const setup = await page.request.put(`/api/v1/items/${id}/progress`, {headers: {Origin: new URL(page.url()).origin,
    "X-Kinosail-CSRF": await page.locator('meta[name="kinosail-csrf"]').getAttribute("content") || ""}, data: {seconds: 0, watched: !watched}});
  expect(setup.status()).toBe(200);
  expect(await progress()).toMatchObject({seconds: 0, watched: !watched});
  await page.goto(watch);
  await expect(page.getByRole("button", {name: `Mark ${watched ? "watched" : "unwatched"}`, exact: true})).toBeVisible();
  const media = page.locator("video");
  await expect.poll(() => page.evaluate(() => (window as Window & {watchedStartup: {registered: number}}).watchedStartup.registered)).toBeGreaterThan(0);
  let startupPhase = "pending-media";
  try {
    // Settle native startup before pausing; the one-shot canplay autoplay can otherwise resume this fixture.
    await media.evaluate((video: HTMLVideoElement) => {video.muted = true;});
    await expect.poll(() => media.evaluate((video: HTMLVideoElement) => video.readyState)).toBeGreaterThanOrEqual(2);
    startupPhase = "first-start";
    await startPlaying(media);
    // Settle the initial playing callbacks; hold the resume notifications for the departure race.
    await page.evaluate(() => {(window as Window & {watchedStartup: {release(): void}}).watchedStartup.release();});
    await media.evaluate((video: HTMLVideoElement) => video.pause());
    await expect(media).toHaveJSProperty("paused", true);
    startupPhase = "resume";
    await startPlaying(media);
    await expect.poll(() => page.evaluate(() => (window as Window & {watchedStartup: {pending: Array<() => void>}}).watchedStartup.pending.length)).toBeGreaterThan(0);
  } catch (error) {
    const mediaState = await media.evaluate((video: HTMLVideoElement) => ({seconds: video.currentTime,
      duration: Number.isFinite(video.duration) ? video.duration : null, paused: video.paused, ended: video.ended,
      seeking: video.seeking, readyState: video.readyState, networkState: video.networkState,
      errorCode: video.error?.code ?? null, decodedFrames: video.getVideoPlaybackQuality().totalVideoFrames,
      buffered: Array.from({length: Math.min(video.buffered.length, 8)}, (_, index) => [video.buffered.start(index), video.buffered.end(index)])})).catch(() => ({unavailable: true}));
    await info.attach("late-native-startup-failure-media-state", {body: JSON.stringify({revision: process.env.KINOSAIL_TEST_REVISION,
      browser: info.project.name, watched, cancelledAgain, phase: startupPhase, media: mediaState,
      events: await page.evaluate(() => (window as Window & {watchedNativeStartup: unknown[]}).watchedNativeStartup).catch(() => []),
      boundary: "Initial decoded-start setup before held navigation; no source URL or error text"}), contentType: "application/json"});
    throw error;
  }
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
    const relay = (window as Window & {qaWatchedRelay: Window}).qaWatchedRelay;
    addEventListener("message", async event => {
      if (event.source !== relay || event.origin !== location.origin || event.data !== "qa-watched-commit") return;
      const video = document.querySelector("video")!;
      if (cancelledAgain) {
        const form = document.querySelector<HTMLFormElement>('form[action^="/watched/"]')!;
        document.addEventListener("submit", event => event.preventDefault(), {capture: true, once: true});
        if (form.dispatchEvent(new SubmitEvent("submit", {bubbles: true, cancelable: true, submitter: form.querySelector("button")}))) throw new Error("Second submission was not cancelled");
      }
      // Native navigation can pause WebKit media before the response arrives.
      // Establish fresh decoded motion inside the held request before releasing
      // the page's queued real notifications and requesting the late native pause.
      const start = {seconds: video.currentTime, frames: video.getVideoPlaybackQuality().totalVideoFrames};
      let rejected = false, minimumFrames = start.frames;
      void video.play().catch(() => {rejected = true;});
      const deadline = performance.now() + 8000;
      while (video.paused || video.getVideoPlaybackQuality().totalVideoFrames <= minimumFrames + 2 || video.currentTime <= start.seconds + 0.2) {
        if (rejected || performance.now() >= deadline) {
          relay.postMessage({kind: "qa-watched-paused", failure: "native motion did not resume during held transport"}, location.origin);
          return;
        }
        // Native HLS may reset frame statistics after a seek. Still require
        // three newly decoded frames and advancing time after that reset.
        minimumFrames = Math.min(minimumFrames, video.getVideoPlaybackQuality().totalVideoFrames);
        await new Promise(resolve => setTimeout(resolve, 20));
      }
      const before = {seconds: video.currentTime, frames: video.getVideoPlaybackQuality().totalVideoFrames};
      (window as Window & {watchedStartup: {release(): void}}).watchedStartup.release();
      const report = (event: Event) => relay.postMessage({kind: "qa-watched-paused", event: event.type, paused: video.paused,
        before, seconds: video.currentTime, frames: video.getVideoPlaybackQuality().totalVideoFrames}, location.origin);
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
    // Preserve the actual form, cookies and CSRF on the single real Server POST.
    const response = await route.fetch({maxRedirects: 0, headers: {...await route.request().allHeaders(), "Sec-Fetch-Site": "same-origin"}});
    expect(response.status()).toBe(303);
    committed = true;
    await pending;
    // WebKit cannot fulfill redirects, and redirects bypass subsequent routes.
    // Abort this owned transport barrier after the proof, then reopen the real page.
    await route.abort("aborted");
  });
  let bodyFailed = false;
  try {
    if (!watched) {
      // A departing native HLS document may stop fetching new segments. Decode
      // and buffer the real tail before holding navigation; the late ended
      // callback must use available media rather than require another load.
      await media.evaluate((video: HTMLVideoElement) => {
        video.pause();
        if (!Number.isFinite(video.duration) || video.duration <= 3) throw new Error("Late ended fixture needs a finite video longer than three seconds");
        video.currentTime = video.duration - 2.5;
      });
      await expect.poll(() => media.evaluate((video: HTMLVideoElement) => video.seeking)).toBe(false);
      await expect.poll(() => media.evaluate((video: HTMLVideoElement) => {
        const target = video.duration - 0.1;
        return Array.from({length: video.buffered.length}, (_, index) =>
          video.buffered.start(index) <= target && video.buffered.end(index) >= target).some(Boolean);
      })).toBe(true);
      await startPlaying(media);
      await media.evaluate((video: HTMLVideoElement) => video.pause());
      await expect(media).toHaveJSProperty("paused", true);
      const tail = await media.evaluate((video: HTMLVideoElement) => ({seconds: video.currentTime,
        duration: video.duration, decodedFrames: video.getVideoPlaybackQuality().totalVideoFrames,
        buffered: Array.from({length: Math.min(video.buffered.length, 8)}, (_, index) => [video.buffered.start(index), video.buffered.end(index)])}));
      await info.attach("late-ended-buffered-precondition", {body: JSON.stringify({browser: info.project.name,
        cancelledAgain, tail, boundary: "Actual native tail seek and decoded motion before held navigation"}), contentType: "application/json"});
    }
    await page.getByRole("button", {name: `Mark ${watched ? "watched" : "unwatched"}`, exact: true}).click({noWaitAfter: true});
    await expect.poll(() => committed).toBe(true);
    expect(await progress()).toMatchObject({watched, seconds: 0});
    await relay.evaluate(origin => opener!.postMessage("qa-watched-commit", origin), origin);
    const observation = () => relay.evaluate(() => (window as Window & {qaWatchedObserved?: {event: string; paused: boolean; failure?: string; before: {seconds: number; frames: number}; seconds: number; frames: number}}).qaWatchedObserved);
    await expect.poll(() => observation().then(value => value?.failure || value?.paused)).toBe(true);
    const paused = (await observation())!;
    expect(paused.event).toBe(watched ? "pause" : "ended");
    expect(paused.before.seconds).toBeGreaterThan(0.2);
    expect(paused.before.frames).toBeGreaterThan(2);
    expect(paused.seconds).toBeGreaterThan(0.2);
    // Keep the accepted real form's transport pending while the native pause and
    // page-owned progress callbacks finish; media time and Server state stay real.
    await page.waitForTimeout(500);
    expect(await progress()).toMatchObject({watched, seconds: 0});
    expect(latePositions).toEqual([]);
    // The relay's proof is complete; remove this artificial auxiliary window
    // before reopening real pages with Cross-Origin-Opener-Policy isolation.
    await relay.close();
    release();
    await page.unrouteAll({behavior: "wait"});
    await page.goto(watch);
    await expect(page.getByRole("button", {name: `Mark ${watched ? "unwatched" : "watched"}`, exact: true})).toBeVisible();
    await page.getByRole("link", {name: "Library", exact: true}).click();
    await expect(page).toHaveURL("/");
    const stored = await progress();
    expect(stored).toMatchObject({watched, seconds: 0});
    await info.attach("accepted-watched-late-native-notification", {body: JSON.stringify({revision: process.env.KINOSAIL_TEST_REVISION,
      browser: info.project.name, cancelledAgain, stored, paused, latePositions, result: "passed",
      data: "Real Go Server and moving decoded media; queued actual native playing notifications; actual form POST replayed once with real 303 acknowledgement; held transport aborted after proof and real page reopened"}), contentType: "application/json"});
  } catch (error) {
    bodyFailed = true;
    // Read through the owned relay before aborting the pending navigation.
    // The old document can disappear before an ordinary afterEach inspection.
    const media = await relay.evaluate(() => {
      const video = (opener as Window | null)?.document.querySelector<HTMLVideoElement>("video");
      if (!video) return {unavailable: true};
      const ranges = (value: TimeRanges) => Array.from({length: Math.min(value.length, 8)},
        (_, index) => [value.start(index), value.end(index)]);
      return {seconds: video.currentTime, nativeSeconds: Object.getOwnPropertyDescriptor(HTMLMediaElement.prototype, "currentTime")?.get?.call(video) ?? null,
        duration: Number.isFinite(video.duration) ? video.duration : null, paused: video.paused, ended: video.ended, seeking: video.seeking,
        readyState: video.readyState, networkState: video.networkState, errorCode: video.error?.code ?? null,
        decodedFrames: video.getVideoPlaybackQuality().totalVideoFrames,
        events: (opener as Window & {watchedNativeStartup?: unknown[]}).watchedNativeStartup ?? [],
        buffered: ranges(video.buffered), seekable: ranges(video.seekable)};
    }).catch(() => ({unavailable: true}));
    await info.attach("late-native-failure-media-state", {body: JSON.stringify({revision: process.env.KINOSAIL_TEST_REVISION,
      browser: info.project.name, watched, cancelledAgain, phase: committed ? "held-or-reopened" : "pre-submit-tail", media,
      boundary: "Native snapshot before owned transport cleanup; no source URL or error text"}), contentType: "application/json"});
    throw error;
  }
  finally {
    release();
    const cleanup = await Promise.allSettled([page.unrouteAll({behavior: "wait"}), relay.close()]);
    const failures = cleanup.filter(result => result.status === "rejected");
    for (const failure of failures) info.annotations.push({type: "cleanup-error", description: String(failure.reason).slice(0, 1000)});
    if (!bodyFailed && failures.length) throw new AggregateError(failures.map(failure => failure.reason), "Watched fixture cleanup failed");
  }
});
}
}

// Keep ordinary form navigation covered without the delayed-callback injection.
test("watched controls navigate and persist both states during decoded playback", {tag: "@smoke"}, async ({page}, info) => {
  await login(page);
  const watch = await firstPlayable(page), id = watch.split("/").at(-1)!;
  const origin = new URL(page.url()).origin;
  const setup = await page.request.put(`/api/v1/items/${id}/progress`, {headers: {Origin: origin,
    "X-Kinosail-CSRF": await page.locator('meta[name="kinosail-csrf"]').getAttribute("content") || ""}, data: {seconds: 0, watched: false}});
  expect(setup.status()).toBe(200);
  await page.goto(watch);
  const observations: {watched: boolean; status: number; redirectedStatus: number}[] = [];
  for (const watched of [true, false]) {
    const media = page.locator("video");
    await media.evaluate((video: HTMLVideoElement) => {video.muted = true;});
    await expect.poll(() => media.evaluate((video: HTMLVideoElement) => video.readyState)).toBeGreaterThanOrEqual(2);
    try { await startPlaying(media); }
    catch (error) {
      const snapshot = await media.evaluate((video: HTMLVideoElement) => ({seconds: video.currentTime,
        nativeSeconds: Object.getOwnPropertyDescriptor(HTMLMediaElement.prototype, "currentTime")?.get?.call(video) ?? null,
        duration: Number.isFinite(video.duration) ? video.duration : null, paused: video.paused, ended: video.ended,
        seeking: video.seeking, readyState: video.readyState, networkState: video.networkState,
        errorCode: video.error?.code ?? null, decodedFrames: video.getVideoPlaybackQuality().totalVideoFrames,
        events: (window as Window & {watchedNativeStartup: unknown[]}).watchedNativeStartup})).catch(() => ({unavailable: true}));
      await info.attach("ordinary-native-startup-failure-media-state", {body: JSON.stringify({revision: process.env.KINOSAIL_TEST_REVISION,
        browser: info.project.name, watched, snapshot, boundary: "Ordinary native start; no queued-playing injection or source URL"}), contentType: "application/json"});
      throw error;
    }
    const [saved, redirected] = await Promise.all([
      page.waitForResponse(response => response.url() === `${origin}/watched/${id}` && response.request().method() === "POST"),
      page.waitForResponse(response => response.url() === `${origin}${watch}` && response.request().redirectedFrom()?.url() === `${origin}/watched/${id}`),
      page.getByRole("button", {name: `Mark ${watched ? "watched" : "unwatched"}`, exact: true}).click(),
    ]);
    expect(saved.status()).toBe(303);
    expect(redirected.status()).toBe(200);
    await expect(page.getByRole("button", {name: `Mark ${watched ? "unwatched" : "watched"}`, exact: true})).toBeVisible();
    await expect.poll(async () => {
      const response = await page.request.get(`/api/v1/items/${id}`);
      expect(response.status()).toBe(200);
      return Boolean((await response.json()).item.progress.watched);
    }).toBe(watched);
    observations.push({watched, status: saved.status(), redirectedStatus: redirected.status()});
  }
  await info.attach("ordinary-watched-form-navigation", {body: JSON.stringify({revision: process.env.KINOSAIL_TEST_REVISION,
    observations, data: "Real decoded media and native form navigation; no deferred callbacks or mocked transport", result: "passed"}), contentType: "application/json"});
});
