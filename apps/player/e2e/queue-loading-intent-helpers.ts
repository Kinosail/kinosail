import {expect, type Page, type TestInfo} from "@playwright/test";

type Track = {id: string; stream: string};
type Observation = {position: number; paused: boolean; readyState: number; sourcePath: string; duration: number; clockMs: number};
type IntentWindow = Window & {queueLoadingMetadata?: Observation};

export async function write(page: Page, path: string, body: Record<string, string | number | boolean>, method = "PUT") {
  return page.evaluate(async ({path, body, method}) => {
    const csrf = document.querySelector<HTMLMetaElement>('meta[name="kinosail-csrf"]')?.content || "";
    return (await fetch(path, {method, headers: {"Content-Type": "application/json", "X-Kinosail-CSRF": csrf},
      body: JSON.stringify(body)})).status;
  }, {path, body, method});
}

export async function tracks(page: Page, albumTitle = "R08 Fictional Session"): Promise<Track[]> {
  const albums = await page.request.get("/api/v1/albums");
  expect(albums.status()).toBe(200);
  const album = (await albums.json()).albums.find((album: {title: string}) => album.title === albumTitle);
  expect(album, "the existing fictional album must be present").toBeTruthy();
  const response = await page.request.get(`/api/v1/albums/${album.id}`);
  expect(response.status()).toBe(200);
  const observed = (await response.json()).tracks;
  expect(observed).toHaveLength(2);
  return observed;
}

