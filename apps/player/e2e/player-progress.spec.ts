import { expect, test, type Page, type Route } from "@playwright/test";
import { readFile } from "node:fs/promises";
import {execFileSync} from "node:child_process";
import {createHash} from "node:crypto";

// Isolated HTTP failure/ordering coverage; see engineering/qa/2026-10-04-r03-progress.
const frozenRevision = process.env.KINOSAIL_PROGRESS_FROZEN_REVISION;
if (frozenRevision && !/^[a-f0-9]{40}$/.test(frozenRevision)) throw new Error("Frozen progress source must name an exact commit.");
const source = frozenRevision ? execFileSync("git", ["show", `${frozenRevision}:packages/webassets/static/player-progress.js`], {encoding: "utf8"}) :
  await readFile(new URL("../../../packages/webassets/static/player-progress.js", import.meta.url), "utf8");
const queueSource = frozenRevision ? "" : await readFile(new URL("../../../packages/webassets/static/player-audio-queue.js", import.meta.url), "utf8");
const failure = "Your latest position is not saved. Retry while this page is open.";
const requests: Array<{ body: URLSearchParams; route: Route }> = [];
let status = 204;
let aborted = false;
let deferred = false;

test.beforeEach(async ({ page }, testInfo) => {
  status = 204; aborted = false; deferred = false; requests.length = 0;
  await page.clock.install();
  await page.route("**/progress/**", async (route) => {
    requests.push({ body: new URLSearchParams(route.request().postData() || ""), route });
    if (deferred) return;
    if (aborted) return route.abort("failed");
    await route.fulfill({ status, headers: { "X-Request-ID": "qa-request-03" }, body: status === 204 ? "" : "private-server-error?token=synthetic" });
  });
  await page.route("https://127.0.0.1:38127/", route => route.fulfill({ contentType: "text/html", body: `
    <!doctype html><html lang="en"><head><meta charset="utf-8"><title>R03 fixture</title></head>
    <body data-viewer-profile="qa-viewer"><main class="player-shell">
      <video data-progress="/progress/movie?playbackToken=synthetic" data-start="0"></video>
      <div class="player-progress-notice" data-progress-notice hidden>
        <span role="status" aria-live="polite" data-progress-status></span>
        <button class="quiet" type="button" data-progress-retry>Retry saving position</button>
        <button class="quiet" type="button" data-progress-continue hidden>Continue without saving</button>
      </div>
      <label>Audio track <select data-audio-track><option value="0">Original</option><option value="1">Other</option></select></label><small data-audio-status></small>
    </main></body></html>` }));
  await page.route("**/watch/next", route => route.fulfill({ contentType: "text/html", body: "<h1>Next episode</h1>" }));
  await page.route("**/api/v1/audio/movie/queue", route => route.fulfill({json: {items: [
    {id: "movie", kind: "audio", title: "First song", stream: "/media/movie"},
    {id: "next", kind: "audio", title: "Next song", stream: "/media/next"},
  ]}}));
  await page.route("**/api/v1/items/next", route => route.fulfill({json: {profileId: "qa-viewer", item: {id: "next", kind: "audio", title: "Next song", stream: "/media/next"}}}));
  await page.goto("/");
  if (testInfo.title.includes("audio queue")) await page.locator("video").evaluate(media => media.dataset.queue = "/api/v1/audio/movie/queue");
  await page.addScriptTag({ content: `
    const player = document.querySelector('video'), csrf = 'synthetic-csrf', playbackSession = 'qa-session-03';
    let playbackPreparation, preparationPausePending = 0;
    let playbackTraceMethod = 'direct', playbackTimelineOffset = 0;
    const setPlayerTime = seconds => player.currentTime = seconds;
    const withPlaybackSession = source => source + '?playbackSession=' + playbackSession, updateNowPlaying = () => {};
    const requestPlay = () => window.holdQueuePlay ? new Promise(resolve => window.finishQueuePlay = resolve) : Promise.resolve();
    const playerStorage = {get: () => '', set: () => {}};
    const playbackTrace = () => {}, flushPlaybackTrace = () => {};
    let position = 42, paused = true;
    Object.defineProperties(player, {currentTime: {get: () => position, set: value => position = value}, duration: {value: 100}, readyState: {value: 4}, paused: {get: () => paused}, load: {value: () => queueMicrotask(() => player.dispatchEvent(new Event('loadedmetadata')))}});
    Object.assign(window, {setPaused: value => paused = value, prepare: value => playbackPreparation = value});
  ` + source + queueSource });
});

