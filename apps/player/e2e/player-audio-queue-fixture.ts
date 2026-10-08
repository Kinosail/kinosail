import {expect, type Page} from "@playwright/test";
import {playerSource} from "./static-sources";

// Isolated fault injection: a populated Server cannot naturally return a forged
// cross-profile queue projection or deterministically delay a source-load pause.
export const queueItem = (id: string) => ({id, kind: "audio", title: id === "track" ? "First track" : "Next track",
  artist: id === "track" ? "First artist" : "Next artist", album: "Fictional album", track: id === "track" ? 1 : 2,
  stream: `/media/${id}`, artwork: `/art/${id}`, progress: {seconds: 0, watched: false}});
export const queueURL = "https://audio.test/api/v1/audio/track/queue";

export async function openAudio(page: Page, policy: string, homeAssistant = false) {
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
    media.addEventListener("error", (event) => {if (event.isTrusted) event.stopImmediatePropagation();}, true);
    Object.defineProperties(media, {
      currentTime: { value: 12, writable: true },
      duration: { value: 120, configurable: true },
      readyState: { value: HTMLMediaElement.HAVE_ENOUGH_DATA },
      play: { value: async () => {} },
      load: { value: () => queueMicrotask(() => media.dispatchEvent(new Event("loadedmetadata"))), configurable: true },
    });
  });
}


export async function startQueue(page: Page) {
  await page.addScriptTag({content: playerSource});
  await expect(page.getByRole("button", {name: "Next track", exact: true})).toBeEnabled();
  // Explicit user seek intent; loading metadata alone must never claim playback.
  await page.locator("audio").dispatchEvent("kinosail:seek-intent");
}
