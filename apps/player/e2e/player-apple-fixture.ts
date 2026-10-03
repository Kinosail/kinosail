import type {Page} from "@playwright/test";

// Only the isolated policy tests use this simulation; media is simulated separately.
export async function installAppleFullscreenApi(page: Page, device: "iPhone" | "iPad" = "iPhone") {
  await page.evaluate(device => {
    Object.defineProperty(navigator, "userAgent", {configurable: true, value: `Mozilla/5.0 (${device}; CPU OS 18_0 like Mac OS X) AppleWebKit/605.1.15 Mobile/15E148`});
    const video = document.querySelector("video")!;
    let fullscreen = false;
    Object.defineProperties(video, {
      webkitEnterFullscreen: {value: () => { fullscreen = true; video.dispatchEvent(new Event("webkitbeginfullscreen")); }, configurable: true},
      webkitExitFullscreen: {value: () => { fullscreen = false; video.dispatchEvent(new Event("webkitendfullscreen")); }},
      webkitDisplayingFullscreen: {get: () => fullscreen},
    });
    video.setAttribute("playsinline", "");
    video.setAttribute("data-autoplay", "");
  }, device);
}

export function nativePlayerMarkup(markup: string) {
    markup = markup.replace('<video id="player-media"', '<video controls data-native-controls id="player-media"');
    markup = markup.replace('<strong>Arrival</strong>', '<strong>Arrival</strong><button type="button" aria-label="Settings" aria-controls="player-settings" aria-expanded="false" data-player-settings>Settings</button><button type="button" aria-label="Enter fullscreen" data-player-fullscreen>Fullscreen</button>');
    const start = markup.indexOf('<div class="player-controls"');
    const end = markup.indexOf('<div class="player-settings"', start);
    markup = markup.slice(0, start) + `<div class="player-controls player-native-controls" data-player-controls hidden><button class="player-center-control" type="button" aria-label="Play" data-player-toggle><span data-play-icon></span></button></div>` + markup.slice(end);
  return markup;
}