async function pauseAt(page: Page, seconds: number) {
  await page.locator("video").evaluate((media: HTMLVideoElement, seconds) => { media.currentTime = seconds; media.dispatchEvent(new Event("pause")); }, seconds);
}

for (const mode of ["HTTP 503", "network rejection"]) test(`failed pause save is visible and retries the same revision: ${mode}`, {tag: "@smoke"}, async ({ page }) => {
  status = 503; aborted = mode === "network rejection";
  await pauseAt(page, 42);
  await expect(page.locator("[data-progress-status]")).toHaveText(failure);
  await expect(page.locator("[data-progress-notice]")).toBeVisible();
  const first = requests[0].body;
  expect(first.get("seconds")).toBe("42");
  status = 204; aborted = false;
  await page.getByRole("button", { name: "Retry saving position", exact: true }).click();
  await expect(page.locator("[data-progress-notice]")).toBeHidden();
  expect(requests[1].body.toString()).toBe(first.toString());
});

test("latest pending revision wins over late failures and successes", {tag: "@smoke"}, async ({ page }) => {
  deferred = true;
  await pauseAt(page, 42);
  await expect.poll(() => requests.length).toBe(1);
  await pauseAt(page, 60);
  await requests[0].route.fulfill({ status: 503 });
  await expect.poll(() => requests.length).toBe(2);
  await requests[1].route.fulfill({ status: 503 });
  await expect(page.locator("[data-progress-status]")).toHaveText(failure);
  deferred = false;
  await page.getByRole("button", { name: "Retry saving position", exact: true }).click();
  await expect(page.locator("[data-progress-notice]")).toBeHidden();
  expect(requests[2].body.get("seconds")).toBe("60");
  expect(Number(requests[2].body.get("revision"))).toBeGreaterThan(Number(requests[0].body.get("revision")));
});

