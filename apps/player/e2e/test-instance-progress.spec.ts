import { expect, test, type Page, type TestInfo } from "@playwright/test";
import { execFileSync } from "node:child_process";
import { createHash } from "node:crypto";
import { readFile } from "node:fs/promises";
import { configureTestInstance, firstPlayable, login } from "./test-instance-helpers";
import AxeBuilder from "@axe-core/playwright";

configureTestInstance();
const baseline = process.env.KINOSAIL_R03_BASELINE === "1";
const currentSource = await readFile(new URL("../../../packages/webassets/static/player-progress.js", import.meta.url), "utf8");
const baselineSource = baseline ? execFileSync("git", ["show", "2e9ede47:packages/webassets/static/player-progress.js"], {encoding: "utf8"}) : "";
// Baseline asset replay must bypass the candidate's service-worker precache.
test.use({serviceWorkers: baseline ? "block" : "allow"});
async function serverProgress(page: Page, id: string) {
  const response = await page.request.get(`/api/v1/items/${id}`);
  expect(response.status()).toBe(200);
  const body = await response.json();
  expect(body.item).toHaveProperty("progress");
  expect(typeof body.item.progress).toBe("object");
  return body.item.progress;
}

// Real Go markup and the delivered CSS are required: bare progress fixtures miss
// the mobile primary-player-actions rule overriding the notice's hidden state.
async function inspectNotice(page: Page, testInfo: TestInfo, state: string, visible: boolean) {
  const projections = [];
  for (const viewport of [{width: 360, height: 800}, {width: 390, height: 844},
    {width: 700, height: 900}, {width: 701, height: 900},
    {width: 1440, height: 900}, {width: 1920, height: 1080}]) {
    await page.setViewportSize(viewport);
    const notice = page.locator("[data-progress-notice]");
    const projection = await notice.evaluate(element => ({
      hidden: (element as HTMLElement).hidden, display: getComputedStyle(element).display,
      height: element.getBoundingClientRect().height, busy: element.getAttribute("aria-busy"),
    }));
    projections.push({state, viewport, ...projection});
    if (state === "pending") {
      expect(projection.busy).toBe("true");
      await expect(page.locator("[data-progress-status]")).toHaveText("Saving progress…");
      await expect(page.getByRole("button", {name: "Retry saving position", exact: true})).toBeDisabled();
    }
    await testInfo.attach(`${state}-${viewport.width}-computed`, {body: JSON.stringify(projection), contentType: "application/json"});
    await page.screenshot({path: testInfo.outputPath(`${state}-${viewport.width}.png`), fullPage: true});
    if (visible) {
      await expect(notice).toBeVisible();
      const retry = page.getByRole("button", {name: "Retry saving position", exact: true});
      await expect(retry).toBeVisible();
      expect((await retry.boundingBox())!.height).toBeGreaterThanOrEqual(44);
    } else {
      expect(projection).toMatchObject({hidden: true, display: "none", height: 0});
      await expect(notice).toBeHidden();
      await expect(page.getByRole("button", {name: "Retry saving position", exact: true})).toBeHidden();
    }
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  }
  await testInfo.attach(`${state}-responsive-notice`, {body: JSON.stringify(projections), contentType: "application/json"});
}

test.beforeEach(async ({page}) => {
  if (baseline) await page.route(/\/static\/player\.js(?:\?.*)?$/, async route => {
    const response = await route.fetch();
    const body = await response.text();
    expect(body).toContain(currentSource);
    await route.fulfill({response, body: body.replace(currentSource, baselineSource)});
  });
});

test("real Server rejects invalid progress without changing stored state and web reports the rejection", {tag: "@smoke"}, async ({page}, testInfo) => {
  await login(page);
  const watch = await firstPlayable(page);
  await page.goto(watch);
  const assetURL = await page.locator('script[src*="/static/player.js"]').getAttribute("src");
  expect(assetURL).toMatch(/^\/static\/player\.js\?v=[a-f0-9]{64}$/);
  const asset = await page.request.get(assetURL!);
  expect(asset.status()).toBe(200);
  expect(asset.headers()["cache-control"]).toContain("immutable");
  const delivered = await asset.body();
  expect(new URL(assetURL!, page.url()).searchParams.get("v")).toBe(createHash("sha256").update(delivered).digest("hex"));
  expect(delivered.toString()).toContain(currentSource);
  const media = page.locator("video");
  await expect.poll(() => media.evaluate((video: HTMLVideoElement) => video.readyState)).toBeGreaterThanOrEqual(2);
  await media.evaluate((video: HTMLVideoElement) => video.pause());
  await expect.poll(() => media.evaluate((video: HTMLVideoElement) => video.paused)).toBe(true);
  // Settle the pause's valid save before injecting a rejected public request.
  await page.waitForTimeout(300);
  const id = watch.split("/").at(-1)!;
  const before = await serverProgress(page, id);
  await media.evaluate(video => video.dataset.progress = video.dataset.progress!.split("?")[0] + "?playbackToken=invalid");
  const rejected = page.waitForResponse(response => new URL(response.url()).pathname === `/progress/${id}` && response.status() === 400);
  // Advance the position so deduplication cannot suppress the rejected request.
  await media.evaluate((video: HTMLVideoElement) => { video.currentTime = Math.min(video.duration, video.currentTime + 1); video.dispatchEvent(new Event("pause")); });
  await rejected;
  const after = await serverProgress(page, id);
  expect(after).toEqual(before);
  if (baseline) await expect(page.locator("[data-progress-notice]")).toBeHidden();
  else {
    await expect(page.locator("[data-progress-status]")).toHaveText("Your position is not saved. Reload this page to check access to this title.");
    await expect(page.locator("[data-progress-notice]")).toBeVisible();
    await expect(page.getByRole("button", {name: "Retry saving position", exact: true})).toBeHidden();
  }
  await page.screenshot({path: testInfo.outputPath(baseline ? "baseline-real-server-rejection.png" : "real-server-rejection.png"), fullPage: true});
});

