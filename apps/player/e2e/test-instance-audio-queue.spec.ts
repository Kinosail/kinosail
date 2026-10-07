import {expect, test, type Page} from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import {createHash} from "node:crypto";
import {configureTestInstance, login} from "./test-instance-helpers";

configureTestInstance();
type Track = {id: string; title: string; artist: string; album: string; track: number; stream: string; artwork: string};
type ActionWindow = Window & {r08Actions: Partial<Record<MediaSessionAction, MediaSessionActionHandler | null>>};
const fixtureAlbum = "R08 Fictional Session";

async function albumTracks(page: Page): Promise<Track[]> {
  const albums = await page.request.get("/api/v1/albums");
  expect(albums.status()).toBe(200);
  const selected = (await albums.json()).albums.find((album: {title: string}) => album.title === fixtureAlbum);
  expect(selected).toBeTruthy();
  const album = await page.request.get(`/api/v1/albums/${selected.id}`);
  expect(album.status()).toBe(200);
  const tracks = (await album.json()).tracks;
  expect(tracks).toHaveLength(2);
  expect(tracks.map((track: Track) => track.track)).toEqual([1, 2]);
  expect(tracks[1].title).toBe("Copper <Moon> & Harbor");
  return tracks;
}

async function snapshot(page: Page) {
  return page.evaluate(() => {
    const audio = document.querySelector("audio")!;
    const metadata = navigator.mediaSession.metadata;
    const path = (value?: string | null) => value ? new URL(value, location.href).pathname : "";
    return {heading: document.querySelector(".title-block h1")?.textContent?.trim(),
      byline: document.querySelector(".title-byline")?.textContent?.trim(),
      accessibleLabel: audio.getAttribute("aria-label"), documentTitle: document.title,
      sourcePath: path(audio.currentSrc || audio.src), progressPath: path(audio.dataset.progress),
      castPath: path(audio.dataset.castApi), tracePath: path(audio.dataset.playbackTrace),
      sourceSessionMatches: new URL(audio.currentSrc || audio.src).searchParams.get("playbackSession") === audio.dataset.playbackSession,
      title: audio.dataset.title, artist: audio.dataset.artist, album: audio.dataset.album,
      artworkPath: path(audio.dataset.artwork), visibleArtworkPath: path(document.querySelector<HTMLImageElement>("[data-now-playing-artwork]")?.getAttribute("src")),
      metadataTitle: metadata?.title, metadataArtist: metadata?.artist, metadataAlbum: metadata?.album,
      metadataArtworkPath: path(metadata?.artwork[0]?.src),
      queuePosition: Number(audio.dataset.queuePosition || 0), queueTotal: Number(audio.dataset.queueTotal || 0)};
  });
}