test("ended failure holds next navigation and offers an explicit continue", async ({ page }) => {
  status = 503;
  await page.locator("video").evaluate(media => { media.dataset.next = "/watch/next"; media.dispatchEvent(new Event("ended")); });
  await expect(page.locator("[data-progress-status]")).toHaveText("Watched status is not saved. Retry or continue without saving.");
  expect(new URL(page.url()).pathname).toBe("/");
  await page.getByRole("button", { name: "Continue without saving", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Next episode" })).toBeVisible();
});

test("ended Retry saves watched state before next navigation", {tag: "@smoke"}, async ({ page }) => {
  status = 503;
  await page.locator("video").evaluate(media => { media.dataset.next = "/watch/next"; media.dispatchEvent(new Event("ended")); });
  await expect(page.getByRole("button", { name: "Continue without saving", exact: true })).toBeVisible();
  await pauseAt(page, 100);
  status = 204;
  await page.getByRole("button", { name: "Retry saving position", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Next episode" })).toBeVisible();
  expect(requests.at(-1)!.body.get("watched")).toBe("true");
  expect(requests.at(-1)!.body.get("seconds")).toBe("0");
});

for (const code of [401, 403, 400, 404]) test(`HTTP ${code} does not repeat policy or authentication failures`, async ({ page }) => {
  status = code;
  await pauseAt(page, 42);
  await expect(page.locator("[data-progress-notice]")).toBeVisible();
  await expect(page.getByRole("button", { name: "Retry saving position", exact: true })).toBeHidden();
  expect(await page.locator("[data-progress-status]").textContent()).not.toContain("private-server-error");
  await page.evaluate(() => (window as Window & {setPaused(value: boolean): void}).setPaused(false));
  await page.clock.fastForward(30_000);
  expect(requests.length).toBe(1);
});

for (const owner of ["profile", "item", "cast", "offline"]) test(`pending local save cannot cross ${owner} ownership`, async ({ page }) => {
  status = 503;
  await pauseAt(page, 42);
  await expect(page.locator("[data-progress-notice]")).toBeVisible();
  await page.evaluate(owner => {
    const media = document.querySelector("video")!;
    if (owner === "profile") document.body.dataset.viewerProfile = "another-viewer";
    if (owner === "item") media.dataset.progress = "/progress/next";
    if (owner === "cast") media.dataset.castActive = "true";
    if (owner === "offline") media.dataset.offline = "true";
  }, owner);
  await page.locator("[data-progress-retry]").evaluate(button => (button as HTMLButtonElement).click());
  await expect(page.locator("[data-progress-notice]")).toBeHidden();
  expect(requests.length).toBe(1);
});

test("preparation and cast never emit local progress; offline uses its own journal", async ({ page }) => {
  await page.evaluate(() => (window as Window & {prepare(value: object): void}).prepare({}));
  await pauseAt(page, 42);
  await page.evaluate(() => { (window as Window & {prepare(value: unknown): void}).prepare(undefined); document.querySelector("video")!.dataset.castActive = "true"; });
  await pauseAt(page, 42);
  expect(requests.length).toBe(0);
  await page.evaluate(() => {
    const media = document.querySelector("video")!;
    delete media.dataset.castActive; media.dataset.offline = "true";
    Object.assign(window, {offlineCalls: 0, KinosailOfflineMedia: {saveProgress: async () => { (window as Window & {offlineCalls: number}).offlineCalls++; return {ok: true}; }}});
  });
  await pauseAt(page, 42);
  await expect.poll(() => page.evaluate(() => (window as Window & {offlineCalls: number}).offlineCalls)).toBe(1);
  expect(requests.length).toBe(0);
});

test("audio track change stays on the page when progress fails", async ({ page }) => {
  status = 503;
  await page.getByLabel("Audio track").selectOption("1");
  await expect(page.locator("[data-audio-status]")).toHaveText("Could not save your position. Try changing the audio again.");
  await expect(page.getByLabel("Audio track")).toBeEnabled();
  await expect(page.locator("[data-progress-notice]")).toBeVisible();
  expect(new URL(page.url()).search).toBe("");
});

test("timeout gives a bounded Retry state without blocking viewing", async ({ page }) => {
  deferred = true;
  await pauseAt(page, 42);
  await expect.poll(() => requests.length).toBe(1);
  await page.clock.fastForward(10_000);
  await expect(page.locator("[data-progress-status]")).toHaveText(failure);
  await expect(page.getByRole("button", { name: "Retry saving position", exact: true })).toBeEnabled();
});

test("periodic, hidden-page and page-close saves inspect failures and preserve latest progress", async ({ page }) => {
  status = 503;
  await page.evaluate(() => (window as Window & {setPaused(value: boolean): void}).setPaused(false));
  await page.clock.fastForward(10_000);
  await expect(page.locator("[data-progress-status]")).toHaveText(failure);
  await page.locator("video").evaluate((media: HTMLVideoElement) => media.currentTime = 60);
  await page.evaluate(() => { Object.defineProperty(document, "visibilityState", {configurable: true, value: "hidden"}); document.dispatchEvent(new Event("visibilitychange")); });
  await expect.poll(() => requests.length).toBe(2);
  expect(requests[1].body.get("seconds")).toBe("60");
  await page.locator("video").evaluate((media: HTMLVideoElement) => media.currentTime = 65);
  await page.evaluate(() => dispatchEvent(new Event("pagehide")));
  await expect.poll(() => requests.length).toBe(3);
  expect(requests[2].body.get("seconds")).toBe("65");
  const stored = await page.evaluate(() => ({local: Object.keys(localStorage), session: Object.keys(sessionStorage)}));
  expect(stored).toEqual({local: [], session: []});
});

test("HTTP failure diagnostics carry bounded session/revision/request correlation without secrets", async ({ page }) => {
  status = 503;
  await pauseAt(page, 42);
  await expect(page.locator("[data-progress-status]")).toHaveText(failure);
  expect(requests[0].route.request().headers()["x-playback-session"]).toBe("qa-session-03");
  const projection = await page.locator("[data-progress-status]").evaluate(element => ({...(element as HTMLElement).dataset}));
  expect(projection).toMatchObject({progressFailure: "server", progressSession: "qa-session-03", progressRevision: "1", progressRequestId: "qa-request-03"});
  const diagnostic = JSON.stringify(projection);
  expect(diagnostic).toContain("qa-session-03");
  expect(diagnostic).toContain("qa-request-03");
  expect(diagnostic).not.toMatch(/synthetic|private-server-error|playbackToken|\/progress\//);
});

test("bounded positions reject invalid media time without making a request", async ({ page }) => {
  await pauseAt(page, -1);
  await expect(page.locator("[data-progress-notice]")).toBeVisible();
  expect(requests.length).toBe(0);
});

test("page-close dispatch sends the latest position while an older request is in flight", {tag: "@smoke"}, async ({ page }) => {
  deferred = true;
  await pauseAt(page, 42);
  await expect.poll(() => requests.length).toBe(1);
  await page.locator("video").evaluate((media: HTMLVideoElement) => media.currentTime = 60);
  await page.evaluate(() => dispatchEvent(new Event("pagehide")));
  await expect.poll(() => requests.length, {timeout: 1500}).toBe(2);
  expect(requests[1].body.get("seconds")).toBe("60");
  await requests[1].route.fulfill({status: 204});
  await requests[0].route.fulfill({status: 503});
  await expect(page.locator("[data-progress-notice]")).toBeHidden();
});

test("watched failure on the last title survives later pause and hiding events", async ({ page }) => {
  status = 503;
  await page.locator("video").dispatchEvent("ended");
  await expect(page.locator("[data-progress-status]")).toHaveText("Watched status is not saved. Retry while this page is open.");
  await expect(page.locator("[data-progress-continue]")).toBeHidden();
  await pauseAt(page, 100);
  await page.evaluate(() => dispatchEvent(new Event("pagehide")));
  await expect.poll(() => requests.length).toBe(2);
  expect(requests[1].body.get("watched")).toBe("true");
  expect(requests[1].body.get("revision")).toBe(requests[0].body.get("revision"));
});

test("ended policy failure still offers Continue without saving", async ({ page }) => {
  status = 403;
  await pauseAt(page, 42);
  await expect(page.locator("[data-progress-notice]")).toBeVisible();
  await page.locator("video").evaluate(media => { media.dataset.next = "/watch/next"; media.dispatchEvent(new Event("ended")); });
  await expect(page.getByRole("button", { name: "Continue without saving", exact: true })).toBeVisible();
  expect(requests.length).toBe(1);
  await page.getByRole("button", { name: "Continue without saving", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Next episode" })).toBeVisible();
});

test("localized recovery notices use Go-rendered copy", async ({ page }) => {
  status = 503;
  await page.locator("[data-progress-status]").evaluate(element => (element as HTMLElement).dataset.unsaved = "La posición todavía no está guardada.");
  await pauseAt(page, 42);
  await expect(page.locator("[data-progress-status]")).toHaveText("La posición todavía no está guardada.");
});

test("audio track change cannot mistake an unexpected successful HTTP page for a progress acknowledgement", async ({ page }) => {
  status = 200;
  await page.getByLabel("Audio track").selectOption("1");
  await expect(page.locator("[data-progress-status]")).toHaveText("Your position is not saved. Reload this page to reconnect to Kinosail Server.");
  await expect(page.getByLabel("Audio track")).toBeEnabled();
  expect(new URL(page.url()).search).toBe("");
});

test("audio queue does not drop a failed watched save when it advances", async ({ page }) => {
  status = 503;
  await page.waitForFunction("audioQueue.length === 1");
  await page.locator("video").dispatchEvent("ended");
  await expect(page.getByRole("button", {name: "Continue without saving", exact: true})).toBeVisible();
  await expect(page.locator("video")).toHaveAttribute("data-progress", "/progress/movie?playbackToken=synthetic");
  status = 204;
  await page.getByRole("button", {name: "Retry saving position", exact: true}).click();
  await expect(page.locator("video")).toHaveAttribute("data-progress", "/progress/next");
  expect(requests.at(-1)!.body.get("watched")).toBe("true");
  await expect(page.locator("[data-progress-notice]")).toBeHidden();
});

test("audio queue saves a new-track pause while final watched continuation still settles", async ({page}, testInfo) => {
  await testInfo.attach("progress-source-provenance", {body: JSON.stringify({revision: frozenRevision || "working-tree",
    senderAndQueueSHA256: createHash("sha256").update(source + queueSource).digest("hex")}), contentType: "application/json"});
  try {
    await page.waitForFunction("audioQueue.length === 1");
    await page.evaluate(() => Object.assign(window, {holdQueuePlay: true}));
    await page.locator("video").dispatchEvent("ended");
    await expect(page.locator("video")).toHaveAttribute("data-progress", "/progress/next");
    await expect.poll(() => page.locator("video").evaluate((media: HTMLVideoElement) => new URL(media.src).pathname)).toBe("/media/next");
    await page.waitForFunction("typeof window.finishQueuePlay === 'function'");
    await pauseAt(page, 3);
    await page.evaluate(() => (window as Window & {finishQueuePlay(): void}).finishQueuePlay());
    await testInfo.attach("queued-new-track-pause", {body: JSON.stringify(await page.evaluate("({progressPath: player.dataset.progress, pendingSeconds: pendingProgress?.seconds, pendingRevision: pendingProgress?.revision, flightSettled: progressFlight === undefined})")), contentType: "application/json"});
    await expect.poll(() => requests.length, {timeout: 1500}).toBe(2);
    expect(new URL(requests[1].route.request().url()).pathname).toBe("/progress/next");
    expect(requests[1].body.get("seconds")).toBe("3");
    expect(requests[1].body.get("watched")).toBeNull();
  } finally {
    await page.evaluate(() => (window as Window & {finishQueuePlay?: () => void}).finishQueuePlay?.()).catch(() => {});
  }
});