test("populated player retries the latest progress through the real Server and renders accessible states", {tag: "@smoke"}, async ({page}, testInfo) => {
  test.skip(baseline, "the baseline rejection reproduction is separate from repaired recovery");
  await login(page);
  const watch = await firstPlayable(page);
  let fail = false;
  let hold = false;
  let release: (() => void) | undefined;
  const requests: Array<{seconds: number; revision: string}> = [];
  await page.route("**/progress/**", async route => {
    const body = new URLSearchParams(route.request().postData() || "");
    requests.push({seconds: Number(body.get("seconds")), revision: body.get("revision")!});
    if (fail) return route.fulfill({status: 503});
    if (!hold) return route.continue();
    await new Promise<void>(resolve => release = resolve);
    await route.continue();
  });
  await page.goto(watch);
  await inspectNotice(page, testInfo, "idle", false);
  fail = true;
  const media = page.locator("video");
  await expect.poll(() => media.evaluate((video: HTMLVideoElement) => video.readyState)).toBeGreaterThanOrEqual(2);
  await media.evaluate((video: HTMLVideoElement) => { video.pause(); video.currentTime = Math.min(video.duration / 3, 30); video.dispatchEvent(new Event("pause")); });
  await expect(page.locator("[data-progress-status]")).toHaveText("Your latest position is not saved. Retry while this page is open.");
  await inspectNotice(page, testInfo, "failed", true);
  for (const viewport of [{width: 390, height: 844}, {width: 1440, height: 900}, {width: 1920, height: 1080}]) {
    await page.setViewportSize(viewport);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    await expect(page.getByRole("button", {name: "Retry saving position", exact: true})).toBeVisible();
    const failures = (await new AxeBuilder({page}).include("[data-progress-notice]").analyze()).violations;
    expect(failures).toEqual([]);
    await page.screenshot({path: testInfo.outputPath(`unsaved-${viewport.width}.png`), fullPage: true});
  }
  const pending = requests.at(-1)!;
  fail = false;
  hold = true;
  await page.getByRole("button", {name: "Retry saving position", exact: true}).click();
  await expect(page.locator("[data-progress-notice]")).toHaveAttribute("aria-busy", "true");
  await expect(page.locator("[data-progress-status]")).toHaveText("Saving progress…");
  await expect(page.getByRole("button", {name: "Retry saving position", exact: true})).toBeDisabled();
  await inspectNotice(page, testInfo, "pending", true);
  await page.screenshot({path: testInfo.outputPath("pending-save.png"), fullPage: true});
  await expect.poll(() => Boolean(release)).toBe(true);
  release!();
  await expect(page.locator("[data-progress-notice]")).toBeHidden();
  await inspectNotice(page, testInfo, "saved", false);
  const id = watch.split("/").at(-1)!;
  const state = await serverProgress(page, id);
  expect(state.seconds).toBeCloseTo(pending.seconds, 2);
  expect(String(state.revision)).toBe(pending.revision);
  expect(requests.at(-1)).toEqual(pending);
  await page.screenshot({path: testInfo.outputPath("saved.png"), fullPage: true});
  await page.reload();
  await expect(page.locator("[data-progress-notice]")).toBeHidden();
  expect(await page.locator("[data-progress-status]").textContent()).not.toContain("Your position is saved");
});

test('Mark watched waits for an older played-position request before changing stored status', {tag:'@smoke'}, async ({page},info) => {
  await login(page);
  const watch=await firstPlayable(page), id=watch.split('/').at(-1)!;
  const publicProgress=()=>page.evaluate(async id=>{const response=await fetch(`/api/v1/items/${id}`);if(response.status!==200)throw new Error(`Public progress HTTP ${response.status}`);return (await response.json()).item.progress;},id);
  await page.goto(watch);
  const media=page.locator('video');
  await media.evaluate(async(video:HTMLVideoElement)=>{video.muted=true;await video.play();});
  await expect.poll(()=>media.evaluate((video:HTMLVideoElement)=>video.getVideoPlaybackQuality().totalVideoFrames)).toBeGreaterThan(2);
  let release!:()=>void, held=false, watchedRequests=0;
  const barrier=new Promise<void>(resolve=>release=resolve);
  page.on('request',request=>{if(new URL(request.url()).pathname===`/watched/${id}`)watchedRequests++;});
  await page.route(`**/progress/${id}*`,async route=>{held=true;await barrier;await route.continue();});
  try {
    await media.evaluate((video:HTMLVideoElement)=>video.pause());
    await expect.poll(()=>held).toBe(true);
    await page.getByRole('button',{name:'Mark watched',exact:true}).click();
    await expect(page.locator('[data-progress-status]')).toHaveText('Saving progress…');
    expect(watchedRequests).toBe(0);
    await page.screenshot({path:info.outputPath('watched-awaits-position.png'),fullPage:true});
    release();
    await expect(page.getByRole('button',{name:'Mark unwatched',exact:true})).toBeVisible();
    await expect.poll(()=>publicProgress().then(state=>state.watched)).toBe(true);
    await page.getByRole('link',{name:'Library',exact:true}).click();
    await expect(page).toHaveURL('/');
    expect((await publicProgress()).watched).toBe(true);
  } finally {release();}
});
