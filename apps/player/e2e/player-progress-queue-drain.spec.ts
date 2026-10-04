import {expect, test} from "@playwright/test";
import {createHash} from "node:crypto";
import {playerSource} from "./static-sources";

// Isolated ordering fault: a real Server cannot deterministically hold the
// browser's play promise across a new-source pause. No production CSP is changed.
test.use({baseURL: "https://queue-progress.test"});

test("new track pause is dispatched after watched continuation release", {tag: "@smoke"}, async ({page}, testInfo) => {
  const writes: Array<{path: string; seconds: string | null; watched: string | null}> = [];
  const item = (id: string) => ({id, kind: "audio", title: id, stream: `/media/${id}`, progress: {seconds: 0, watched: false}});
  await page.route("https://queue-progress.test/**", route => {
    const path = new URL(route.request().url()).pathname;
    if (path === "/") return route.fulfill({contentType: "text/html", body: `<!doctype html><html lang="en"><title>First track</title><body data-viewer-profile="qa-viewer">
      <audio data-title="First track" data-kind="audio" data-progress="/progress/first" data-start="0" data-queue="/api/v1/audio/first/queue" data-playback-session="qa-drain-session" src="/media/first"></audio>
      <div data-progress-notice hidden><span data-progress-status></span><button data-progress-retry>Retry saving position</button><button data-progress-continue hidden>Continue without saving</button></div>
      </body></html>`});
    if (path === "/api/v1/audio/first/queue") return route.fulfill({json: {items: [item("first"), item("next")]}});
    if (path === "/api/v1/items/next") return route.fulfill({json: {item: item("next"), profileId: "qa-viewer"}});
    if (path.startsWith("/progress/")) {
      const body = new URLSearchParams(route.request().postData() || "");
      writes.push({path, seconds: body.get("seconds"), watched: body.get("watched")});
    }
    return route.fulfill({status: 204});
  });
  await page.goto("/");
  await page.evaluate(() => {
    const audio = document.querySelector("audio")!;
    audio.addEventListener("error", event => event.stopImmediatePropagation(), true);
    let position = 42;
    Object.defineProperties(audio, {
      currentTime: {get: () => position, set: value => position = value}, duration: {value: 100}, readyState: {value: 4},
      play: {value: () => new Promise<void>(resolve => Object.assign(window, {releaseQueuePlayback: resolve}))},
      load: {value: () => queueMicrotask(() => audio.dispatchEvent(new Event("loadedmetadata")))},
    });
  });
  await testInfo.attach("source-provenance", {body: JSON.stringify({sha256: createHash("sha256").update(playerSource).digest("hex")}), contentType: "application/json"});
  try {
    await page.addScriptTag({content: playerSource});
    await page.waitForFunction("audioQueue.length === 1");
    const audio = page.locator("audio");
    await audio.dispatchEvent("ended");
    await expect(audio).toHaveAttribute("data-progress", "/progress/next");
    await expect.poll(() => audio.evaluate((media: HTMLAudioElement) => new URL(media.src).pathname)).toBe("/media/next");
    await page.waitForFunction("typeof window.releaseQueuePlayback === 'function'");
    await audio.evaluate((media: HTMLAudioElement) => {media.currentTime = 3; media.dispatchEvent(new Event("pause"));});
    await page.evaluate(() => (window as Window & {releaseQueuePlayback(): void}).releaseQueuePlayback());
    await expect.poll(() => writes.length, {timeout: 1500}).toBe(2);
    expect(writes).toEqual([{path: "/progress/first", seconds: "0", watched: "true"}, {path: "/progress/next", seconds: "3", watched: null}]);
    await testInfo.attach("progress-http-dispatches", {body: JSON.stringify(writes), contentType: "application/json"});
  } finally {
    await page.evaluate(() => (window as Window & {releaseQueuePlayback?: () => void}).releaseQueuePlayback?.()).catch(() => {});
  }
});
