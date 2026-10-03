import {createHash} from "node:crypto";
import {writeFile} from "node:fs/promises";
import {expect, type Locator, type Page, type Route, type TestInfo} from "@playwright/test";

export const directRetryViewports = [{width: 390, height: 844}, {width: 1440, height: 900}];

type MediaState = Awaited<ReturnType<typeof mediaState>>;
const mediaState = (video: Locator) => video.evaluate((media: HTMLVideoElement) => {
  const stage = media.closest(".media-stage")!;
  const status = stage.querySelector<HTMLElement>("[data-player-status]")!;
  const skeleton = status.querySelector<HTMLElement>(".buffer-skeleton")!;
  const bounds = stage.getBoundingClientRect();
  return {ready: media.readyState, paused: media.paused, time: media.currentTime,
    duration: Number.isFinite(media.duration) ? media.duration : null,
    frames: media.getVideoPlaybackQuality().totalVideoFrames,
    error: media.error ? {code: media.error.code, message: media.error.message} : null,
    geometry: {width: bounds.width, height: bounds.height},
    status: {hidden: status.hidden, busy: status.getAttribute("aria-busy"), text: status.innerText,
      skeletonVisible: !skeleton.hidden && !status.hidden && skeleton.getClientRects().length > 0}};
});
const sourceDigest = (source: string, baseURL = "https://private-fixture.invalid") => {
  const original = new URL(source, baseURL);
  original.hash = ""; // Resume position belongs to the existing source-change controller.
  return createHash("sha256").update(original.href).digest("hex");
};

