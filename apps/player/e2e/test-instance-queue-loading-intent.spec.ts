import {expect, test, type Page} from "@playwright/test";
import {configureTestInstance, login} from "./test-instance-helpers";

configureTestInstance();
type Track = {id: string; stream: string};
type Observation = {position: number; paused: boolean; readyState: number; sourcePath: string};
type IntentWindow = Window & {queueLoadingMetadata?: Observation};

async function write(page: Page, path: string, body: unknown, method = "PUT") {
  return page.evaluate(async ({path, body, method}) => {
    const csrf = document.querySelector<HTMLMetaElement>('meta[name="kinosail-csrf"]')?.content || "";
    return (await fetch(path, {method, headers: {"Content-Type": "application/json", "X-Kinosail-CSRF": csrf},
      body: JSON.stringify(body)})).status;
  }, {path, body, method});
}

async function tracks(page: Page): Promise<Track[]> {
  const albums = await page.request.get("/api/v1/albums");
  expect(albums.status()).toBe(200);
  const album = (await albums.json()).albums.find((album: {title: string}) => album.title === "R08 Fictional Session");
  expect(album, "the existing fictional album must be present").toBeTruthy();
  const response = await page.request.get(`/api/v1/albums/${album.id}`);
  expect(response.status()).toBe(200);
  const observed = (await response.json()).tracks;
  expect(observed).toHaveLength(2);
  return observed;
}

// Only delay the real Server media request. Media, authorization, state,
// commands, browser events and progress acknowledgements remain genuine.
for (const command of ["seek", "stop"] as const) {
  test(`real queued track preserves ${command} arriving before actual metadata`, {tag: ["@smoke", "@network-delay"]}, async ({page}, info) => {
    await login(page);
    const [first, second] = await tracks(page);
    for (const [track, seconds] of [[first, 0], [second, 5]] as const) {
      expect(await write(page, `/api/v1/items/${track.id}/progress`, {seconds, watched: false})).toBe(200);
    }
    expect(await write(page, "/api/v1/settings/home-assistant", {enabled: true})).toBe(200);
    let unblock!: () => void, held = 0;
    const barrier = new Promise<void>(resolve => unblock = resolve);
    const mediaRoute = `**${second.stream}*`;
    await page.route(mediaRoute, async route => {
      if (new URL(route.request().url()).pathname !== second.stream) return route.continue();
      held++;
      await barrier;
      await route.continue().catch(() => {});
    });
    try {
      await page.goto(`/watch/${first.id}`, {waitUntil: "domcontentloaded"});
      const media = page.locator("audio");
      await expect(media).toHaveAttribute("data-home-assistant", "true");
      await expect.poll(() => media.evaluate((audio: HTMLAudioElement) => audio.readyState)).toBeGreaterThanOrEqual(2);
      await media.evaluate((audio: HTMLAudioElement) => {audio.muted = true; audio.pause();});
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
      await expect(media).toHaveAttribute("data-start", "5");
      await expect.poll(() => held).toBeGreaterThan(0);
      expect(await media.evaluate((audio: HTMLAudioElement) => audio.readyState)).toBe(0);
      await expect.poll(() => media.evaluate((audio: HTMLAudioElement) => audio.paused)).toBe(false);
      const consumed = page.waitForResponse(async response => new URL(response.url()).pathname ===
        `/api/v1/home-assistant/players/${target!.id}` && response.request().method() === "PUT" &&
        response.status() === 200 && (await response.json()).command === command);
      void consumed.catch(() => {});
      const position = command === "seek" ? 2 : 0;
      const queued = await write(page, `/api/v1/home-assistant/players/${target!.id}/commands`,
        command === "seek" ? {command, position} : {command}, "POST");
      expect(queued).toBe(202);
      const delivered = await consumed;
      await expect.poll(() => media.evaluate((audio: HTMLAudioElement) => audio.currentTime)).toBeCloseTo(position, 2);
      if (command === "stop") await expect.poll(() => media.evaluate((audio: HTMLAudioElement) => audio.paused)).toBe(true);
      const beforeMetadata = await media.evaluate((audio: HTMLAudioElement) => ({position: audio.currentTime,
        paused: audio.paused, readyState: audio.readyState, sourcePath: new URL(audio.src).pathname}));
      expect(beforeMetadata.readyState).toBe(0);
      expect(beforeMetadata.sourcePath).toBe(second.stream);
      await media.evaluate((audio: HTMLAudioElement) => audio.addEventListener("loadedmetadata", () => {
        (window as IntentWindow).queueLoadingMetadata = {position: audio.currentTime, paused: audio.paused,
          readyState: audio.readyState, sourcePath: new URL(audio.currentSrc || audio.src).pathname};
        audio.pause();
      }, {once: true}));
      unblock();
      await expect.poll(() => page.evaluate(() => Boolean((window as IntentWindow).queueLoadingMetadata))).toBe(true);
      const afterMetadata = await page.evaluate(() => (window as IntentWindow).queueLoadingMetadata!);
      await info.attach("actual-queue-loading-intent", {body: JSON.stringify({command, position,
        savedPosition: 5, realCommandQueuedStatus: queued, realCommandConsumedStatus: delivered.status(),
        beforeMetadata, afterMetadata, mediaDelayOnly: true, syntheticMetadataEvent: false}), contentType: "application/json"});
      expect(afterMetadata.sourcePath).toBe(second.stream);
      expect(afterMetadata.readyState).toBeGreaterThanOrEqual(1);
      expect(afterMetadata.position).toBeCloseTo(position, 1);
    } finally {
      unblock();
      await page.unroute(mediaRoute);
      if (new URL(page.url()).pathname.startsWith("/watch/")) await page.goto("/");
      expect(await write(page, "/api/v1/settings/home-assistant", {enabled: false})).toBe(200);
    }
  });
}
