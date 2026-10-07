import { expect, test } from "@playwright/test";
import { playerSource } from "./static-sources";
import {openAudio, queueItem, queueURL, startQueue} from "./player-audio-queue-fixture";

for (const policy of ["compatible", "direct-first", "direct-only", "", "unknown"]) {
  test(`audio keeps its source and queue with saved policy ${policy || "unset"}`, async ({ page }) => {
    const errors: string[] = [];
    const navigation: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    await openAudio(page, policy);
    page.on("request", (request) => { if (request.isNavigationRequest()) navigation.push(request.url()); });
    const queue = page.waitForResponse(queueURL);
    await page.addScriptTag({ content: playerSource });
    await queue;
    await expect(page.locator("audio")).toHaveAttribute("src", "/media/track?playbackSession=audio-policy-session");
    await page.locator("audio").dispatchEvent("ended");
    await expect(page.locator("audio")).toHaveAttribute("src", "/media/next?playbackSession=audio-policy-session");
    await expect(page.locator("audio")).toHaveAttribute("data-progress", "/progress/next");
    expect(await page.evaluate(() => localStorage.getItem("kinosail.playback-policy-v2"))).toBe(policy);
    expect(navigation).toEqual([]);
    expect(errors).toEqual([]);
  });
}

test("Home Assistant state uses the session CSRF token and consumes a command", async ({ page }) => {
  await openAudio(page, "", true);
  const reports: { csrf?: string; body: { itemId: string; position: number; duration: number } }[] = [];
  await page.route("https://audio.test/api/v1/home-assistant/players/*", async (route) => {
    reports.push({ csrf: route.request().headers()["x-kinosail-csrf"], body: route.request().postDataJSON() });
    await route.fulfill({ json: { command: "seek", position: 35 } });
  });
  await page.addScriptTag({ content: playerSource });
  await expect.poll(() => reports.length).toBe(1);
  expect(reports[0].csrf).toBe("session-csrf-token");
  expect(reports[0].body).toMatchObject({ itemId: "track", position: 12, duration: 120 });
  await expect(page.locator("audio")).toHaveJSProperty("currentTime", 35);
});

for (const invalid of ["external stream", "missing authorization", "wrong profile", "wrong item"]) {
  test(`queue rejects ${invalid} before progress or Now Playing side effects`, async ({page}) => {
    await openAudio(page, "direct-first");
    const writes: string[] = [];
    page.on("request", request => {if (new URL(request.url()).pathname.startsWith("/progress/")) writes.push(request.url());});
    await page.route("https://audio.test/api/v1/items/next", route => route.fulfill({json: {
      profileId: invalid === "wrong profile" ? "other-viewer" : "qa-viewer",
      item: {...queueItem(invalid === "wrong item" ? "different" : "next"),
        stream: invalid === "external stream" ? "https://untrusted.invalid/media/next?token=private" : invalid === "missing authorization" ? "" : "/media/next"},
    }}));
    await startQueue(page);
    await page.getByRole("button", {name: "Next track", exact: true}).click();
    await expect(page.locator("[data-audio-queue-status]")).toHaveAttribute("data-queue-failure", "invalid");
    await expect(page.locator("audio")).toHaveAttribute("data-progress", "/progress/track");
    await expect(page.locator("audio")).toHaveAttribute("src", "/media/track?playbackSession=audio-policy-session");
    await expect(page.locator(".title-block h1")).toHaveText("First track");
    expect(writes).toEqual([]);
    expect(await page.locator("[data-audio-queue-status]").textContent()).not.toMatch(/private|untrusted/);
  });
}

test("manual queue advance retains failed position and waits for Retry before another choice", async ({page}) => {
  await openAudio(page, "direct-only");
  let progressStatus = 503;
  const revisions: string[] = [];
  await page.route("https://audio.test/progress/track", route => {
    revisions.push(new URLSearchParams(route.request().postData()!).get("revision")!);
    return route.fulfill({status: progressStatus});
  });
  await startQueue(page);
  await page.getByRole("button", {name: "Next track", exact: true}).click();
  await expect(page.locator("[data-progress-notice]")).toBeVisible();
  await expect(page.locator("audio")).toHaveAttribute("data-progress", "/progress/track");
  progressStatus = 204;
  await page.getByRole("button", {name: "Retry saving position", exact: true}).click();
  await expect(page.locator("[data-progress-notice]")).toBeHidden();
  expect(revisions[1]).toBe(revisions[0]);
  await page.getByRole("button", {name: "Next track", exact: true}).click();
  await expect(page.locator("audio")).toHaveAttribute("data-progress", "/progress/next");
});