// Only delay the real Server media request. Media, authorization, state,
// commands, browser events and progress acknowledgements remain genuine.
export async function runLoadingIntent(page: Page, info: TestInfo, command: "seek" | "stop", albumTitle = "R08 Fictional Session", savedPosition = 5) {
  await page.goto("/");
  const [first, second] = await tracks(page, albumTitle);
  for (const [track, seconds] of [[first, 0], [second, savedPosition]] as const) {
    expect(await write(page, `/api/v1/items/${track.id}/progress`, {seconds, watched: false})).toBe(200);
  }
  expect(await write(page, "/api/v1/settings/home-assistant", {enabled: true})).toBe(200);
  const accepted: Array<{seconds: number; revision: number; status: number}> = [];
  page.on("response", response => {
    if (new URL(response.url()).pathname !== `/progress/${second.id}` || response.request().method() !== "POST") return;
    const body = new URLSearchParams(response.request().postData() || "");
    if (accepted.length < 100) accepted.push({seconds: Number(body.get("seconds")), revision: Number(body.get("revision")), status: response.status()});
  });
  const control = `/__queue-media${second.stream}`;
  expect((await page.request.put(control)).status()).toBe(204);
  let released = false;
  const unblock = async () => {
    if (released) return;
    expect((await page.request.delete(control)).status()).toBe(204);
    released = true;
  };
  try {
    await page.goto(`/watch/${first.id}`, {waitUntil: "domcontentloaded"});
    const media = page.locator("audio");
    await expect(media).toHaveAttribute("data-home-assistant", "true");
    await media.evaluate(async (audio: HTMLAudioElement) => {audio.muted = true; await audio.play();});
    await expect.poll(() => media.evaluate((audio: HTMLAudioElement) => audio.readyState)).toBeGreaterThanOrEqual(2);
    await media.evaluate((audio: HTMLAudioElement) => audio.pause());
    await page.waitForFunction("audioQueue.length === 1");
    let target: {id: string; itemId: string} | undefined;
    await expect.poll(async () => {
      const targets = await page.request.get("/api/v1/home-assistant/players");
      expect(targets.status()).toBe(200);
      target = (await targets.json()).players.find((value: {itemId: string}) => value.itemId === first.id);
      return Boolean(target);
    }, {message: "the real document must publish its public target"}).toBe(true);
    await page.getByRole("button", {name: "Next track", exact: true}).click();
    await expect(media).toHaveAttribute("data-progress", `/progress/${second.id}`);
    await expect(media).toHaveAttribute("data-start", String(savedPosition));
    await expect.poll(async () => (await (await page.request.get("/__queue-media")).json()).held).toBeGreaterThan(0);
    expect(await media.evaluate((audio: HTMLAudioElement) => audio.readyState)).toBe(0);
    await expect.poll(() => media.evaluate((audio: HTMLAudioElement) => audio.paused)).toBe(false);
    const queueBefore = await page.evaluate("({sourceChanging: queueSourceChanging, progressReady: queueProgressReady, progressRevision})");
    expect(queueBefore.sourceChanging).toBe(true); expect(queueBefore.progressReady).toBe(false);
    const consumed = page.waitForResponse(async response => new URL(response.url()).pathname ===
      `/api/v1/home-assistant/players/${target!.id}` && response.request().method() === "PUT" &&
      response.status() === 200 && (await response.json()).command === command);
    void consumed.catch(() => {});
    const position = command === "seek" ? 2 : 0;
    const queued = await write(page, `/api/v1/home-assistant/players/${target!.id}/commands`,
      command === "seek" ? {command, position} : {command}, "POST");
    expect(queued).toBe(202);
    const delivered = await consumed;
    expect(delivered.request().postDataJSON().itemId).toBe(second.id);
    await expect.poll(() => media.evaluate((audio: HTMLAudioElement) => audio.currentTime)).toBeCloseTo(position, 2);
    if (command === "stop") await expect.poll(() => media.evaluate((audio: HTMLAudioElement) => audio.paused)).toBe(true);
    const beforeMetadata = await media.evaluate((audio: HTMLAudioElement) => ({position: audio.currentTime,
      paused: audio.paused, readyState: audio.readyState, sourcePath: new URL(audio.src).pathname,
      duration: Number.isFinite(audio.duration) ? audio.duration : 0, clockMs: performance.now()}));
    expect(beforeMetadata.readyState).toBe(0);
    expect(beforeMetadata.sourcePath).toBe(second.stream);
    await media.evaluate((audio: HTMLAudioElement) => audio.addEventListener("loadedmetadata", () => {
      (window as IntentWindow).queueLoadingMetadata = {position: audio.currentTime, paused: audio.paused,
        readyState: audio.readyState, sourcePath: new URL(audio.currentSrc || audio.src).pathname,
        duration: audio.duration, clockMs: performance.now()};
      audio.pause();
    }, {once: true}));
    await unblock();
    await expect.poll(() => page.evaluate(() => Boolean((window as IntentWindow).queueLoadingMetadata))).toBe(true);
    const afterMetadata = await page.evaluate(() => (window as IntentWindow).queueLoadingMetadata!);
    const publicState = await page.request.get(`/api/v1/items/${second.id}`);
    expect(publicState.status()).toBe(200);
    const publicPosition = (await publicState.json()).item.progress.seconds;
    await info.attach("actual-queue-loading-intent", {body: JSON.stringify({command, position,
      savedPosition, realCommandQueuedStatus: queued, realCommandConsumedStatus: delivered.status(),
      beforeMetadata, afterMetadata, queueBefore, publicTargetPublishedForCurrentItem: true,
      publicProgressStatus: 200, publicPosition, accepted,
      actualPlatformMediaSessionQualified: false, mediaDelayOnly: true, syntheticMetadataEvent: false}), contentType: "application/json"});
    expect(afterMetadata.sourcePath).toBe(second.stream);
    expect(afterMetadata.readyState).toBeGreaterThanOrEqual(1);
    expect(afterMetadata.duration).toBeGreaterThan(savedPosition);
    expect(afterMetadata.position).toBeCloseTo(position, 1);
    if (command === "stop") expect(afterMetadata.paused).toBe(true);
    await expect.poll(() => accepted.some(value => value.status === 204 && value.seconds === position && value.revision > 0)).toBe(true);
    let publicReadback: number | undefined;
    await expect.poll(async () => {
      const response = await page.request.get(`/api/v1/items/${second.id}`);
      expect(response.status()).toBe(200);
      const body = await response.json();
      expect(body.item.id, "readback must identify the installed queued track").toBe(second.id);
      const progress = body.item.progress;
      expect(progress, "accepted progress must remain an object").toBeTruthy();
      expect(typeof progress).toBe("object");
      expect(Array.isArray(progress)).toBe(false);
      // PlaybackState omits zero seconds. This branch is reached only after
      // the current track's zero write received 204 with a positive revision.
      if (command === "stop" && !Object.hasOwn(progress, "seconds")) publicReadback = 0;
      else {
        expect(typeof progress.seconds, "seek and present seconds must be numeric").toBe("number");
        expect(Number.isFinite(progress.seconds)).toBe(true);
        expect(progress.seconds).toBeGreaterThanOrEqual(0);
        publicReadback = progress.seconds;
      }
      return publicReadback;
    }).toBeCloseTo(position, 1);
    await info.attach("accepted-current-public-progress", {body: JSON.stringify({command, position, savedPosition,
      accepted, publicProgressStatus: 200, publicPosition: publicReadback}), contentType: "application/json"});
  } finally {
    await unblock();
    if (new URL(page.url()).pathname.startsWith("/watch/")) await page.goto("/");
    expect(await write(page, "/api/v1/settings/home-assistant", {enabled: false})).toBe(200);
  }
}
