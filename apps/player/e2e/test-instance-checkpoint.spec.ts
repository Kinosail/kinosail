import { expect, test, type Page } from "@playwright/test";
import { execFileSync } from "node:child_process";
import { readFile } from "node:fs/promises";
import { configureTestInstance, login } from "./test-instance-helpers";

configureTestInstance();
const phase = process.env.KINOSAIL_CHECKPOINT_SOURCE || "candidate";
if (!["deployed", "current", "candidate"].includes(phase)) throw new Error("Unknown checkpoint source phase");
const pinned = {deployed: "5f91114d16e7c049e0da1d9c119653a6e8161b90", current: "ee567e9b9d6b4dc4211055e1f2ef968c5ce63209"};
const source = await readFile(new URL("../../../packages/webassets/static/player-progress.js", import.meta.url), "utf8");
const historical = phase === "candidate" ? "" : execFileSync("git", ["show", `${pinned[phase as keyof typeof pinned]}:packages/webassets/static/player-progress.js`], {encoding: "utf8"});
test.use({serviceWorkers: phase === "candidate" ? "allow" : "block"});
test.beforeEach(async ({page}) => {
  if (historical) await page.route(/\/static\/player\.js(?:\?.*)?$/, async route => {
    const response = await route.fetch(), body = await response.text();
    expect(body).toContain(source);
    await route.fulfill({response, body: body.replace(source, historical)});
  });
});

async function checkpoint(page: Page, id: string) {
  const response = await page.request.get(`/api/v1/items/${id}`);
  expect(response.status()).toBe(200);
  const body = await response.json();
  expect(body.item.progress).toBeTruthy();
  return body.item.progress as {seconds: number; revision: number};
}


async function observeExit(page: Page, id: string) {
  await page.evaluate(item => {
    const video = document.querySelector("video")!;
    const key = "kinosail:checkpoint-exit-observation";
    const observations: Array<Record<string, unknown>> = [];
    const record = (stage: string, detail: Record<string, unknown> = {}) => {
      observations.push({stage, elapsedMs: Math.round(performance.now()), seconds: video.currentTime,
        readyState: video.readyState, paused: video.paused, ended: video.ended, seeking: video.seeking,
        visibility: document.visibilityState, ...detail});
      sessionStorage.setItem(key, JSON.stringify(observations.slice(-32)));
    };
    sessionStorage.removeItem(key);
    const originalFetch = window.fetch;
    window.fetch = function(input, init) {
      const request = input instanceof Request ? input : null;
      const matches = new URL(request?.url || String(input), location.href).pathname === `/progress/${item}`
        && (init?.method || request?.method) === "POST";
      const body = init?.body instanceof URLSearchParams ? init.body : new URLSearchParams();
      if (matches) record("progress-dispatch", {submittedSeconds: Number(body.get("seconds")),
        revision: Number(body.get("revision")), keepalive: init?.keepalive === true});
      const flight = originalFetch.call(this, input, init);
      if (matches) void flight.then(response => record("progress-response", {status: response.status}),
        () => record("progress-rejected"));
      return flight;
    };
    addEventListener("pagehide", event => record("pagehide-before-player", {persisted: event.persisted}), {capture: true});
    addEventListener("pagehide", event => record("pagehide-after-player", {persisted: event.persisted}));
    document.addEventListener("visibilitychange", () => record("visibilitychange"), {capture: true});
    video.addEventListener("kinosail:page-exit", () => record("player-exit"), {capture: true});
    for (const event of ["pause", "emptied"]) video.addEventListener(event, () => record(event), {capture: true});
    document.querySelector('a.back[href="/"]')?.addEventListener("click", () => record("library-click"), {capture: true});
    record("observation-start");
  }, id);
}

async function openMovie(page: Page) {
  await login(page);
  await page.goto("/?q=Checkpoint%20Example&view=movies");
  const card = page.locator('a.card[href^="/watch/"]').filter({hasText: "Checkpoint Example"}).first();
  await expect(card).toBeVisible();
  const watch = (await card.getAttribute("href"))!;
  await card.click();
  await page.waitForURL(url => url.pathname === watch);
  const media = page.locator("video"), id = watch.split("/").at(-1)!;
  await expect.poll(() => media.evaluate((video: HTMLVideoElement) => video.readyState)).toBeGreaterThanOrEqual(2);
  const duration = await media.evaluate((video: HTMLVideoElement) => video.duration);
  expect(duration).toBeGreaterThan(20);
  await media.evaluate((video: HTMLVideoElement) => video.play());
  await expect.poll(() => media.evaluate((video: HTMLVideoElement) => video.getVideoPlaybackQuality().totalVideoFrames)).toBeGreaterThan(2);
  await expect.poll(() => media.evaluate((video: HTMLVideoElement) => video.currentTime)).toBeGreaterThan(0.2);
  await media.evaluate((video: HTMLVideoElement) => video.pause());
  const paused = await media.evaluate((video: HTMLVideoElement) => video.currentTime);
  await expect.poll(async () => Math.abs((await checkpoint(page, id)).seconds - paused)).toBeLessThan(0.1);
  return {watch, media, id, paused, duration};
}