test("queue source load ignores its delayed pause until new metadata belongs to that source", async ({page}) => {
  await openAudio(page, "");
  const nextWrites: string[] = [];
  page.on("request", request => {if (new URL(request.url()).pathname === "/progress/next") nextWrites.push(request.postData() || "");});
  await page.locator("audio").evaluate(audio => {
    Object.defineProperty(audio, "load", {value: () => audio.dispatchEvent(new Event("pause")), configurable: true});
  });
  await startQueue(page);
  await page.getByRole("button", {name: "Next track", exact: true}).click();
  await expect(page.locator("audio")).toHaveAttribute("data-progress", "/progress/next");
  expect(nextWrites).toEqual([]);
  await page.locator("audio").dispatchEvent("loadedmetadata");
  await page.locator("audio").dispatchEvent("pause");
  expect(nextWrites).toEqual([]);
  await page.locator("audio").dispatchEvent("kinosail:seek-intent");
  await page.locator("audio").dispatchEvent("pause");
  await expect.poll(() => nextWrites.length).toBe(1);
});

// Isolated source failure: deterministically reject after source assignment but
// before metadata without deleting media or changing Server authorization.
test("failed queue source cannot replace its saved position with reset zero before metadata", async ({page}, testInfo) => {
  await openAudio(page, "");
  const nextWrites: string[] = [];
  page.on("request", request => {if (new URL(request.url()).pathname === "/progress/next") nextWrites.push(request.postData() || "");});
  await page.route("https://audio.test/api/v1/items/next", route => route.fulfill({json: {
    item: {...queueItem("next"), progress: {seconds: 42, watched: false}}, profileId: "qa-viewer",
  }}));
  await page.locator("audio").evaluate(audio => Object.defineProperty(audio, "load", {value: () => {
    audio.currentTime = 0;
    queueMicrotask(() => audio.dispatchEvent(new Event("error")));
  }, configurable: true}));
  await startQueue(page);
  await page.getByRole("button", {name: "Next track", exact: true}).click();
  await expect(page.locator("audio")).toHaveAttribute("data-progress", "/progress/next");
  await expect(page.locator("audio")).toHaveAttribute("data-start", "42");
  await expect(page.locator("[data-audio-queue-status]")).toHaveAttribute("data-queue-failure", "media");
  await expect(page.getByRole("link", {name: "Current track details and actions", exact: true})).toHaveAttribute("href", "/watch/next");
  await expect(page.getByRole("button", {name: "Previous track", exact: true})).toBeEnabled();
  await testInfo.attach("failed-source-before-recovery", {body: JSON.stringify(await page.locator("audio").evaluate(audio => ({
    progressPath: audio.dataset.progress, savedSeconds: Number(audio.dataset.start), mediaSeconds: audio.currentTime,
    sourcePath: new URL(audio.src).pathname, failure: document.querySelector<HTMLElement>("[data-audio-queue-status]")!.dataset.queueFailure,
  }))), contentType: "application/json"});
  await page.locator("audio").dispatchEvent("pause");
  await page.getByRole("button", {name: "Previous track", exact: true}).click();
  await expect(page.locator("audio")).toHaveAttribute("data-progress", "/progress/track");
  await testInfo.attach("failed-source-progress-http-dispatches", {body: JSON.stringify(nextWrites), contentType: "application/json"});
  expect(nextWrites).toEqual([]);
});

test("late item authorization cannot cross a profile change", async ({page}) => {
  await openAudio(page, "");
  let lookup: import("@playwright/test").Route | undefined;
  const writes: string[] = [];
  page.on("request", request => {if (new URL(request.url()).pathname.startsWith("/progress/")) writes.push(request.url());});
  await page.route("https://audio.test/api/v1/items/next", route => {lookup = route;});
  await startQueue(page);
  await page.getByRole("button", {name: "Next track", exact: true}).click();
  await expect.poll(() => !!lookup).toBe(true);
  await page.evaluate(() => document.body.dataset.viewerProfile = "new-viewer");
  await lookup!.fulfill({json: {item: queueItem("next"), profileId: "qa-viewer"}});
  await expect(page.locator("[data-audio-queue-status]")).toHaveAttribute("data-queue-failure", "ownership");
  await expect(page.locator("audio")).toHaveAttribute("data-progress", "/progress/track");
  await expect(page.locator(".title-block h1")).toHaveText("First track");
  expect(writes).toEqual([]);
});

