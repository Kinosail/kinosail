import { expect, test, type Page } from "@playwright/test";
import { playerSource } from "./static-sources";

// Isolated fault injection: a populated Server cannot naturally return a forged
// cross-profile queue projection or deterministically delay a source-load pause.
const queueItem = (id: string) => ({id, kind: "audio", title: id === "track" ? "First track" : "Next track",
  artist: id === "track" ? "First artist" : "Next artist", album: "Fictional album", track: id === "track" ? 1 : 2,
  stream: `/media/${id}`, artwork: `/art/${id}`, progress: {seconds: 0, watched: false}});
const queueURL = "https://audio.test/api/v1/audio/track/queue";

async function openAudio(page: Page, policy: string, homeAssistant = false) {
  await page.addInitScript((value) => localStorage.setItem("kinosail.playback-policy-v2", value), policy);
  await page.route("https://audio.test/**", (route) => {
    const path = new URL(route.request().url()).pathname;
    if (path === "/watch/track") return route.fulfill({ contentType: "text/html", body: `
      <!doctype html><meta name="kinosail-csrf" content="session-csrf-token">
      <title>First track · Kinosail Player</title><body data-viewer-profile="qa-viewer">
      <div class="media-stage"><img class="viewer" data-now-playing-artwork src="/art/track" alt="">
      <audio controls src="/media/track?playbackSession=audio-policy-session" data-title="First track" data-artist="First artist"
      data-album="Fictional album" data-artwork="/art/track" data-kind="audio" data-playback-session="audio-policy-session"
      data-playback-trace="/api/v1/items/track/playback-events" data-cast-api="/api/v1/items/track/cast"
      data-progress="/progress/track" data-queue="/api/v1/audio/track/queue" data-home-assistant="${homeAssistant}"></audio></div>
      <div class="title-block"><h1 data-now-playing-title>First track</h1><p class="title-byline" data-now-playing-byline>First artist · Fictional album · Track 1</p></div>
      <div data-audio-queue-controls><button type="button" data-audio-previous disabled>Previous track</button>
      <span role="status" aria-live="polite" data-audio-queue-status></span><button type="button" data-audio-next disabled>Next track</button></div>
      <div class="primary-player-actions"><form action="/watched/track"><button>Mark watched</button></form></div>
      <div data-current-track-actions hidden><a data-current-track-details>Current track details and actions</a></div>
      <div data-progress-notice hidden><span role="status" data-progress-status></span><button data-progress-retry>Retry saving position</button>
      <button data-progress-continue hidden>Continue without saving</button></div></body>` });
    if (path === "/api/v1/audio/track/queue") return route.fulfill({json: {items: [queueItem("track"), queueItem("next")]}});
    if (path === "/api/v1/items/track" || path === "/api/v1/items/next") return route.fulfill({json: {item: queueItem(path.split("/").at(-1)!), profileId: "qa-viewer"}});
    return route.fulfill({ status: 204 });
  });
  await page.goto("https://audio.test/watch/track");
  await page.evaluate(() => {
    const media = document.querySelector("audio")!;
    media.addEventListener("error", (event) => event.stopImmediatePropagation(), true);
    Object.defineProperties(media, {
      currentTime: { value: 12, writable: true },
      duration: { value: 120 },
      readyState: { value: HTMLMediaElement.HAVE_ENOUGH_DATA },
      play: { value: async () => {} },
      load: { value: () => queueMicrotask(() => media.dispatchEvent(new Event("loadedmetadata"))), configurable: true },
    });
  });
}

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

async function startQueue(page: Page) {
  await page.addScriptTag({content: playerSource});
  await expect(page.getByRole("button", {name: "Next track", exact: true})).toBeEnabled();
}

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
  await expect.poll(() => nextWrites.length).toBe(1);
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
    await page.getByRole("button", {name: "Next track", exact: true}).click();
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
