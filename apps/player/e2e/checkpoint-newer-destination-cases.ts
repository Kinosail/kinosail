import {expect, test, type Request as PlaywrightRequest} from "@playwright/test";
import {startPlaying} from "./checkpoint-setup-cases";
import type {NavigationCheckpointFlows} from "./checkpoint-navigation-cases";

declare const progressNavigation: object | undefined;

export function registerNewerDestinationCheckpoint(flows: NavigationCheckpointFlows) {
  const {phase, browsePath, checkpoint, openMovie} = flows;
  test("a newer same-tab destination withdraws a pending Library checkpoint navigation", {tag: "@smoke"}, async ({page}, testInfo) => {
    test.skip(phase !== "candidate", "historical sources are reserved for the original checkpoint reproductions");
    // Both destinations/store are populated. Held transports expose the otherwise brief intent race.
    const {media, id, session} = await openMovie(page, {key: "kinosail:checkpoint-newer-destination", iteration: 0, testInfo});
    await startPlaying(media);
    const start = await media.evaluate((video: HTMLVideoElement) => video.currentTime);
    await expect.poll(() => media.evaluate((video: HTMLVideoElement) => video.currentTime)).toBeGreaterThan(start + 0.3);
    let releaseProgress!: () => void, releaseDestination!: () => void;
    const progressGate = new Promise<void>(resolve => releaseProgress = resolve);
    const destinationGate = new Promise<void>(resolve => releaseDestination = resolve);
    let pendingProgress = 0, canceledAcknowledgements = 0;
    let writes = 0, acknowledgementHeld = false, destinationRequested = false, unexpectedLibraryRequests = 0;
    const observeNavigation = (request: PlaywrightRequest) => {
      const url = new URL(request.url());
      if (request.isNavigationRequest() && url.pathname + url.search === browsePath) unexpectedLibraryRequests++;
    };
    page.on("request", observeNavigation);
    await page.route(`**/progress/${id}*`, async route => {
      pendingProgress++;
      try {
        writes++;
        const response = await route.fetch();
        expect(response.status()).toBe(204);
        acknowledgementHeld = true;
        await progressGate;
        try {await route.fulfill({response});}
        catch (error) {
          if (!destinationRequested || !String(error).includes("Route is already handled")) throw error;
          canceledAcknowledgements++;
        }
      } finally { pendingProgress--; }
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
      sessionStorage.removeItem("kinosail:checkpoint-navigation-intents");
      document.addEventListener("click", event => {
        const anchor = event.target instanceof Element ? event.target.closest("a") : null;
        if (!anchor || !anchor.matches("[data-browse-return]") && anchor.textContent?.trim() !== "Other Library view") return;
        const receipts = JSON.parse(sessionStorage.getItem("kinosail:checkpoint-navigation-intents") || "[]");
        receipts.push({destination: anchor.matches("[data-browse-return]") ? "Library" : anchor.textContent?.trim(), libraryIntentPending: Boolean(progressNavigation)});
        sessionStorage.setItem("kinosail:checkpoint-navigation-intents", JSON.stringify(receipts));
      });
    });
    try {
      await page.getByRole("link", {name: "Back to search results", exact: true}).click({noWaitAfter: true});
      await expect.poll(() => writes).toBeGreaterThan(0);
      await expect.poll(() => acknowledgementHeld).toBe(true);
      await page.getByRole("link", {name: "Other Library view", exact: true}).click({noWaitAfter: true});
      await expect.poll(() => destinationRequested).toBe(true);
      releaseProgress();
      // A pending document navigation can freeze old-page DOM queries. Read the real store instead.
      await expect.poll(async () => {
        const state = await checkpoint(page, id, session);
        return state.sessionMatches && state.seconds >= start + 0.3 - 0.1;
      }).toBe(true);
      expect(unexpectedLibraryRequests).toBe(0);
      releaseDestination();
      await expect(page).toHaveURL(url => url.pathname === "/" && url.searchParams.get("view") === "movies");
      // Join our callbacks, not Playwright bookkeeping left pending by a canceled old route.
      await expect.poll(() => pendingProgress).toBe(0);
      await page.unrouteAll();
      const saved = await checkpoint(page, id, session);
      expect(saved.sessionMatches).toBe(true);
      expect(saved.seconds).toBeGreaterThanOrEqual(start + 0.3 - 0.1);
      const intents = await page.evaluate(() => JSON.parse(sessionStorage.getItem("kinosail:checkpoint-navigation-intents") || "[]"));
      expect(intents).toEqual([{destination: "Library", libraryIntentPending: true},
        {destination: "Other Library view", libraryIntentPending: false}]);
      expect(unexpectedLibraryRequests).toBe(0);
      await testInfo.attach("newer-navigation-checkpoint", {body: JSON.stringify({phase, writes, saved, intents, pendingProgress, canceledAcknowledgements,
        unexpectedLibraryRequests, chosenDestination: "movies Library view"}), contentType: "application/json"});
    } finally {
      page.off("request", observeNavigation);
      releaseProgress(); releaseDestination();
      await page.unrouteAll({behavior: "ignoreErrors"});
    }
  });
}
