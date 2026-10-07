import { expect, test, type Locator, type Page, type Request as PlaywrightRequest, type TestInfo } from "@playwright/test";

type Checkpoint = {seconds: number; revision: number; sessionMatches?: boolean};
type Observation = {key: string; iteration: number; testInfo: TestInfo};
type Movie = {watch: string; media: Locator; id: string; session?: string};

// Share the existing real-media flow without duplicating setup or changing required suite selection.
export function registerNavigationCheckpoints(flows: {
  phase: string;
  browsePath: string;
  checkpoint: (page: Page, id: string, session?: string) => Promise<Checkpoint>;
  openMovie: (page: Page, observation?: Observation) => Promise<Movie>;
}) {
  const {phase, checkpoint, openMovie, browsePath} = flows;
test.describe("acknowledged Library navigation", () => {
  test.use({serviceWorkers: "block"});
  // The populated media and progress store remain real. Only transport acknowledgement is held:
  // an unload keepalive cannot prove this important pending-response navigation boundary.
  test("Library waits for a real checkpoint acknowledgement before leaving moving media", {tag: "@smoke"}, async ({page}, testInfo) => {
    test.skip(phase !== "candidate", "historical sources are reserved for the original checkpoint reproductions");
    const {watch, media, id, session} = await openMovie(page, {key: "kinosail:checkpoint-ack-observation", iteration: 0, testInfo});
    let release!: () => void;
    const acknowledgement = new Promise<void>(resolve => release = resolve);
    const writes: Array<{seconds: number; revision: number; sessionMatches: boolean}> = [];
    await media.evaluate((video: HTMLVideoElement) => video.play());
    const start = await media.evaluate((video: HTMLVideoElement) => video.currentTime);
    await expect.poll(() => media.evaluate((video: HTMLVideoElement) => video.currentTime)).toBeGreaterThan(start + 0.5);
    const leaveAt = await media.evaluate((video: HTMLVideoElement) => video.currentTime);
    const before = await checkpoint(page, id, session);
    expect(before.sessionMatches).toBe(true);
    await page.route(`**/progress/${id}*`, async route => {
      const form = new URLSearchParams(route.request().postData() || "");
      writes.push({seconds: Number(form.get("seconds")), revision: Number(form.get("revision")),
        sessionMatches: form.get("session") === session});
      await acknowledgement;
      await route.continue();
    });
    try {
      await page.getByRole("link", {name: "Back to search results", exact: true}).click({noWaitAfter: true});
      await expect.poll(() => writes.length, {timeout: 1500}).toBeGreaterThan(0);
      await expect(page).toHaveURL(url => url.pathname === watch, {timeout: 500});
      await expect(media).toHaveJSProperty("paused", true);
      await expect(page.locator("[data-progress-notice]")).toHaveAttribute("aria-busy", "true");
      await page.screenshot({path: testInfo.outputPath("pending-library-save.png"), fullPage: true});
      const whileHeld = await checkpoint(page, id, session);
      expect(whileHeld).toEqual(before);
      // The server receives every original validated request only after this bounded hold.
      release();
      await page.unrouteAll({behavior: "wait"});
      await expect(page).toHaveURL(browsePath);
      const saved = await checkpoint(page, id, session);
      expect(saved.sessionMatches).toBe(true);
      expect(saved.seconds).toBeGreaterThanOrEqual(leaveAt - 0.1);
      expect(saved.revision).toBeGreaterThan(before.revision);
      expect(writes.every(write => write.sessionMatches && write.seconds >= leaveAt - 0.1)).toBe(true);
      await testInfo.attach("acknowledged-library-checkpoint", {body: JSON.stringify({phase, leaveAt, before,
        whileHeld, saved, writes, transport: "response held before forwarding original request to real Server"}),
        contentType: "application/json"});
      await page.goto(watch);
      await expect.poll(async () => Number(await page.locator("video").getAttribute("data-start"))).toBeGreaterThanOrEqual(leaveAt - 0.1);
      await expect.poll(() => page.locator("video").evaluate((video: HTMLVideoElement) => video.readyState)).toBeGreaterThanOrEqual(2);
      await expect.poll(() => page.locator("video").evaluate((video: HTMLVideoElement) => video.currentTime)).toBeGreaterThanOrEqual(leaveAt - 0.1);
      await page.locator("video").evaluate((video: HTMLVideoElement) => video.play());
      await expect.poll(() => page.locator("video").evaluate((video: HTMLVideoElement) => video.currentTime)).toBeGreaterThan(leaveAt + 0.2);
      await expect.poll(() => page.locator("video").evaluate((video: HTMLVideoElement) => video.getVideoPlaybackQuality().totalVideoFrames)).toBeGreaterThan(2);
      await page.screenshot({path: testInfo.outputPath("acknowledged-library-reentry.png"), fullPage: true});
    } finally {
      release();
      await page.unrouteAll({behavior: "ignoreErrors"});
    }
  });

  test("failed Library checkpoint stays recoverable and resumed playback cancels departure", {tag: "@smoke"}, async ({page}, testInfo) => {
    test.skip(phase !== "candidate", "historical sources are reserved for the original checkpoint reproductions");
    // The populated Server/media are real; a synthetic 503 isolates the unique failed-navigation
    // continuation boundary that successful populated journeys cannot reliably produce.
    const {watch, media, id, session} = await openMovie(page, {key: "kinosail:checkpoint-failed-navigation", iteration: 0, testInfo});
    await media.evaluate((video: HTMLVideoElement) => video.play());
    const start = await media.evaluate((video: HTMLVideoElement) => video.currentTime);
    await expect.poll(() => media.evaluate((video: HTMLVideoElement) => video.currentTime)).toBeGreaterThan(start + 0.5);
    const before = await checkpoint(page, id, session);
    let fail = true;
    await page.route(`**/progress/${id}*`, route => fail ? route.fulfill({status: 503}) : route.continue());
    try {
      await page.getByRole("link", {name: "Back to search results", exact: true}).click({noWaitAfter: true});
      await expect(page).toHaveURL(url => url.pathname === watch);
      await expect(media).toHaveJSProperty("paused", true);
      await expect(page.locator("[data-progress-notice]")).toBeVisible();
      await expect(page.getByRole("button", {name: "Retry saving position", exact: true})).toBeVisible();
      await expect(page.getByRole("button", {name: "Continue without saving", exact: true})).toBeVisible();
      expect(await checkpoint(page, id, session)).toEqual(before);
      await page.screenshot({path: testInfo.outputPath("failed-library-save.png"), fullPage: true});
      // Continuing playback withdraws the Library intent; the next successful save must stay here.
      fail = false;
      await media.evaluate((video: HTMLVideoElement) => video.play());
      const resumed = await media.evaluate((video: HTMLVideoElement) => video.currentTime);
      await expect.poll(() => media.evaluate((video: HTMLVideoElement) => video.currentTime)).toBeGreaterThan(resumed + 0.3);
      await media.evaluate((video: HTMLVideoElement) => video.pause());
      const paused = await media.evaluate((video: HTMLVideoElement) => video.currentTime);
      await expect.poll(async () => {
        const state = await checkpoint(page, id, session);
        return state.sessionMatches ? Math.abs(state.seconds - paused) : Infinity;
      }).toBeLessThan(0.1);
      const recovered = await checkpoint(page, id, session);
      expect(recovered.revision).toBeGreaterThan(before.revision);
      await expect(page.locator("[data-progress-notice]")).toBeHidden();
      await expect(page).toHaveURL(url => url.pathname === watch);
      // A fresh failed exit still permits an explicit departure without discarding valid progress.
      await media.evaluate((video: HTMLVideoElement) => video.play());
      await expect.poll(() => media.evaluate((video: HTMLVideoElement) => video.currentTime)).toBeGreaterThan(paused + 0.3);
      fail = true;
      await page.getByRole("link", {name: "Back to search results", exact: true}).click({noWaitAfter: true});
      await expect(page.getByRole("button", {name: "Continue without saving", exact: true})).toBeVisible();
      const departurePosition = await media.evaluate((video: HTMLVideoElement) => video.currentTime);
      await page.getByRole("button", {name: "Continue without saving", exact: true}).click();
      await expect(page).toHaveURL(browsePath);
      const afterUnsavedDeparture = await checkpoint(page, id, session);
      const fallbackAccepted = afterUnsavedDeparture.revision > recovered.revision;
      if (fallbackAccepted) {
        expect(afterUnsavedDeparture.sessionMatches).toBe(true);
        expect(Math.abs(afterUnsavedDeparture.seconds - departurePosition)).toBeLessThan(0.1);
      } else expect(afterUnsavedDeparture).toEqual(recovered);
      await testInfo.attach("failed-library-recovery", {body: JSON.stringify({phase, before, paused, recovered,
        departurePosition, afterUnsavedDeparture, fallbackAccepted,
        failure: "synthetic 503 at browser transport boundary", cancelledNavigation: true}),
        contentType: "application/json"});
    } finally { await page.unrouteAll({behavior: "ignoreErrors"}); }
  });


  test("an existing authentication or policy failure still allows an explicit Library departure", {tag: "@smoke"}, async ({page}, testInfo) => {
    test.skip(phase !== "candidate", "historical sources are reserved for the original checkpoint reproductions");
    // Synthetic HTTP failures protect the pending-navigation recovery seam; media/store remain real.
    for (const [iteration, status] of [401, 403].entries()) {
      const {watch, media, id, session} = await openMovie(page, {key: `kinosail:checkpoint-terminal-navigation:${iteration}`, iteration, testInfo});
      await media.evaluate((video: HTMLVideoElement) => video.play());
      const start = await media.evaluate((video: HTMLVideoElement) => video.currentTime);
      await expect.poll(() => media.evaluate((video: HTMLVideoElement) => video.currentTime)).toBeGreaterThan(start + 0.3);
      const before = await checkpoint(page, id, session);
      await page.route(`**/progress/${id}*`, route => route.fulfill({status}));
      try {
        await media.evaluate((video: HTMLVideoElement) => video.pause());
        const departurePosition = await media.evaluate((video: HTMLVideoElement) => video.currentTime);
        await expect(page.locator("[data-progress-status]")).toHaveAttribute("data-progress-failure", status === 401 ? "authentication" : "policy");
        await page.getByRole("link", {name: "Back to search results", exact: true}).click({noWaitAfter: true});
        await expect(page).toHaveURL(url => url.pathname === watch);
        await expect(page.getByRole("button", {name: "Retry saving position", exact: true})).toBeHidden();
        await expect(page.getByRole("button", {name: "Continue without saving", exact: true})).toBeVisible();
        expect(await checkpoint(page, id, session)).toEqual(before);
        await page.getByRole("button", {name: "Continue without saving", exact: true}).click();
        await expect(page).toHaveURL(browsePath);
        const after = await checkpoint(page, id, session);
        const fallbackAccepted = after.revision > before.revision;
        if (fallbackAccepted) {
          expect(after.sessionMatches).toBe(true);
          expect(Math.abs(after.seconds - departurePosition)).toBeLessThan(0.1);
        } else expect(after).toEqual(before);
        await testInfo.attach(`terminal-library-failure-${status}`, {body: JSON.stringify({phase, status, before,
          departurePosition, after, fallbackAccepted}),
          contentType: "application/json"});
      } finally { await page.unrouteAll({behavior: "ignoreErrors"}); }
    }
  });

  test("Library deadline expires without online renewal and explicit Retry gets a fresh bounded attempt", {tag: "@smoke"}, async ({page}, testInfo) => {
    test.skip(phase !== "candidate", "historical sources are reserved for the original checkpoint reproductions");
    // A held public transport is the sole isolation: replay cannot rely on a naturally stalled Server.
    const {watch, media, id, session} = await openMovie(page, {key: "kinosail:checkpoint-navigation-deadline", iteration: 0, testInfo});
    await media.evaluate((video: HTMLVideoElement) => video.play());
    const start = await media.evaluate((video: HTMLVideoElement) => video.currentTime);
    await expect.poll(() => media.evaluate((video: HTMLVideoElement) => video.currentTime)).toBeGreaterThan(start + 0.3);
    const before = await checkpoint(page, id, session);
    let release!: () => void;
    const held = new Promise<void>(resolve => release = resolve);
    const writes: Array<{seconds: number; revision: number; sessionMatches: boolean}> = [];
    await page.route(`**/progress/${id}*`, async route => {
      const form = new URLSearchParams(route.request().postData() || "");
      writes.push({seconds: Number(form.get("seconds")), revision: Number(form.get("revision")),
        sessionMatches: form.get("session") === session});
      await held;
      // An expired original request may already be cancelled; durable assertions verify the fresh retry.
      await route.continue().catch(() => {});
    });
    const began = await page.evaluate(() => performance.now());
    try {
      await page.getByRole("link", {name: "Back to search results", exact: true}).click({noWaitAfter: true});
      await expect.poll(() => writes.length, {timeout: 1500}).toBeGreaterThan(0);
      await expect(page.locator("[data-progress-status]")).toHaveAttribute("data-progress-failure", "timeout", {timeout: 10000});
      const elapsedMs = await page.evaluate(started => performance.now() - started, began);
      expect(elapsedMs).toBeGreaterThanOrEqual(7500);
      expect(elapsedMs).toBeLessThan(10000);
      await expect(page).toHaveURL(url => url.pathname === watch);
      await expect(page.getByRole("button", {name: "Continue without saving", exact: true})).toBeVisible();
      expect(await checkpoint(page, id, session)).toEqual(before);
      const expiredWrites = writes.length;
      await page.evaluate(() => dispatchEvent(new Event("online")));
      await page.waitForTimeout(200);
      expect(writes.length).toBe(expiredWrites);
      await expect(page).toHaveURL(url => url.pathname === watch);
      await page.getByRole("button", {name: "Retry saving position", exact: true}).click();
      await expect.poll(() => writes.length).toBeGreaterThan(expiredWrites);
      await expect(page.locator("[data-progress-notice]")).toHaveAttribute("aria-busy", "true");
      release();
      await page.unrouteAll({behavior: "wait"});
      await expect(page).toHaveURL(browsePath);
      const saved = await checkpoint(page, id, session);
      expect(saved.sessionMatches).toBe(true);
      expect(saved.revision).toBeGreaterThan(before.revision);
      expect(saved.seconds).toBeGreaterThanOrEqual(start + 0.3 - 0.1);
      await testInfo.attach("bounded-library-navigation", {body: JSON.stringify({phase, boundMs: 8000,
        elapsedMs, before, saved, expiredWrites, writes, onlineRenewal: false}), contentType: "application/json"});
    } finally { release(); await page.unrouteAll({behavior: "ignoreErrors"}); }
  });


  test("a newer same-tab destination waits for the latest checkpoint and replaces pending Library navigation", {tag: "@smoke"}, async ({page}, testInfo) => {
    test.skip(phase !== "candidate", "historical sources are reserved for the original checkpoint reproductions");
    // Both destinations/store are populated. Held transports expose the otherwise brief intent race.
    const {media, id, session} = await openMovie(page, {key: "kinosail:checkpoint-newer-destination", iteration: 0, testInfo});
    await media.evaluate((video: HTMLVideoElement) => video.play());
    const start = await media.evaluate((video: HTMLVideoElement) => video.currentTime);
    await expect.poll(() => media.evaluate((video: HTMLVideoElement) => video.currentTime)).toBeGreaterThan(start + 0.3);
    let releaseProgress!: () => void, releaseDestination!: () => void;
    const progressGate = new Promise<void>(resolve => releaseProgress = resolve);
    const destinationGate = new Promise<void>(resolve => releaseDestination = resolve);
    let writes = 0, destinationRequested = false, unexpectedLibraryRequests = 0;
    const observeNavigation = (request: PlaywrightRequest) => {
      const url = new URL(request.url());
      if (request.isNavigationRequest() && url.pathname + url.search === browsePath) unexpectedLibraryRequests++;
    };
    page.on("request", observeNavigation);
    await page.route(`**/progress/${id}*`, async route => {
      writes++;
      await progressGate;
      await route.continue();
    });
    await page.route("**/?view=movies", async route => {
      destinationRequested = true;
      await destinationGate;
      await route.continue();
    });
    await page.evaluate(() => {
      const link = document.createElement("a");
      link.href = "/?view=movies"; link.textContent = "Other Library view";
      document.body.prepend(link);
    });
    try {
      await page.getByRole("link", {name: "Back to search results", exact: true}).click({noWaitAfter: true});
      await expect.poll(() => writes).toBeGreaterThan(0);
      await page.getByRole("link", {name: "Other Library view", exact: true}).click({noWaitAfter: true});
      expect(destinationRequested).toBe(false);
      expect(unexpectedLibraryRequests).toBe(0);
      await expect(page).toHaveURL(url => url.pathname.startsWith("/watch/"));
      await expect(media).toHaveJSProperty("paused", true);
      await expect(page.locator("[data-progress-notice]")).toHaveAttribute("aria-busy", "true");
      releaseProgress();
      await expect.poll(() => destinationRequested).toBe(true);
      // A pending document navigation can freeze old-page DOM queries. Read the real store instead.
      await expect.poll(async () => {
        const state = await checkpoint(page, id, session);
        return state.sessionMatches && state.seconds >= start + 0.3 - 0.1;
      }).toBe(true);
      expect(unexpectedLibraryRequests).toBe(0);
      releaseDestination();
      await page.unrouteAll({behavior: "wait"});
      await expect(page).toHaveURL(url => url.pathname === "/" && url.searchParams.get("view") === "movies");
      const saved = await checkpoint(page, id, session);
      expect(saved.sessionMatches).toBe(true);
      expect(saved.seconds).toBeGreaterThanOrEqual(start + 0.3 - 0.1);
      expect(unexpectedLibraryRequests).toBe(0);
      await testInfo.attach("newer-navigation-checkpoint", {body: JSON.stringify({phase, writes, saved, neitherDestinationBeforeAcknowledgement: true,
        unexpectedLibraryRequests, chosenDestination: "movies Library view"}), contentType: "application/json"});
    } finally {
      page.off("request", observeNavigation);
      releaseProgress(); releaseDestination();
      await page.unrouteAll({behavior: "ignoreErrors"});
    }
  });

});

}