export async function verifyDirectRetry(page: Page, info: TestInfo, browserName: string, openWatch: () => Promise<string>, viewport: {width: number; height: number}) {
  const receipt = {revision: process.env.KINOSAIL_TEST_REVISION ?? process.env.GITHUB_SHA, binarySHA256: process.env.KINOSAIL_TEST_BINARY_SHA256,
    command: process.env.KINOSAIL_TEST_COMMAND ?? `pnpm --dir e2e test${process.env.KINOSAIL_BROWSER_SMOKE === "1" ? " --grep=@smoke" : ""}`,
    environment: {browserName, browserVersion: page.context().browser()?.version(), node: process.version, platform: process.platform, viewport, baseURL: info.project.use.baseURL},
    data: "Existing public watch path and current Viewer state; ?direct=1 override; exactly one induced media HTTP 500",
    console: [] as {type: string; text: string}[], pageErrors: [] as string[],
    requests: [] as {kind: string; at: number; path: string; sourceSHA256: string; status?: number}[],
    states: {} as Record<string, MediaState>, routeEvents: [] as {phase: string; at: number; path: string; result: string}[], failedRequests: 0, retryRequests: 0,
    failedSourceSHA256: "", retrySourceSHA256: "", renderedSourceShape: {} as Record<string, boolean>, result: "started", failure: ""};
  page.on("console", message => receipt.console.push({type: message.type(), text: message.text()}));
  page.on("pageerror", error => receipt.pageErrors.push(error.message));
  page.on("request", request => {
    if (request.resourceType() === "media") receipt.requests.push({kind: "request", at: Date.now(), path: new URL(request.url()).pathname, sourceSHA256: sourceDigest(request.url())});
  });
  page.on("response", response => {
    if (response.request().resourceType() === "media") receipt.requests.push({kind: "response", at: Date.now(), path: new URL(response.url()).pathname, sourceSHA256: sourceDigest(response.url()), status: response.status()});
  });
  page.on("requestfailed", request => {
    if (request.resourceType() === "media") receipt.requests.push({kind: `requestfailed:${request.failure()?.errorText}`, at: Date.now(), path: new URL(request.url()).pathname, sourceSHA256: sourceDigest(request.url())});
  });
  const video = page.locator("video");
  let releasePending: (() => void) | undefined;
  let ownedRouteInstalled = false;
  let mediaMode: "pending" | "pass" | "fail-next" = "pending";
  try {
    const watchURL = new URL(await openWatch(), info.project.use.baseURL);
    watchURL.searchParams.set("direct", "1");
    const watch = watchURL.pathname + watchURL.search;
    let release!: () => void, started!: () => void;
    const pending = new Promise<void>(resolve => {release = resolve; releasePending = resolve;});
    const requested = new Promise<void>(resolve => {started = resolve;});
    const handleMedia = async (route: Route) => {
      const event = {phase: mediaMode, at: Date.now(), path: new URL(route.request().url()).pathname, result: "started"};
      receipt.routeEvents.push(event);
      try {
        if (mediaMode === "pending") {started(); await pending;}
        if (mediaMode === "fail-next") {
          mediaMode = "pass"; receipt.failedRequests++;
          await route.fulfill({status: 500, contentType: "text/plain", body: "Private QA media failure"});
          event.result = "fulfilled-500";
        } else {await route.continue(); event.result = "continued";}
      } catch (error) {event.result = error instanceof Error ? error.message : String(error); throw error;}
    };
    await page.route("**/media/**", handleMedia);
    ownedRouteInstalled = true;
    await page.goto(watch, {waitUntil: "domcontentloaded"});
    await requested;
    await expect(page.locator("[data-player-status]")).toHaveAttribute("aria-busy", "true");
    receipt.states.pending = await mediaState(video);
    await expect(video).toHaveAttribute("data-playback-policy", "direct");
    receipt.renderedSourceShape = await video.evaluate(media => Object.fromEntries(["data-direct", "data-adaptive", "data-hls"].map(name => [name, media.hasAttribute(name)])));
    expect(receipt.renderedSourceShape).toEqual({"data-direct": false, "data-adaptive": false, "data-hls": false});
    await expect(video).toHaveJSProperty("controls", true);
    expect(receipt.states.pending.status.skeletonVisible).toBe(true);
    expect(receipt.states.pending.status.busy).toBe("true");
    expect(receipt.states.pending.ready).toBe(0);
    await page.screenshot({path: info.outputPath(`direct-${viewport.width}-pending.png`)});
    // Removing this route while its callback is suspended makes Playwright handle it twice.
    mediaMode = "pass"; release();
    await expect.poll(() => video.evaluate((media: HTMLVideoElement) => media.readyState)).toBeGreaterThanOrEqual(3);
    await expect(page.locator("[data-player-status]")).toBeHidden();
    receipt.states.loaded = await mediaState(video);
    expect(receipt.states.loaded.status.hidden).toBe(true);
    expect(receipt.states.loaded.geometry).toEqual(receipt.states.pending.geometry);
    expect(receipt.states.loaded.duration).toBeGreaterThan(0);
    await page.screenshot({path: info.outputPath(`direct-${viewport.width}-loaded.png`)});
    if (await video.evaluate((media: HTMLVideoElement) => media.paused)) {
      await page.locator(".media-stage").focus(); await page.keyboard.press("k");
    }
    const before = await video.evaluate((media: HTMLVideoElement) => media.currentTime);
    await expect.poll(() => video.evaluate((media: HTMLVideoElement) => media.currentTime)).toBeGreaterThan(before + 0.3);
    receipt.states.decodedControl = await mediaState(video);
    expect(receipt.states.decodedControl.frames).toBeGreaterThan(0);
    expect(receipt.pageErrors).toEqual([]);
    await video.evaluate((media: HTMLVideoElement) => media.pause());
    mediaMode = "fail-next";
    await page.goto(watch, {waitUntil: "domcontentloaded"});
    const retry = page.getByRole("button", {name: "Retry Direct Play", exact: true});
    await expect(retry).toBeVisible();
    receipt.states.failed = await mediaState(video);
    expect(await video.evaluate(media => ["data-direct", "data-adaptive", "data-hls"].every(name => !media.hasAttribute(name)))).toBe(true);
    expect(receipt.failedRequests).toBe(1);
    await expect.poll(() => receipt.requests.some(request => request.kind === "response" && request.status === 500)).toBe(true);
    expect(receipt.states.failed.error?.code).toBeGreaterThan(0);
    expect(receipt.states.failed.status.skeletonVisible).toBe(false);
    expect(receipt.states.failed.status.busy).toBe("false");
    expect(receipt.states.failed.geometry).toEqual(receipt.states.loaded.geometry);
    receipt.failedSourceSHA256 = sourceDigest(await video.getAttribute("src") ?? "", page.url());
    await page.screenshot({path: info.outputPath(`direct-${viewport.width}-failed.png`)});
    expect(receipt.pageErrors).toEqual([]);
    const retryBoundary = receipt.requests.length;
    await retry.click();
    const retryEvents = () => receipt.requests.slice(retryBoundary).filter(request => request.sourceSHA256 === receipt.failedSourceSHA256);
    await expect.poll(() => retryEvents().filter(request => request.kind === "request").length).toBeGreaterThan(0);
    await expect.poll(() => retryEvents().filter(request => request.kind === "response" && request.status! >= 200 && request.status! < 300).length).toBeGreaterThan(0);
    receipt.retryRequests = retryEvents().filter(request => request.kind === "request").length;
    receipt.retrySourceSHA256 = sourceDigest(await video.getAttribute("src") ?? "", page.url());
    expect(receipt.retrySourceSHA256).toBe(receipt.failedSourceSHA256);
    await expect.poll(() => video.evaluate((media: HTMLVideoElement) => !media.error && media.readyState >= 3)).toBe(true);
    const resumedAt = await video.evaluate((media: HTMLVideoElement) => media.currentTime);
    await expect.poll(() => video.evaluate((media: HTMLVideoElement) => media.currentTime)).toBeGreaterThan(resumedAt + 0.3);
    await expect(page.locator("[data-player-status]")).toBeHidden();
    receipt.states.recovered = await mediaState(video);
    expect(receipt.states.recovered.paused).toBe(false);
    expect(receipt.states.recovered.frames).toBeGreaterThan(0);
    expect(receipt.states.recovered.status.hidden).toBe(true);
    expect(receipt.states.recovered.status.skeletonVisible).toBe(false);
    expect(receipt.states.recovered.geometry).toEqual(receipt.states.loaded.geometry);
    expect(receipt.pageErrors).toEqual([]);
    await page.screenshot({path: info.outputPath(`direct-${viewport.width}-recovered.png`)});
    receipt.result = "passed";
  } catch (error) {
    receipt.result = "failed"; receipt.failure = error instanceof Error ? error.message : String(error);
    throw error;
  } finally {
    mediaMode = "pass"; releasePending?.();
    // These fresh journey pages own only this route. Drain callbacks before removal,
    // retaining any route failures rather than suppressing them at teardown.
    try {if (ownedRouteInstalled) await page.unrouteAll({behavior: "wait"});}
    catch (error) {
      receipt.result = "failed";
      receipt.failure += `\nRoute cleanup: ${error instanceof Error ? error.message : String(error)}`;
      throw error;
    } finally {
    try {
      receipt.states.final = await mediaState(video);
      await page.screenshot({path: info.outputPath(`direct-${viewport.width}-final.png`)});
    } catch { /* The page may have failed before a video was rendered. */ }
    const artifact = info.outputPath(`direct-${viewport.width}-retry-receipt.json`);
    await writeFile(artifact, JSON.stringify(receipt, null, 2));
    await info.attach(`direct-${viewport.width}-retry-receipt`, {path: artifact, contentType: "application/json"});
    }
  }
}

// Reuse the current Viewer state after setup; this creates no profile.
export async function verifyAuthenticatedDirectRetryWidths(authenticatedPage: Page, info: TestInfo) {
  const browser = authenticatedPage.context().browser();
  if (!browser) throw new Error("direct retry needs the existing test browser");
  const storageState = await authenticatedPage.context().storageState();
  for (const viewport of directRetryViewports) {
    const context = await browser.newContext({baseURL: info.project.use.baseURL, ignoreHTTPSErrors: false,
      storageState, viewport, hasTouch: viewport.width === 390});
    try {
      const page = await context.newPage();
      await verifyDirectRetry(page, info, info.project.name, async () => {
        await page.goto("/?view=movies");
        const card = page.locator('a.card[href^="/watch/"]').filter({hasText: "Direct Retry Control"});
        await expect(card).toHaveCount(1);
        const watchPath = await card.getAttribute("href");
        if (!watchPath) throw new Error("direct-retry smoke MP4 fixture has no watch link");
        return watchPath;
      }, viewport);
    } finally {
      await context.close();
    }
  }
}
