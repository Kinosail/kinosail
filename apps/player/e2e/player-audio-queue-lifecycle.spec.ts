import {expect, test} from "@playwright/test";
import {openAudio, queueItem, startQueue} from "./player-audio-queue-fixture";
import {playerSource} from "./static-sources";

// These held responses exercise closed-page and ownership races that the real
// Server cannot produce deterministically. They are isolated browser controls.
for (const boundary of ["pagehide", "cast", "room"]) {
  test(`late queue item authorization cannot cross ${boundary}`, {tag: ["@smoke", "@routed-fault"]}, async ({page}) => {
    await openAudio(page, "");
    let lookup: import("@playwright/test").Route | undefined;
    const writes: string[] = [];
    page.on("request", request => {
      if (new URL(request.url()).pathname.startsWith("/progress/")) writes.push(request.url());
    });
    await page.route("https://audio.test/api/v1/items/next", route => {lookup = route;});
    await startQueue(page);
    await page.getByRole("button", {name: "Next track", exact: true}).click();
    await expect.poll(() => !!lookup).toBe(true);
    await expect(page.locator("[data-audio-queue-controls]")).toHaveAttribute("aria-busy", "true");
    if (boundary === "pagehide") await page.evaluate(() => {
      // Avoid a real page-exit checkpoint here; this case isolates the late read.
      document.querySelector("audio")!.dataset.offline = "true";
      dispatchEvent(new PageTransitionEvent("pagehide"));
    });
    else await page.locator("audio").evaluate((audio, owner) => {
      if (owner === "cast") audio.dataset.castActive = "true";
      else audio.dataset.room = "existing-room";
    }, boundary);
    if (boundary === "pagehide") await expect(page.locator("[data-audio-queue-controls]")).toHaveAttribute("aria-busy", "true");
    await lookup!.fulfill({json: {item: queueItem("next"), profileId: "qa-viewer"}});
    if (boundary !== "pagehide") await expect(page.locator("[data-audio-queue-status]")).toHaveAttribute("data-queue-failure", "ownership");
    // Busy clears only when the held queue operation completes, so the old
    // identity assertions cannot pass before its response is consumed.
    await expect(page.locator("[data-audio-queue-controls]")).not.toHaveAttribute("aria-busy");
    await expect(page.locator("[data-audio-next]")).toBeDisabled();
    await expect(page.locator("audio")).toHaveAttribute("data-progress", "/progress/track");
    await expect(page.locator(".title-block h1")).toHaveText("First track");
    expect(writes).toEqual([]);
    if (boundary === "pagehide") await expect(page.locator("audio")).not.toHaveAttribute("src", /media\/next/);
  });
}

test("queued short track resumes its saved position without claiming unplayed progress", {tag: ["@smoke", "@routed-fault"]}, async ({page}) => {
  await openAudio(page, "");
  await page.locator("audio").evaluate(audio => Object.defineProperty(audio, "duration", {value: 8}));
  await page.route("https://audio.test/api/v1/items/next", route => route.fulfill({json: {
    item: {...queueItem("next"), progress: {seconds: 3, watched: false}}, profileId: "qa-viewer",
  }}));
  const nextWrites: string[] = [];
  page.on("request", request => {
    if (new URL(request.url()).pathname === "/progress/next") nextWrites.push(request.postData() || "");
  });
  await startQueue(page);
  await page.getByRole("button", {name: "Next track", exact: true}).click();
  await expect(page.locator("audio")).toHaveAttribute("data-progress", "/progress/next");
  await expect(page.locator("audio")).toHaveJSProperty("currentTime", 3);
  await page.locator("audio").dispatchEvent("pause");
  expect(nextWrites).toEqual([]);
});

test("late initial queue response cannot warm media or publish controls after pagehide", {tag: ["@smoke", "@routed-fault"]}, async ({page}) => {
  await openAudio(page, "");
  let lookup: import("@playwright/test").Route | undefined;
  const nextReads: string[] = [];
  page.on("request", request => {
    if (new URL(request.url()).pathname === "/media/next") nextReads.push(request.url());
  });
  await page.route("https://audio.test/api/v1/audio/track/queue", route => {lookup = route;});
  await page.addScriptTag({content: playerSource});
  await expect.poll(() => !!lookup).toBe(true);
  await expect(page.locator("[data-audio-queue-controls]")).toHaveAttribute("aria-busy", "true");
  await page.evaluate(() => dispatchEvent(new PageTransitionEvent("pagehide")));
  await expect(page.locator("[data-audio-queue-controls]")).toHaveAttribute("aria-busy", "true");
  await lookup!.fulfill({json: {items: [queueItem("track"), queueItem("next")]}});
  await expect(page.locator("[data-audio-queue-controls]")).not.toHaveAttribute("aria-busy");
  await expect(page.locator("[data-audio-next]")).toBeDisabled();
  await expect(page.locator("audio")).not.toHaveAttribute("data-queue-total");
  expect(nextReads).toEqual([]);
});