test("overlapping queue actions authorize and advance only once", async ({page}) => {
  await openAudio(page, "");
  let lookup: import("@playwright/test").Route | undefined, lookups = 0, writes = 0;
  page.on("request", request => {if (new URL(request.url()).pathname.startsWith("/progress/")) writes++;});
  await page.route("https://audio.test/api/v1/items/next", route => {lookups++; lookup = route;});
  await startQueue(page);
  await page.locator("[data-audio-next]").evaluate(button => {
    button.dispatchEvent(new MouseEvent("click", {bubbles: true}));
    button.dispatchEvent(new MouseEvent("click", {bubbles: true}));
  });
  await expect.poll(() => !!lookup).toBe(true);
  await lookup!.fulfill({json: {item: queueItem("next"), profileId: "qa-viewer"}});
  await expect(page.locator("audio")).toHaveAttribute("data-progress", "/progress/next");
  await expect(page.locator("audio")).toHaveAttribute("data-queue-position", "2");
  expect(lookups).toBe(1);
  expect(writes).toBe(1);
});

for (const saved of [true, false]) test(`offline queue transition respects its own progress journal: ${saved ? "saved" : "failed"}`, async ({page}) => {
  await openAudio(page, "");
  const serverWrites: string[] = [];
  page.on("request", request => {if (new URL(request.url()).pathname.startsWith("/progress/")) serverWrites.push(request.url());});
  await startQueue(page);
  await page.evaluate(saved => {
    document.querySelector("audio")!.dataset.offline = "true";
    Object.assign(window, {r08Offline: {saved: 0, detached: 0}, KinosailOfflineMedia: {
      saveProgress: async () => { (window as Window & {r08Offline: {saved: number}}).r08Offline.saved++; return {ok: saved}; },
      unbindProgress: () => { (window as Window & {r08Offline: {detached: number}}).r08Offline.detached++; },
    }});
  }, saved);
  await page.getByRole("button", {name: "Next track", exact: true}).click();
  await expect.poll(() => page.evaluate(() => (window as Window & {r08Offline: {saved: number}}).r08Offline.saved)).toBe(1);
  if (!saved) await expect(page.locator("[data-audio-queue-status]")).toHaveAttribute("data-queue-failure", "progress");
  await expect(page.locator("audio")).toHaveAttribute("data-progress", saved ? "/progress/next" : "/progress/track");
  expect(await page.evaluate(() => (window as Window & {r08Offline: {saved: number; detached: number}}).r08Offline)).toEqual({saved: 1, detached: saved ? 1 : 0});
  expect(serverWrites).toEqual([]);
});

test("failed new artwork clears the previous cover without reverting current metadata", async ({page}) => {
  await openAudio(page, "");
  await startQueue(page);
  await page.getByRole("button", {name: "Next track", exact: true}).click();
  await expect(page.locator("audio")).toHaveAttribute("data-progress", "/progress/next");
  await page.locator("[data-now-playing-artwork]").dispatchEvent("error");
  await expect(page.locator("[data-now-playing-artwork]")).toBeHidden();
  await expect(page.locator(".title-block h1")).toHaveText("Next track");
  expect(await page.evaluate(() => navigator.mediaSession.metadata?.title)).toBe("Next track");
});

for (const boundary of ["missing artwork", "unsupported system metadata", "cast owner", "room owner"]) {
  test(`queue preserves truthful local ownership with ${boundary}`, async ({page}) => {
    await openAudio(page, "");
    if (boundary === "missing artwork") await page.route("https://audio.test/api/v1/items/next", route => route.fulfill({json: {
      item: {...queueItem("next"), artwork: ""}, profileId: "qa-viewer",
    }}));
    if (boundary === "unsupported system metadata") await page.evaluate(() => Object.defineProperty(window, "MediaMetadata", {value: undefined}));
    await startQueue(page);
    if (boundary === "cast owner") await page.locator("audio").evaluate(audio => audio.dataset.castActive = "true");
    if (boundary === "room owner") await page.locator("audio").evaluate(audio => audio.dataset.room = "existing-room");
    if (boundary === "cast owner" || boundary === "room owner") {
      await expect(page.getByRole("button", {name: "Next track", exact: true})).toBeDisabled();
      await page.locator("[data-audio-next]").evaluate(button => button.dispatchEvent(new MouseEvent("click", {bubbles: true})));
    } else await page.getByRole("button", {name: "Next track", exact: true}).click();
    if (boundary === "cast owner" || boundary === "room owner") {
      await expect(page.locator("audio")).toHaveAttribute("data-progress", "/progress/track");
      await expect(page.locator(".title-block h1")).toHaveText("First track");
    } else {
      await expect(page.locator("audio")).toHaveAttribute("data-progress", "/progress/next");
      await expect(page.locator(".title-block h1")).toHaveText("Next track");
      if (boundary === "missing artwork") {
        await expect(page.locator("[data-now-playing-artwork]")).toBeHidden();
        expect(await page.evaluate(() => navigator.mediaSession.metadata?.artwork)).toEqual([]);
      }
    }
  });
}