test("real album queue advances source and all Now Playing identity to the fictional second track", {tag: "@smoke"}, async ({page}, testInfo) => {
  await login(page);
  const [first, second] = await albumTracks(page);
  const trace: Array<{item: string; event: string; status: number}> = [];
  page.on("response", response => {
    const request = response.request();
    const path = new URL(request.url()).pathname;
    if (path.endsWith("/playback-events") && request.method() === "POST") {
      const item = path.includes(first.id) ? "first" : path.includes(second.id) ? "second" : "other";
      trace.push({item, event: request.postDataJSON().event, status: response.status()});
    }
  });
  const queue = page.waitForResponse(response => new URL(response.url()).pathname === `/api/v1/audio/${first.id}/queue`);
  await page.goto(`/watch/${first.id}`);
  expect((await queue).status()).toBe(200);
  const playerAsset = await page.locator('script[src^="/static/player.js?"]').getAttribute("src");
  expect(playerAsset).toBeTruthy();
  const asset = await page.request.get(playerAsset!);
  expect(asset.status()).toBe(200);
  const assetHash = createHash("sha256").update(await asset.body()).digest("hex");
  expect(new URL(playerAsset!, "https://fixture.invalid").searchParams.get("v")).toBe(assetHash);
  expect(asset.headers()["cache-control"]).toBe("public, max-age=31536000, immutable");
  await testInfo.attach("player-content-cache-delivery", {body: JSON.stringify({path: playerAsset, sha256: assetHash,
    cacheControl: asset.headers()["cache-control"]}), contentType: "application/json"});
  const media = page.locator("audio");
  await expect.poll(() => media.evaluate((audio: HTMLAudioElement) => audio.readyState)).toBeGreaterThanOrEqual(2);
  await expect(page.locator(".title-block h1")).toHaveText(first.title);
  await expect.poll(() => page.evaluate(() => navigator.mediaSession.metadata?.title)).toBe(first.title);
  await page.waitForFunction("audioQueue.length === 1");
  await media.evaluate(async (audio: HTMLAudioElement) => {audio.muted = true; audio.currentTime = audio.duration - .15; await audio.play();});
  await expect.poll(() => media.evaluate((audio: HTMLAudioElement) => new URL(audio.currentSrc || audio.src).pathname)).toBe(second.stream);
  await expect.poll(() => media.evaluate((audio: HTMLAudioElement) => audio.readyState)).toBeGreaterThanOrEqual(2);
  await media.evaluate((audio: HTMLAudioElement) => audio.pause());
  await expect(page.locator("[data-now-playing-artwork]")).toBeVisible();
  const observed = await snapshot(page);
  await testInfo.attach("now-playing-after-real-advance", {body: JSON.stringify(observed, null, 2), contentType: "application/json"});
  await page.screenshot({path: testInfo.outputPath("second-track-before-assertions.png"), fullPage: true});
  expect(observed).toEqual({heading: second.title, byline: `${second.artist} · ${second.album} · Track ${second.track}`,
    accessibleLabel: second.title, documentTitle: `${second.title} · Kinosail Player`,
    sourcePath: second.stream, progressPath: `/progress/${second.id}`, castPath: `/api/v1/items/${second.id}/cast`,
    tracePath: `/api/v1/items/${second.id}/playback-events`, sourceSessionMatches: true,
    title: second.title, artist: second.artist, album: second.album, artworkPath: second.artwork,
    visibleArtworkPath: second.artwork, metadataTitle: second.title, metadataArtist: second.artist,
    metadataAlbum: second.album, metadataArtworkPath: second.artwork, queuePosition: 2, queueTotal: 2});
  await expect(page.locator(`form[action="/watched/${first.id}"]`)).toBeHidden();
  await expect(page.locator(`form[action="/list/${first.id}"]`)).toBeHidden();
  await expect(page.getByRole("link", {name: "Current track details and actions", exact: true})).toHaveAttribute("href", `/watch/${second.id}`);
  expect(await page.locator(".title-block h1 moon").count()).toBe(0);
  await expect.poll(() => trace.some(value => value.item === "first" && value.event === "ended" && value.status === 204)).toBe(true);
  await expect.poll(() => trace.some(value => value.item === "second" && value.event === "play-request" && value.status === 204)).toBe(true);
  const firstState = await page.request.get(`/api/v1/items/${first.id}`);
  expect(firstState.status()).toBe(200);
  expect((await firstState.json()).item.progress.watched).toBe(true);
  for (const viewport of [{width: 390, height: 844}, {width: 1440, height: 900}, {width: 1920, height: 1080}]) {
    await page.setViewportSize(viewport);
    await testInfo.attach("viewport-item-actions", {body: JSON.stringify({viewport,
      originalWatchedVisible: await page.locator(`form[action="/watched/${first.id}"]`).isVisible(),
      originalListVisible: await page.locator(`form[action="/list/${first.id}"]`).isVisible(),
      noticeVisible: await page.locator("[data-progress-notice]").isVisible(),
    }), contentType: "application/json"});
    await expect(page.locator(`form[action="/watched/${first.id}"]`)).toBeHidden({timeout: 1500});
    await expect(page.locator(`form[action="/list/${first.id}"]`)).toBeHidden({timeout: 1500});
    await expect(page.locator("[data-progress-notice]")).toBeHidden({timeout: 1500});
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    expect((await new AxeBuilder({page}).include(".title-block").include("[data-audio-queue-controls]").include("[data-current-track-actions]").include(".media-stage").analyze()).violations).toEqual([]);
    await page.screenshot({path: testInfo.outputPath(`second-track-${viewport.width}.png`), fullPage: true});
  }
  const stylesheet = await page.locator('link[rel="stylesheet"][href^="/static/app.css?"]').getAttribute("href");
  expect(stylesheet).toBeTruthy();
  const css = await page.request.get(stylesheet!);
  expect(css.status()).toBe(200);
  expect(new URL(stylesheet!, "https://fixture.invalid").searchParams.get("v")).not.toBe("electric-47");
  expect(await css.text()).toContain(".primary-player-actions:not([hidden]){display:grid!important");
  expect(css.headers()["cache-control"]).toBe("public, max-age=31536000, immutable");
  await testInfo.attach("css-cache-delivery", {body: JSON.stringify({path: stylesheet,
    sha256: createHash("sha256").update(await css.body()).digest("hex"), cacheControl: css.headers()["cache-control"]}), contentType: "application/json"});
});