for (const saved of [true, false]) test(`ended offline queue requires its own watched acknowledgement: ${saved ? "saved" : "failed"}`, {tag: ["@smoke", "@routed-fault"]}, async ({page}) => {
  await openAudio(page, "");
  await startQueue(page);
  await page.evaluate(saved => {
    document.querySelector("audio")!.dataset.offline = "true";
    Object.assign(window, {r08OfflineEnded: {saved: 0, watched: false}, KinosailOfflineMedia: {
      saveProgress: async (_audio: HTMLAudioElement, watched: boolean) => {
        const observed = (window as Window & {r08OfflineEnded: {saved: number; watched: boolean}}).r08OfflineEnded;
        observed.saved++; observed.watched = watched; return {ok: saved};
      },
      unbindProgress: () => {},
    }});
    const request = window.fetch;
    Object.assign(window, {r08EndedTrackReads: 0});
    window.fetch = (...args) => {
      const input = args[0], target = new URL(input instanceof Request ? input.url : String(input), location.href);
      if (target.pathname === "/api/v1/items/next") (window as Window & {r08EndedTrackReads: number}).r08EndedTrackReads++;
      return request(...args);
    };
  }, saved);
  // The journal resolves synchronously. Complete its microtask continuation
  // before checking that failed acknowledgement did not begin authorization.
  await page.evaluate(async () => {
    document.querySelector("audio")!.dispatchEvent(new Event("ended"));
    await new Promise<void>(resolve => setTimeout(resolve, 0));
  });
  await expect.poll(() => page.evaluate(() => (window as Window & {r08OfflineEnded: {saved: number}}).r08OfflineEnded.saved)).toBe(1);
  if (saved) await expect(page.locator("audio")).toHaveAttribute("data-progress", "/progress/next");
  else {
    await expect(page.locator("[data-audio-queue-controls]")).not.toHaveAttribute("aria-busy");
    await expect(page.locator("audio")).toHaveAttribute("data-progress", "/progress/track");
    expect(await page.evaluate(() => (window as Window & {r08EndedTrackReads: number}).r08EndedTrackReads)).toBe(0);
  }
  expect(await page.evaluate(() => (window as Window & {r08OfflineEnded: {saved: number; watched: boolean}}).r08OfflineEnded)).toEqual({saved: 1, watched: true});
});

test("returning queue item does not reuse the previous source's played intent", {tag: ["@smoke", "@routed-fault"]}, async ({page}) => {
  await openAudio(page, "");
  const writes: string[] = [];
  page.on("request", request => {
    if (new URL(request.url()).pathname.startsWith("/progress/")) writes.push(new URL(request.url()).pathname);
  });
  await startQueue(page);
  await page.getByRole("button", {name: "Next track", exact: true}).click();
  await expect(page.locator("audio")).toHaveAttribute("data-progress", "/progress/next");
  await expect(page.getByRole("button", {name: "Previous track", exact: true})).toBeEnabled();
  await page.getByRole("button", {name: "Previous track", exact: true}).click();
  await expect(page.locator("audio")).toHaveAttribute("data-progress", "/progress/track");
  await expect(page.getByRole("button", {name: "Next track", exact: true})).toBeEnabled();
  await page.locator("audio").dispatchEvent("pause");
  await expect(page.locator("[data-audio-queue-controls]")).not.toHaveAttribute("aria-busy");
  expect(writes).toEqual(["/progress/track"]);
});

test("canonical catalog year strings survive queue validation and current-track identity", {tag: ["@smoke", "@routed-fault"]}, async ({page}) => {
  await openAudio(page, "");
  await page.route("https://audio.test/api/v1/audio/track/queue", route => route.fulfill({json: {
    items: [{...queueItem("track"), year: "2026"}, {...queueItem("next"), year: "2027"}],
  }}));
  await page.route("https://audio.test/api/v1/items/next", route => route.fulfill({json: {
    profileId: "qa-viewer", item: {...queueItem("next"), year: "2027"},
  }}));
  await startQueue(page);
  await page.getByRole("button", {name: "Next track", exact: true}).click();
  await expect(page.locator("audio")).toHaveAttribute("data-progress", "/progress/next");
  await expect(page.locator(".title-block h1")).toHaveText("Next track 2027");
  await expect(page.locator("audio")).toHaveAttribute("data-title", "Next track");
});