test("completed paused seek persists before Library navigation and resumes actual movie frames", {tag: "@smoke"}, async ({page}, testInfo) => {
  const {watch, media, id, paused, duration} = await openMovie(page);
  const delta = paused + 10 < duration - 10 ? 10 : -10;
  const target = paused + delta;
  expect(target).toBeGreaterThanOrEqual(0);
  expect(target).toBeLessThan(duration - 10);
  await page.getByRole("region", {name: "Video player", exact: true}).press(delta > 0 ? "ArrowRight" : "ArrowLeft");
  await expect.poll(() => media.evaluate((video: HTMLVideoElement) => video.seeking)).toBe(false);
  await expect.poll(() => media.evaluate((video: HTMLVideoElement, seconds) => Math.abs(video.currentTime - seconds), target)).toBeLessThan(0.1);
  await expect(media).toHaveJSProperty("paused", true);
  await expect.poll(() => media.evaluate((video: HTMLVideoElement) => video.readyState)).toBeGreaterThanOrEqual(2);
  await page.screenshot({path: testInfo.outputPath("completed-paused-seek.png"), fullPage: true});
  const beforeLeaving = await checkpoint(page, id);
  await testInfo.attach("paused-seek-checkpoint", {body: JSON.stringify({phase, paused, target, beforeLeaving}), contentType: "application/json"});
  await expect.poll(async () => Math.abs((await checkpoint(page, id)).seconds - target), {timeout: 3000}).toBeLessThan(0.1);
  await testInfo.attach("accepted-seek-checkpoint", {body: JSON.stringify({phase, target, saved: await checkpoint(page, id)}), contentType: "application/json"});
  await page.getByRole("link", {name: "Library", exact: true}).click();
  await expect(page).toHaveURL(/\/$/);
  await page.goto(watch);
  await expect.poll(async () => Math.abs(Number(await page.locator("video").getAttribute("data-start")) - target)).toBeLessThan(0.1);
  await expect.poll(() => page.locator("video").evaluate((video: HTMLVideoElement) => video.readyState)).toBeGreaterThanOrEqual(2);
  await expect.poll(() => page.locator("video").evaluate((video: HTMLVideoElement) => video.currentTime)).toBeGreaterThanOrEqual(target - 0.1);
  await page.locator("video").evaluate((video: HTMLVideoElement) => video.play());
  await expect.poll(() => page.locator("video").evaluate((video: HTMLVideoElement) => video.currentTime)).toBeGreaterThan(target + 0.2);
  await expect.poll(() => page.locator("video").evaluate((video: HTMLVideoElement) => video.getVideoPlaybackQuality().totalVideoFrames)).toBeGreaterThan(2);
  await page.screenshot({path: testInfo.outputPath("reentered-moving-movie.png"), fullPage: true});
});

test("Library exit checkpoints actual playing time before teardown without reset-position overwrite", {tag: "@smoke"}, async ({page}, testInfo) => {
  const {watch, media, id} = await openMovie(page);
  await media.evaluate((video: HTMLVideoElement) => video.play());
  const first = await media.evaluate((video: HTMLVideoElement) => video.currentTime);
  await expect.poll(() => media.evaluate((video: HTMLVideoElement) => video.currentTime)).toBeGreaterThan(first + 0.5);
  await observeExit(page, id);
  const leaveAt = await media.evaluate((video: HTMLVideoElement) => video.currentTime);
  const beforeExit = await checkpoint(page, id);
  const writes: Array<{seconds: number; revision: number}> = [];
  page.on("request", request => {
    if (new URL(request.url()).pathname !== `/progress/${id}` || request.method() !== "POST") return;
    const form = new URLSearchParams(request.postData() || "");
    writes.push({seconds: Number(form.get("seconds")), revision: Number(form.get("revision"))});
  });
  try {
    await page.getByRole("link", {name: "Library", exact: true}).click();
    await expect(page).toHaveURL(/\/$/);
    await expect.poll(async () => (await checkpoint(page, id)).seconds, {timeout: 3000}).toBeGreaterThanOrEqual(leaveAt - 0.1);
    await page.waitForTimeout(300);
    const saved = await checkpoint(page, id);
    expect(saved.seconds).toBeGreaterThanOrEqual(leaveAt - 0.1);
    expect(saved.revision).toBeGreaterThan(beforeExit.revision);
    expect(writes.every(write => write.seconds >= leaveAt - 0.1)).toBe(true);
    await testInfo.attach("exit-checkpoint", {body: JSON.stringify({phase, leaveAt, beforeExit, saved, browserObservedWrites: writes, observationLimit: "Chromium may omit pagehide keepalive from page request events; actual persisted position/revision are authoritative."}), contentType: "application/json"});
  } finally {
    const lifecycle = await page.evaluate(() => JSON.parse(sessionStorage.getItem("kinosail:checkpoint-exit-observation") || "[]"));
    await testInfo.attach("exit-lifecycle-observation", {body: JSON.stringify({phase, revision: process.env.GITHUB_SHA || "",
      browser: page.context().browser()?.version(), leaveAt, beforeExit, lifecycle}), contentType: "application/json"});
  }
  await page.goto(watch);
  await expect.poll(async () => Number(await page.locator("video").getAttribute("data-start"))).toBeGreaterThanOrEqual(leaveAt - 0.1);
  await page.screenshot({path: testInfo.outputPath("exit-checkpoint-reentry.png"), fullPage: true});
});