// Isolated 503 injection in an actual Go-rendered page. Initial and Retry 204
// acknowledgements remain real Server requests; canonical queue cases are unrouted.
test("mobile R03 progress notice stays hidden after real acknowledgement and reopens only on failure", {tag: ["@smoke", "@routed-fault"]}, async ({page}, testInfo) => {
  await login(page);
  const [first] = await albumTracks(page);
  await page.setViewportSize({width: 390, height: 844});
  await page.goto(`/watch/${first.id}`);
  const media = page.locator("audio"), notice = page.locator("[data-progress-notice]");
  await expect.poll(() => media.evaluate((audio: HTMLAudioElement) => audio.readyState)).toBeGreaterThanOrEqual(2);
  const saved = page.waitForResponse(response => new URL(response.url()).pathname === `/progress/${first.id}` && response.request().method() === "POST");
  await media.evaluate(async (audio: HTMLAudioElement) => {audio.muted = true; audio.currentTime = 1; await audio.play();});
  await expect.poll(() => media.evaluate((audio: HTMLAudioElement) => audio.currentTime)).toBeGreaterThan(1.1);
  await media.evaluate((audio: HTMLAudioElement) => audio.pause());
  const acknowledgement = await saved;
  expect(acknowledgement.status()).toBe(204);
  await testInfo.attach("mobile-progress-after-real-acknowledgement", {body: JSON.stringify({
    viewport: {width: 390, height: 844}, realAcknowledgementStatus: acknowledgement.status(),
    noticeVisible: await notice.isVisible(), hiddenAttribute: await notice.getAttribute("hidden") !== null,
  }), contentType: "application/json"});
  await expect(notice).toBeHidden({timeout: 1500});
  const failureRoute = `**/progress/${first.id}`;
  await page.route(failureRoute, route => route.fulfill({status: 503, headers: {"X-Request-ID": "qa-mobile-progress-failure"}}));
  await media.evaluate(async (audio: HTMLAudioElement) => {audio.currentTime = 2; await audio.play();});
  await expect.poll(() => media.evaluate((audio: HTMLAudioElement) => audio.currentTime)).toBeGreaterThan(2.1);
  await media.evaluate((audio: HTMLAudioElement) => audio.pause());
  await expect(notice).toBeVisible();
  await expect(page.locator("[data-progress-status]")).toHaveAttribute("data-progress-failure", "server");
  await expect(page.locator("[data-progress-status]")).toHaveAttribute("data-progress-request-id", "qa-mobile-progress-failure");
  expect((await new AxeBuilder({page}).include("[data-progress-notice]").analyze()).violations).toEqual([]);
  await page.screenshot({path: testInfo.outputPath("mobile-progress-failed.png"), fullPage: true});
  await page.unroute(failureRoute);
  const retried = page.waitForResponse(response => new URL(response.url()).pathname === `/progress/${first.id}` && response.request().method() === "POST");
  await page.getByRole("button", {name: "Retry saving position", exact: true}).click();
  expect((await retried).status()).toBe(204);
  await expect(notice).toBeHidden();
  await page.screenshot({path: testInfo.outputPath("mobile-progress-saved.png"), fullPage: true});
  await testInfo.attach("proof-class", {body: "Isolated 503 failure injection in a Go-backed UI; acknowledgements and Retry use the actual Server.", contentType: "text/plain"});
});

