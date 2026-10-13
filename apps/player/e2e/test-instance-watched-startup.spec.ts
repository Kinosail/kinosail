import {expect, test} from "@playwright/test";
import {configureTestInstance, firstPlayable, login} from "./test-instance-helpers";
import {startPlaying} from "./checkpoint-setup-cases";
import {observeHeldWatchedMotion, recordWatchedNativeEvents, type WatchedNativeEvent, type WatchedObservation} from "./watched-startup-motion";
import {installDisplayedMotionTracker} from "./watched-motion-counter";
import {installWatchedGuardMutation, verifyMutationInitiations} from "./watched-guard-mutation";
import {installWatchedPlayingBarrier} from "./watched-playing-barrier";

configureTestInstance();
test.use({serviceWorkers: "block"});
test.beforeEach(async ({page}) => {await page.addInitScript(recordWatchedNativeEvents);});

type ProgressReceipt = {method: string; status: number | null};
const baselineAcknowledgement = "http-204-headers";

for (const watched of [true, false]) {
for (const cancelledAgain of [false, true]) {
for (const guardMutation of [false, true]) {
const scenario = `Mark ${watched ? "watched" : "unwatched"}${cancelledAgain ? " after another submission is cancelled" : ""}`;
const title = guardMutation ? `late callbacks detect phase-targeted departure guard mutation: ${scenario}` :
  `accepted Mark ${watched ? "watched" : "unwatched"} survives late native media callbacks from the departing page${cancelledAgain ? " after another submission is cancelled" : ""}`;
test(title, {tag: "@smoke"}, async ({page}, info) => {
  const mutation = guardMutation ? await installWatchedGuardMutation(page) : undefined;
  await page.addInitScript(installDisplayedMotionTracker);
  await page.addInitScript(installWatchedPlayingBarrier);
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
  let committed = false, baselineFrozen = false;
  const baselineRequests: Array<Promise<ProgressReceipt>> = [], latePositions: string[] = [];
  let baselineProgress: ProgressReceipt[] = [];
  page.on("request", request => {
    if (new URL(request.url()).pathname !== `/progress/${id}`) return;
    if (baselineFrozen) {latePositions.push(request.method()); return;}
    baselineRequests.push((async () => {
      const response = await request.response();
      return {method: request.method(), status: response?.status() ?? null};
    })().catch(() => ({method: request.method(), status: null})));
  });
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
      events: await page.evaluate(() => (window as Window & {watchedNativeStartup: WatchedNativeEvent[]}).watchedNativeStartup).catch(() => []),
      boundary: "Initial decoded-start setup before held navigation; no source URL or error text"}), contentType: "application/json"});
    throw error;
  }
  let endpointAdmission: {seconds: number; duration: number; bufferedStart: number; bufferedEnd: number} | undefined;
  if (!watched) {
    // Accepted departure stops HLS loading. Advance naturally with playing
    // callbacks still queued so the ending is already buffered on this timeline.
    await expect.poll(() => media.evaluate((video: HTMLVideoElement) => {
      const remaining = video.duration - video.currentTime;
      return !video.paused && !video.seeking && !video.ended && video.readyState >= 2 &&
        remaining >= 2 && remaining <= 3 && Array.from({length: video.buffered.length}, (_, index) =>
          video.buffered.start(index) <= video.currentTime && video.buffered.end(index) >= video.duration - 0.05).some(Boolean);
    }), {timeout: 20000, intervals: [50, 100]}).toBe(true);
    await media.evaluate((video: HTMLVideoElement) => video.pause());
    await expect(media).toHaveJSProperty("paused", true);
    await expect(media).toHaveJSProperty("ended", false);
    endpointAdmission = await media.evaluate((video: HTMLVideoElement) => {
      const range = Array.from({length: video.buffered.length}, (_, index) =>
        [video.buffered.start(index), video.buffered.end(index)]).find(([start, end]) => start <= video.currentTime && end >= video.currentTime);
      return {seconds: video.currentTime, duration: video.duration, bufferedStart: range?.[0] ?? -1, bufferedEnd: range?.[1] ?? -1};
    });
    expect(endpointAdmission.duration - endpointAdmission.seconds).toBeGreaterThan(1);
    expect(endpointAdmission.bufferedEnd).toBeGreaterThanOrEqual(endpointAdmission.duration - 0.05);
  }
  let release!: () => void;
  const pending = new Promise<void>(resolve => {release = resolve;});
  const origin = new URL(page.url()).origin;
  const relayReady = page.waitForEvent("popup");
  await page.evaluate(() => Object.assign(window, {qaWatchedRelay: window.open("about:blank", "qa-watched-relay")}));
  const relay = await relayReady;
  await relay.evaluate(origin => {
    addEventListener("message", event => {
      if (event.source === opener && event.origin === origin && event.data?.kind === "qa-watched-paused") Object.assign(window, {qaWatchedObserved: event.data});
    });
  }, origin);
  await page.evaluate(observeHeldWatchedMotion, {watched, cancelledAgain, guardMutation});
  await page.route(`**/watched/${id}`, async route => {
    // Native seeks can save progress before this departure. Navigation can also
    // finish a checkpoint before replaying the form. The Server sends 204 only
    // after SetRevision returns. A bodyless 204 is complete at its response headers.
    let deadline: ReturnType<typeof setTimeout> | undefined;
    try {
      expect(baselineFrozen).toBe(false);
      baselineFrozen = true;
      baselineProgress = await Promise.race([Promise.all(baselineRequests), new Promise<never>((_, reject) => {
        deadline = setTimeout(() => reject(new Error("Baseline progress was not acknowledged before watched submission")), 5_000);
      })]);
      expect(baselineProgress.every(receipt => receipt.method === "POST" && receipt.status === 204)).toBe(true);
    } catch (error) {
      await route.abort("aborted").catch(() => {});
      throw error;
    } finally { clearTimeout(deadline); }
    // Preserve the actual form, cookies and CSRF on the single real Server POST.
    const response = await route.fetch({maxRedirects: 0, headers: {...await route.request().allHeaders(), "Sec-Fetch-Site": "same-origin"}});
    expect(response.status()).toBe(303);
    committed = true;
    await pending;
    // WebKit cannot fulfill redirects, and redirects bypass subsequent routes.
    // Abort this owned transport barrier after the proof, then reopen the real page.
    await route.abort("aborted");
  });
  let bodyFailed = false, observed: WatchedObservation | undefined;
  try {
    await page.getByRole("button", {name: `Mark ${watched ? "watched" : "unwatched"}`, exact: true}).click({noWaitAfter: true});
    await expect.poll(() => committed).toBe(true);
    expect(baselineFrozen).toBe(true);
    expect(baselineProgress).toHaveLength(baselineRequests.length);
    expect(latePositions).toEqual([]);
    expect(await progress()).toMatchObject({watched, seconds: 0});
    await relay.evaluate(origin => opener!.postMessage("qa-watched-commit", origin), origin);
    const observation = () => relay.evaluate(() => (window as Window & {qaWatchedObserved?: WatchedObservation}).qaWatchedObserved);
    await expect.poll(async () => {observed = await observation(); return observed?.failure || observed?.paused;}).toBe(true);
    const paused = observed!;
    expect(paused.event).toBe(watched ? "pause" : "ended");
    expect(paused.before.seconds).toBeGreaterThan(0.2);
    expect(paused.before.frames).toBeGreaterThan(2);
    const motion = paused.before.displayedMotion;
    expect(motion.invalid).toBe(false);
    expect(motion.samples).toHaveLength(3);
    expect(paused.before.callbacksReleased).toBeGreaterThan(0);
    expect(motion.samples.every((sample, index, samples) => {
      const previous = samples[index - 1] ?? motion.baseline;
      return sample.seconds > previous.seconds && sample.total - sample.dropped > previous.total - previous.dropped;
    })).toBe(true);
    expect(paused.before.seconds).toBeGreaterThan(paused.before.startSeconds + 0.2);
    expect(paused.seconds).toBeGreaterThan(0.2);
    // Keep the accepted real form's transport pending while the native pause and
    // page-owned progress callbacks finish; media time and Server state stay real.
    await page.waitForTimeout(500);
    if (guardMutation) {
      expect(mutation!.replacements).toBe(1);
      expect(paused.before.guardMutation).toBe(true);
      await expect.poll(() => latePositions.filter(method => method === "POST").length).toBeGreaterThan(0);
      expect(latePositions.every(method => method === "POST")).toBe(true);
      const fetchInitiations = await verifyMutationInitiations(relay, baselineProgress.length, latePositions.length);
      await info.attach("phase-targeted-watched-guard-mutation", {body: JSON.stringify({browser: info.project.name,
        watched, cancelledAgain, mutation, paused, latePositions, baselineProgress, baselineAcknowledgement, fetchInitiations,
        stored: await progress(), boundary: "Mutated served script against populated Server; guard bypass activates only after motion admission. Not historical pre-fix replay."}), contentType: "application/json"});
      return;
    }
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
      browser: info.project.name, cancelledAgain, endpointAdmission, baselineProgress, baselineAcknowledgement,
      baselineBoundary: "actual watched request intercepted before Server POST", stored, paused, latePositions, result: "passed",
      data: "Real Go Server and moving decoded media; queued actual native playing notifications; actual form POST replayed once with real 303 acknowledgement; held transport aborted after proof and real page reopened"}), contentType: "application/json"});
  } catch (error) {
    bodyFailed = true;
    if (observed?.motion) await info.attach("watched-held-motion-failure", {body: JSON.stringify({revision: process.env.KINOSAIL_TEST_REVISION,
      browser: info.project.name, watched, cancelledAgain, motion: observed.motion}), contentType: "application/json"}).catch(() => {});
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
        events: (opener as Window & {watchedNativeStartup?: WatchedNativeEvent[]}).watchedNativeStartup ?? [],
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
        events: (window as Window & {watchedNativeStartup: WatchedNativeEvent[]}).watchedNativeStartup})).catch(() => ({unavailable: true}));
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
