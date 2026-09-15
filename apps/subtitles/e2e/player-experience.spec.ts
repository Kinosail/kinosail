import { test } from "@playwright/test";
import { registerPlayerControlTests } from "./player-experience-control-tests";
import { registerPlayerDeviceTests } from "./player-experience-device-tests";
import { registerPlayerStatusTests } from "./player-experience-status-tests";

test.beforeEach(async ({ page, request }, testInfo) => {
  const markup = `
    <meta charset="utf-8"><body class="player-page"><main class="player-shell"><div class="media-stage">
      <video id="player-media" data-title="Arrival" data-duration="100" data-start="20" data-progress="/progress/movie" data-playback-session="trace-session" data-playback-trace="https://127.0.0.1:38128/api/v1/items/movie/playback-events"${testInfo.title.includes("retries requested autoplay") ? " autoplay" : ""}${testInfo.title.includes("resumed autoplay") ? " data-autoplay" : ""}></video>
      <div class="player-stage-toolbar"><strong>Arrival</strong>${testInfo.title.includes("device playback") || testInfo.title.includes("remote playback") || testInfo.title.includes("AirPlay") ? '<div class="player-stage-actions"><button hidden class="quiet" type="button" aria-label="Play on device" data-cast>Play on device</button><span role="status" aria-live="polite" data-cast-state>Available devices use a direct connection to this Server.</span></div>' : ""}</div>
      <div class="player-controls" data-player-controls hidden><button class="player-center-control" type="button" aria-label="Play" data-player-toggle><span data-play-icon></span></button><button class="player-center-control seek-back" type="button" aria-label="Go back 10 seconds" data-player-back>10</button><button class="player-center-control seek-forward" type="button" aria-label="Go forward 10 seconds" data-player-forward>10</button><div class="player-control-dock"><label class="player-scrubber"><span class="sr-only">Seek</span><input type="range" min="0" max="100" value="0" data-player-seek></label><div class="player-control-row"><button type="button" aria-label="Play" data-player-toggle><span data-play-icon></span></button><button type="button" aria-label="Go back 10 seconds" data-player-back>−10</button><button type="button" aria-label="Go forward 10 seconds" data-player-forward>+10</button><button type="button" aria-label="Mute" data-player-mute><span>·</span></button><label class="player-volume"><span class="sr-only">Volume</span><input type="range" min="0" max="1" value="1" step=".05" data-player-volume></label><output data-player-time></output><span class="player-control-spacer"></span><button type="button" aria-label="Subtitles" data-player-captions>CC</button><button type="button" aria-label="Settings" aria-controls="player-settings" aria-expanded="false" data-player-settings><span>·</span></button><button type="button" aria-label="Theater" aria-pressed="false" data-theater><span data-theater-label>▭</span></button><button type="button" aria-label="Enter fullscreen" data-player-fullscreen><span>·</span></button></div></div></div>
      <div class="player-settings" id="player-settings" hidden><button type="button" data-player-settings-close>Close</button><label>Subtitles <select data-subtitles><option value="off">Off</option><option value="0">English</option></select></label></div>
      <div class="player-buffer" role="status" aria-live="polite" data-player-status><span data-player-message>Loading video…</span><progress hidden max="100" value="0" aria-label="Video buffered" data-buffered>0%</progress></div>
    </div><details class="chapters"><summary><span>Chapters</span><small>15</small></summary><ol class="chapter-list"><li><button type="button" data-chapter data-start="0" data-end="60" data-seek="0"><span>First contact</span><time>0:00</time></button></li><li><button type="button" data-chapter data-start="60" data-end="100" data-seek="60"><span>The answer</span><time>1:00</time></button></li></ol></details></main></body>
  `;
  await page.route("https://127.0.0.1:38128/", (route) => route.fulfill({ contentType: "text/html; charset=utf-8", body: markup }));
  await page.route("**/api/v1/items/movie/playback-events", (route) => route.fulfill({ status: 204 }));
  await page.setContent(markup);
  await page.evaluate(() => {
    try { Object.defineProperty(window, "localStorage", { value: { getItem: () => null, setItem: () => {} } }); } catch {}
    let bufferedEnd = 60;
    let paused = true;
    let playFailure = "";
    let readyState = 4;
    const video = document.querySelector("video")!;
    const textTrack = { mode: "disabled" };
    Object.defineProperties(video, {
      buffered: { get: () => ({ length: 1, start: () => 0, end: () => bufferedEnd }) },
      currentTime: { value: 20, writable: true },
      duration: { value: 100 },
      readyState: { get: () => readyState },
      load: { value() {} },
      paused: { get: () => paused, configurable: true },
      play: { value: async () => { if (playFailure) throw new DOMException("", playFailure); paused = false; video.dispatchEvent(new Event("play")); video.dispatchEvent(new Event("playing")); } },
      pause: { value: () => { paused = true; video.dispatchEvent(new Event("pause")); } },
      volume: { value: 1, writable: true },
      muted: { value: false, writable: true },
      textTracks: { value: [textTrack] },
    });
    Object.assign(window, {
      setBufferedEnd: (value: number) => { bufferedEnd = value; },
      setPaused: (value: boolean) => { paused = value; },
      setPlayFailure: (value: string) => { playFailure = value; },
      setReadyState: (value: number) => { readyState = value; },
      textTrack,
    });
  });
  if (testInfo.title.includes("device playback") || testInfo.title.includes("AirPlay")) await page.evaluate((airplay) => {
    const video = document.querySelector("video")!;
    Object.defineProperty(video, "remote", {configurable: true, value: null});
    Object.defineProperty(video, "webkitShowPlaybackTargetPicker", {configurable: true, value: () => { const context = window as Window & {airplayPickerCalls?: number}; context.airplayPickerCalls = (context.airplayPickerCalls || 0) + 1; }});
    Object.defineProperty(window, "WebKitPlaybackTargetAvailabilityEvent", {configurable: true, value: airplay ? {} : undefined});
  }, testInfo.title.includes("AirPlay"));
  if (testInfo.title.includes("remote playback")) await page.evaluate(() => {
    let availability: ((available: boolean) => void) | undefined;
    const remote = new EventTarget() as EventTarget & {
      prompt: () => Promise<void>;
      state: string;
      watchAvailability: (callback: (available: boolean) => void) => Promise<number>;
      unwatchAvailability: (id: number) => void;
    };
    Object.defineProperties(remote, {
      state: {configurable: true, get: () => "disconnected"},
      prompt: {configurable: true, value: async () => {
        remote.dispatchEvent(new Event("connecting"));
        remote.dispatchEvent(new Event("connect"));
      }},
      watchAvailability: {configurable: true, value: (callback: (available: boolean) => void) => {
        availability = callback;
        callback(false);
        return Promise.resolve(7);
      }},
      unwatchAvailability: {configurable: true, value: () => {availability = undefined;}}
    });
    Object.defineProperty(document.querySelector("video"), "remote", {configurable: true, value: remote});
    Object.assign(window, {setRemoteAvailability: (available: boolean) => availability?.(available), remote});
  });
  if (testInfo.title.includes("Picture-in-Picture")) await page.evaluate((webkit) => {
    const video = document.querySelector("video")!;
    if (webkit) {
      let mode = "inline";
      Object.defineProperty(document, "pictureInPictureEnabled", {configurable: true, value: false});
      Object.defineProperty(video, "requestPictureInPicture", {configurable: true, value: undefined});
      Object.defineProperty(video, "webkitPresentationMode", {configurable: true, get: () => mode});
      Object.defineProperty(video, "webkitSupportsPresentationMode", {configurable: true, value: (value: string) => value === "picture-in-picture"});
      Object.defineProperty(video, "webkitSetPresentationMode", {configurable: true, value: (value: string) => {
        mode = value;
        video.dispatchEvent(new Event("webkitpresentationmodechanged"));
      }});
    } else {
      let active = false;
      Object.defineProperty(document, "pictureInPictureEnabled", {configurable: true, value: true});
      Object.defineProperty(document, "pictureInPictureElement", {configurable: true, get: () => active ? video : null});
      Object.defineProperty(video, "requestPictureInPicture", {configurable: true, value: async () => {
        active = true;
        video.dispatchEvent(new Event("enterpictureinpicture"));
      }});
      Object.defineProperty(document, "exitPictureInPicture", {configurable: true, value: async () => {
        active = false;
        video.dispatchEvent(new Event("leavepictureinpicture"));
      }});
    }
    const button = document.createElement("button");
    button.type = "button";
    button.hidden = true;
    button.setAttribute("aria-label", "Picture-in-Picture");
    button.setAttribute("aria-pressed", "false");
    button.dataset.playerPip = "";
    document.querySelector("[data-player-fullscreen]")?.before(button);
  }, testInfo.title.includes("WebKit Picture-in-Picture"));
  const [stylesheet, playerScript] = await Promise.all([request.get("/static/app.css"), request.get("/static/player.js")]);
  await page.addStyleTag({ content: await stylesheet.text() });
  await page.addScriptTag({ content: await playerScript.text() });
});

registerPlayerStatusTests();
registerPlayerDeviceTests();
registerPlayerControlTests();