test("real album queue keeps system previous and next current and exposes only fresh current-track actions", {tag: "@smoke"}, async ({page}, testInfo) => {
  await login(page);
  const [first, second] = await albumTracks(page);
  await page.addInitScript(() => {
    const session = navigator.mediaSession;
    const register = session.setActionHandler.bind(session);
    (window as ActionWindow).r08Actions = {};
    session.setActionHandler = (action, handler) => {
      (window as ActionWindow).r08Actions[action] = handler;
      register(action, handler);
    };
  });
  await page.goto(`/watch/${first.id}`);
  const media = page.locator("audio");
  await expect.poll(() => media.evaluate((audio: HTMLAudioElement) => audio.readyState)).toBeGreaterThanOrEqual(2);
  await expect(page.getByRole("button", {name: "Previous track", exact: true})).toBeDisabled();
  await expect(page.getByRole("button", {name: "Next track", exact: true})).toBeEnabled();
  await media.evaluate((audio: HTMLAudioElement) => {audio.muted = true; audio.currentTime = 1; audio.pause();});
  await page.getByRole("button", {name: "Next track", exact: true}).click();
  await expect(media).toHaveAttribute("data-progress", `/progress/${second.id}`);
  await expect(page.getByRole("button", {name: "Next track", exact: true})).toBeDisabled();
  await expect.poll(() => page.evaluate(() => typeof (window as ActionWindow).r08Actions.previoustrack)).toBe("function");
  await media.evaluate((audio: HTMLAudioElement) => audio.pause());
  await page.evaluate(async () => (window as ActionWindow).r08Actions.previoustrack!({action: "previoustrack"}));
  await expect(media).toHaveAttribute("data-progress", `/progress/${first.id}`);
  await expect.poll(() => media.evaluate((audio: HTMLAudioElement) => audio.readyState)).toBeGreaterThanOrEqual(2);
  await expect.poll(() => media.evaluate((audio: HTMLAudioElement) => audio.currentTime)).toBeGreaterThanOrEqual(1);
  await media.evaluate((audio: HTMLAudioElement) => audio.pause());
  await expect(page.locator(".title-block h1")).toHaveText(first.title);
  await expect(page.getByRole("button", {name: "Previous track", exact: true})).toBeDisabled();
  await page.evaluate(async () => (window as ActionWindow).r08Actions.nexttrack!({action: "nexttrack"}));
  await expect(media).toHaveAttribute("data-progress", `/progress/${second.id}`);
  await media.evaluate((audio: HTMLAudioElement) => audio.pause());
  await expect(page.locator(`form[action="/watched/${first.id}"]`)).toBeHidden();
  const beforeFirst = await page.request.get(`/api/v1/items/${first.id}`);
  expect(beforeFirst.status()).toBe(200);
  const before = (await beforeFirst.json()).item.progress;
  await page.getByRole("link", {name: "Current track details and actions", exact: true}).click();
  await expect(page).toHaveURL(new RegExp(`/watch/${second.id}$`));
  await expect(page.locator(".title-block h1")).toHaveText(second.title);
  const secondForm = page.locator(`form[action="/watched/${second.id}"]`);
  await expect(secondForm).toBeVisible();
  const beforeSecond = await page.request.get(`/api/v1/items/${second.id}`);
  expect(beforeSecond.status()).toBe(200);
  const wantedWatched = !(await beforeSecond.json()).item.progress.watched;
  await expect(secondForm.locator("button")).toHaveAttribute("value", String(wantedWatched));
  const mutation = page.waitForResponse(response => new URL(response.url()).pathname === `/watched/${second.id}` && response.request().method() === "POST");
  await secondForm.locator("button").click();
  expect((await mutation).status()).toBe(303);
  const afterFirst = await page.request.get(`/api/v1/items/${first.id}`);
  expect(afterFirst.status()).toBe(200);
  expect((await afterFirst.json()).item.progress).toEqual(before);
  const afterSecond = await page.request.get(`/api/v1/items/${second.id}`);
  expect(afterSecond.status()).toBe(200);
  expect((await afterSecond.json()).item.progress.watched).toBe(wantedWatched);
  await page.screenshot({path: testInfo.outputPath("fresh-second-track-actions.png"), fullPage: true});
});

// Actual Go markup and delivered CSS with isolated queue-read delay, 503, and
// single-item projections. Canonical advances above remain unrouted.
test("album queue keeps accessible responsive controls through pending loaded empty and failed reads", {tag: ["@smoke", "@routed-fault"]}, async ({page}, testInfo) => {
  await login(page);
  const [first] = await albumTracks(page);
  const path = `**/api/v1/audio/${first.id}/queue`;
  let release: (() => void) | undefined;
  await page.route(path, async route => {
    const response = await route.fetch();
    expect(response.status()).toBe(200);
    await new Promise<void>(resolve => release = resolve);
    await route.fulfill({response});
  });
  await page.goto(`/watch/${first.id}`);
  await expect.poll(() => !!release).toBe(true);
  const controls = page.locator("[data-audio-queue-controls]");
  const next = page.getByRole("button", {name: "Next track", exact: true});
  const previous = page.getByRole("button", {name: "Previous track", exact: true});
  const inspect = async (state: string) => {
    for (const viewport of [{width: 390, height: 844}, {width: 1440, height: 900}, {width: 1920, height: 1080}]) {
      await page.setViewportSize(viewport);
      await expect(controls).toBeVisible();
      for (const button of [previous, next]) {
        const bounds = (await button.boundingBox())!;
        expect(bounds.height).toBeGreaterThanOrEqual(44);
        expect(bounds.width).toBeGreaterThanOrEqual(44);
      }
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      expect((await new AxeBuilder({page}).include("[data-audio-queue-controls]").analyze()).violations).toEqual([]);
      await testInfo.attach(`${state}-${viewport.width}-queue`, {body: JSON.stringify({viewport,
        bounds: await controls.boundingBox(), busy: await controls.getAttribute("aria-busy"),
        previousDisabled: await previous.isDisabled(), nextDisabled: await next.isDisabled(),
        status: await page.locator("[data-audio-queue-status]").textContent()}), contentType: "application/json"});
      await page.screenshot({path: testInfo.outputPath(`${state}-${viewport.width}.png`), fullPage: true});
    }
  };
  await expect(controls).toHaveAttribute("aria-busy", "true");
  await expect(next).toBeDisabled();
  await expect(previous).toBeDisabled();
  await inspect("pending");
  release!();
  await expect(next).toBeEnabled();
  await expect(controls).not.toHaveAttribute("aria-busy");
  await inspect("loaded");
  await page.unroute(path);
  await page.route(path, async route => {
    const response = await route.fetch();
    expect(response.status()).toBe(200);
    const body = await response.json();
    await route.fulfill({response, json: {...body, items: body.items.slice(0, 1)}});
  });
  await page.reload();
  await expect(page.locator("[data-audio-queue-status]")).toHaveText("Track 1 of 1");
  await expect(next).toBeDisabled();
  await inspect("empty");
  await page.unroute(path);
  await page.route(path, route => route.fulfill({status: 503, headers: {"X-Request-ID": "qa-queue-read-failure"}}));
  await page.reload();
  await expect(page.locator("[data-audio-queue-status]")).toHaveAttribute("data-queue-failure", "server");
  await expect(page.locator("[data-audio-queue-status]")).toHaveAttribute("data-queue-request-id", "qa-queue-read-failure");
  await expect(next).toBeDisabled();
  await inspect("failed");
  await page.unroute(path);
  await page.getByRole("button", {name: "Retry loading queue", exact: true}).click();
  await expect(next).toBeEnabled();
  await expect(page.locator("[data-audio-queue-status]")).not.toHaveAttribute("data-queue-failure");
});
