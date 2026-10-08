import { readFile } from "node:fs/promises";
import { test } from "@playwright/test";
import { playerSource } from "./static-sources";
import { installAppleFullscreenApi, nativePlayerMarkup } from "./player-apple-fixture";
export function installPlayerExperienceFixture(native = false, apple = false, appleDevice: "iPhone" | "iPad" = "iPhone", beforeSource?: (page: import("@playwright/test").Page, title: string) => Promise<void>) {
test.beforeEach(async ({ page }, testInfo) => {
  if (testInfo.title === "theater control gets out of the way during playback" || testInfo.title.includes("Safari startup")) await page.clock.install();
  let markup = `
    <meta charset="utf-8"><body class="player-page"><main class="player-shell"><div class="media-stage${testInfo.title.includes("blocked autoplay reveals Play") ? " is-busy" : ""}">
      <video id="player-media" data-title="Arrival" data-duration="100" data-start="${testInfo.title.includes("automatic skips") ? "0" : "20"}"${testInfo.title.includes("automatic skips") ? ' data-auto-skip="intro"' : ""} data-progress="/progress/movie" data-playback-session="trace-session" data-playback-trace="https://127.0.0.1:38127/api/v1/items/movie/playback-events"${testInfo.title.includes("limited native fullscreen") || testInfo.title.includes("limited in-band") ? ' data-subtitle-picker-limited="true"' : ""}${testInfo.title.includes("retries requested autoplay") ? " autoplay" : ""}${testInfo.title.includes("resumed autoplay") ? " data-autoplay" : ""}>${testInfo.title.includes("limited in-band") ? '<track kind="subtitles" label="English" data-subtitle-source="/captions.vtt">' : ""}</video>
      <div class="player-stage-toolbar"><strong>Arrival</strong>${testInfo.title.includes("device playback") || testInfo.title.includes("remote playback") || testInfo.title.includes("AirPlay") ? '<div class="player-stage-actions"><button hidden class="quiet" type="button" aria-label="Play on device" data-cast>Play on device</button><span role="status" aria-live="polite" data-cast-state>Available devices use a direct connection to this Server.</span></div>' : ""}</div>
      <div class="player-controls" data-player-controls hidden><button class="player-center-control" type="button" aria-label="Play" data-player-toggle><span data-play-icon></span></button><button class="player-center-control seek-back" type="button" aria-label="Go back 10 seconds" data-player-back>10</button><button class="player-center-control seek-forward" type="button" aria-label="Go forward 10 seconds" data-player-forward>10</button><div class="player-control-dock"><label class="player-scrubber"><span class="sr-only">Seek</span><span class="player-seek-preview" data-seek-preview hidden><span data-seek-frame></span><span data-preview-time>0:00</span></span><input type="range" min="0" max="100" value="0" data-player-seek data-trickplay="/trickplay/movie/{second}"></label><div class="player-control-row"><button type="button" aria-label="Play" data-player-toggle><span data-play-icon></span></button><button type="button" aria-label="Go back 10 seconds" data-player-back>−10</button><button type="button" aria-label="Go forward 10 seconds" data-player-forward>+10</button><button type="button" aria-label="Mute" data-player-mute><span>·</span></button><label class="player-volume"><span class="sr-only">Volume</span><input type="range" min="0" max="1" value="1" step=".05" data-player-volume></label><output data-player-time></output><span class="player-control-spacer"></span><button type="button" aria-label="Subtitles" data-player-captions>CC</button><button type="button" aria-label="Settings" aria-controls="player-settings" aria-expanded="false" data-player-settings><span>·</span></button><button type="button" aria-label="Theater" aria-pressed="false" data-theater><span data-theater-label>▭</span></button><button type="button" aria-label="Enter fullscreen" data-player-fullscreen><span>·</span></button></div></div></div>
      <div class="player-settings" id="player-settings" hidden><button type="button" data-player-settings-close>Close</button><label>Subtitles <select data-subtitles><option value="off">Off</option><option value="0">English</option></select></label></div>
      <div class="player-buffer" role="status" aria-live="polite" data-player-status><span class="buffer-skeleton" aria-hidden="true"></span><span data-player-message>Loading video…</span><button type="button" data-player-fallback hidden>Try again</button><progress hidden max="100" value="0" aria-label="Video buffered" data-buffered>0%</progress></div>
    ${testInfo.title.includes("automatic skips") ? '<button hidden data-marker="intro" data-start="0" data-seek="10">Skip intro</button>' : ""}</div><details class="chapters"><summary><span>Chapters</span><small>15</small></summary><ol class="chapter-list"><li><button type="button" data-chapter data-start="0" data-end="60" data-seek="0"><span>First contact</span><time>0:00</time></button></li><li><button type="button" data-chapter data-start="60" data-end="100" data-seek="60"><span>The answer</span><time>1:00</time></button></li></ol></details></main></body>
  `;
  if (native) markup = nativePlayerMarkup(markup);
  await page.route("https://127.0.0.1:38127/", (route) => route.fulfill({ contentType: "text/html; charset=utf-8", body: markup }));
  await page.route("**/api/v1/items/movie/playback-events", (route) => route.fulfill({ status: 204 }));
  if (testInfo.title.includes("progress save") || testInfo.title.includes("saves mobile progress") || testInfo.title.includes("rejected fullscreen")) await page.goto("https://127.0.0.1:38127/");
  else await page.setContent(markup);
  if (apple) await installAppleFullscreenApi(page, appleDevice);
  await page.evaluate(({withInBand, safariStartup, queuedPause, queuedSeeking, apple}) => {
    if (safariStartup) Object.defineProperty(navigator, "userAgent", {configurable: true, value: "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 Mobile/15E148"});
    try { Object.defineProperty(window, "localStorage", { value: { getItem: () => null, setItem: () => {} } }); } catch {}
    let bufferedEnd = 60;
    let bufferedStart = 0;
    let currentTime = queuedSeeking ? 0 : 20;
    let paused = true;
    let playFailure = "";
    let playPending = false;
    let finishPlay = () => {};
    let rejectPendingPlay = (_name: string) => {};
    let readyState = safariStartup ? 0 : 4;
    let networkState = safariStartup ? 2 : 1;
    const video = document.querySelector("video")!;
    if (safariStartup && !apple) Object.defineProperty(video, "webkitEnterFullscreen", {value: undefined, configurable: true});
    if (safariStartup) {
      let sourceLoaded = false;
      Object.defineProperty(video, "currentSrc", {get: () => sourceLoaded ? "https://127.0.0.1:38127/media/movie" : ""});
      video.addEventListener("loadstart", () => { sourceLoaded = true; });
    }
    const trackEvents = new EventTarget();
    const makeTrack = () => {
      let mode = "disabled";
      return {get mode() { return mode; }, set mode(value: string) {
        if (mode === value) return;
        mode = value;
        queueMicrotask(() => trackEvents.dispatchEvent(new Event("change")));
      }};
    };
    const textTrack = makeTrack();
    const textTracks = Object.assign(withInBand ? [makeTrack(), document.querySelector("track")!.track] : [textTrack, makeTrack()], {addEventListener: trackEvents.addEventListener.bind(trackEvents)});
    if (!withInBand) document.querySelector("[data-subtitles]")!.insertAdjacentHTML("beforeend", '<option value="1">French</option>');
    Object.defineProperties(video, {
      buffered: { configurable: true, get: () => ({ length: 1, start: () => bufferedStart, end: () => bufferedEnd }) },
      currentTime: { get: () => currentTime, set: (value: number) => {
        currentTime = value;
        if (queuedSeeking) queueMicrotask(() => { video.dispatchEvent(new Event("seeking")); video.dispatchEvent(new Event("seeked")); });
      } },
      duration: { configurable: true, value: 100 },
      readyState: { get: () => readyState },
      networkState: { get: () => networkState },
      load: { value() {} },
      paused: { get: () => paused, configurable: true },
      play: { configurable: true, value: async () => { if (playFailure) throw new DOMException("", playFailure); paused = false; video.dispatchEvent(new Event("play")); if (playPending) await new Promise<void>((resolve, reject) => { finishPlay = resolve; rejectPendingPlay = name => reject(new DOMException("synthetic interrupted request", name)); }); video.dispatchEvent(new Event("playing")); } },
      pause: { value: () => { paused = true; if (queuedPause) queueMicrotask(() => video.dispatchEvent(new Event("pause"))); else video.dispatchEvent(new Event("pause")); } },
      volume: { value: 1, writable: true },
      muted: { value: false, writable: true },
      textTracks: { value: textTracks },
    });
    Object.assign(window, {
      setBufferedEnd: (value: number) => { bufferedEnd = value; },
      setBufferedStart: (value: number) => { bufferedStart = value; },
      advanceMediaTime: (value: number) => { currentTime = value; },
      setPaused: (value: boolean) => { paused = value; },
      setPlayFailure: (value: string) => { playFailure = value; },
      setPlayPending: (value: boolean) => { playPending = value; },
      finishPlay: () => finishPlay(),
      rejectPendingPlay: (name: string) => rejectPendingPlay(name),
      setReadyState: (value: number) => { readyState = value; },
      setNetworkState: (value: number) => { networkState = value; },
      textTrack,
    });
  }, {withInBand: testInfo.title.includes("limited in-band"), safariStartup: testInfo.title.includes("Safari startup"), queuedPause: testInfo.title.includes("queued pause"), queuedSeeking: testInfo.title.includes("queued seeking"), apple});
  if (testInfo.title.includes("device playback") || testInfo.title.includes("AirPlay")) await page.evaluate((airplay) => {
    const video = document.querySelector("video")!;
    Object.defineProperty(video, "remote", {configurable: true, value: null});
    Object.defineProperty(video, "webkitShowPlaybackTargetPicker", {configurable: true, value: () => { const context = window as Window & {airplayPickerCalls?: number}; context.airplayPickerCalls = (context.airplayPickerCalls || 0) + 1; }});
    Object.defineProperty(window, "WebKitPlaybackTargetAvailabilityEvent", {configurable: true, value: airplay ? {} : undefined});
  }, testInfo.title.includes("AirPlay") && !testInfo.title.includes("without the availability constructor"));
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
  if (testInfo.title.includes("offline source swap")) await page.evaluate(({ activePlayback, activeHLSBlob, delayedDirectProbe, delayedNegotiation, hlsFallback, immediateOffline, nativeHLS, pendingHLS, rapidMode, serverSkip }) => {
    const context = window as Window & {
      KinosailOfflineMedia: { saveProgress: () => Promise<void>; source: () => Promise<string>; remove: (id: string) => Promise<void>; bindProgress: (media: HTMLMediaElement) => Promise<() => void> };
      resolveOfflineSource: () => void;
      setOfflineProbeStatus: (status: number) => void;
      setPaused: (value: boolean) => void;
      setReadyState: (value: number) => void;
    };
    const video = document.querySelector("video")!;
    const negotiationResolvers: Array<(value: { supported: boolean; smooth: boolean; powerEfficient: boolean }) => void> = [];
    if (delayedNegotiation) {
      video.dataset.playbackApi = "/api/v1/items/movie/playback";
      Object.defineProperty(navigator, "mediaCapabilities", { configurable: true, value: {
        decodingInfo: () => new Promise((resolve) => negotiationResolvers.push(resolve)),
      }});
      Object.assign(context, { releaseNegotiation: () => negotiationResolvers.splice(0).forEach((resolve) => resolve({ supported: true, smooth: true, powerEfficient: false })) });
    }
    context.setPaused(!activePlayback);
    context.setReadyState(activePlayback ? 4 : 0);
    video.currentTime = activePlayback ? 42 : 0;
    const blobFallback = hlsFallback || activeHLSBlob;
    let source = pendingHLS ? "" : blobFallback ? "blob:stream" : nativeHLS ? "/hls/movie/index.m3u8" : "/media/movie";
    if (delayedDirectProbe || rapidMode) {
      source = "/media/direct";
      video.dataset.direct = source;
      video.dataset.adaptive = "/hls/movie/index.m3u8";
      video.dataset.playbackPolicy = "automatic";
      video.dataset.compatibilityLabel = "Remux";
      if (delayedDirectProbe) Object.defineProperty(video, "error", {configurable: true, get: () => ({code: MediaError.MEDIA_ERR_DECODE})});
    }
    if (blobFallback && !serverSkip) {
      video.dataset.direct = "/media/direct";
    }
    if (activeHLSBlob || pendingHLS) {
      video.dataset.hls = "/hls/movie/index.m3u8";
      video.dataset.playbackPolicy = "compatible";
      video.dataset.playbackOverride = "true";
      video.dataset.compatibilityLabel = "Remux";
      if (serverSkip) video.dataset.autoSkip = "server";
      if (pendingHLS) Object.defineProperty(video, "canPlayType", {configurable: true, value: () => ""});
    }
    if (nativeHLS) {
      video.dataset.hls = source;
      video.dataset.compatibilityLabel = "Remux";
      Object.defineProperty(video, "canPlayType", { configurable: true, value: (type: string) => type === "application/vnd.apple.mpegurl" ? "probably" : "" });
      Object.defineProperty(video, "currentSrc", { configurable: true, get: () => source });
    }
    const readAttribute = video.getAttribute.bind(video);
    video.getAttribute = (name: string) => name.toLowerCase() === "src" ? source : readAttribute(name);
    Object.defineProperty(video, "src", { configurable: true, get: () => source, set: (value) => {
      source = value;
      video.currentTime = 0;
      context.setPaused(true);
    } });
    let resolveOfflineSource = (_source: string) => {};
    let resolveDirectProbe = (_response: Response) => {};
    const directProbe = new Promise<Response>((resolve) => { resolveDirectProbe = resolve; });
    const removedOffline: string[] = [];
    let offlineProbeStatus = 206;
    const networkFetch = window.fetch.bind(window);
    window.fetch = (input, init) => {
      const url = typeof input === "string" ? input : input.url;
      if (delayedDirectProbe && url.endsWith("/media/direct")) return directProbe;
      return url.includes("/offline-media/") ? Promise.resolve(new Response(offlineProbeStatus === 206 ? "x" : null, { status: offlineProbeStatus, headers: offlineProbeStatus === 206 ? { "Content-Range": "bytes 0-0/1" } : {} })) : networkFetch(input, init);
    };
    context.KinosailOfflineMedia = { saveProgress: async () => {}, bindProgress: async (media) => {
      const position = media.currentTime || Number(media.dataset.start) || 0;
      const loaded = () => { media.currentTime = position; };
      media.addEventListener("loadedmetadata", loaded, {once: true});
      return () => media.removeEventListener("loadedmetadata", loaded);
    }, source: () => immediateOffline ? Promise.resolve("/offline-media/profile/movie-job") : new Promise((resolve) => { resolveOfflineSource = resolve; }), remove: async (id) => { removedOffline.push(id); } };
    context.resolveOfflineSource = () => resolveOfflineSource("/offline-media/profile/movie-job");
    Object.assign(context, {releaseDirectProbe: () => resolveDirectProbe(new Response("x", {status: 206}))});
    context.setOfflineProbeStatus = (status) => { offlineProbeStatus = status; };
    Object.assign(window, { removedOffline });
    document.querySelector(".media-stage")?.insertAdjacentHTML("afterend", `<button data-playback-mode-status></button><span data-playback-reason></span><span data-playback-method-detail></span><div data-playback-recovery hidden><p data-playback-recovery-message></p><button data-player-fallback></button></div><fieldset data-playback-mode><label><input type="radio" name="playback-policy" value="direct-first" checked>Direct first</label><label><input type="radio" name="playback-policy" value="direct-only">Direct only</label><label><input type="radio" name="playback-policy" value="compatible">Compatible</label></fieldset>${nativeHLS || activeHLSBlob || delayedDirectProbe || pendingHLS || rapidMode ? '<label data-quality-control><select data-quality></select><span data-quality-state></span></label>' : ""}`);
  }, {
    activePlayback: testInfo.title.includes("active playback"),
    activeHLSBlob: testInfo.title.includes("active HLS blob"),
    delayedDirectProbe: testInfo.title.includes("delayed direct probe"),
    delayedNegotiation: testInfo.title.includes("delayed negotiation"),
    hlsFallback: testInfo.title.includes("HLS fallback"),
    immediateOffline: testInfo.title.includes("immediately ready offline"),
    nativeHLS: testInfo.title.includes("native HLS"),
    pendingHLS: testInfo.title.includes("pending HLS loader"),
    rapidMode: testInfo.title.includes("rapid playback mode"),
    serverSkip: testInfo.title.includes("server-skip"),
  });
  if (testInfo.title.includes("localized offline")) await page.evaluate(() => Object.assign(document.body.dataset, {
    offlineCopy: "Copia local",
    offlineDescription: "Archivo verificado en este dispositivo.",
    offlineReady: "Listo sin conexión",
    playbackMethod: "Método de reproducción",
    openPlaybackSettings: "Abrir ajustes.",
  }));
  await page.addStyleTag({ content: await readFile("../../../packages/webassets/static/player-app.css", "utf8") });
  if (native) await page.addStyleTag({content: (await Promise.all([readFile("../../../packages/webassets/static/last-light.css", "utf8"), readFile("../internal/server/static/home.css", "utf8")])).join("\n")});
  if (native && testInfo.title.includes("unsupported fullscreen")) await page.evaluate(() => {
    Object.defineProperty(document, "fullscreenEnabled", {configurable: true, value: false});
    Object.defineProperty(document.querySelector("video"), "webkitEnterFullscreen", {configurable: true, value: undefined});
  });
  if (native && testInfo.title.includes("touch native playback")) await page.evaluate(() => Object.defineProperty(navigator, "maxTouchPoints", {configurable: true, value: 1}));
  const playerScript = playerSource;
  if (testInfo.title.includes("native fullscreen fallback") || testInfo.title.includes("both fullscreen APIs")) await page.evaluate((both) => {
    Object.defineProperty(document, "fullscreenEnabled", {configurable: true, value: both});
    const video = document.querySelector("video")!;
    Object.defineProperty(video, "webkitEnterFullscreen", {configurable: true, value: () => {
      const state = window as Window & {nativeFullscreenCalls?: number};
      state.nativeFullscreenCalls = (state.nativeFullscreenCalls || 0) + 1;
    }});
  }, testInfo.title.includes("both fullscreen APIs"));
  if (testInfo.title.includes("pending HLS loader")) {
    await page.evaluate(() => {
      const originalAppend = document.head.append.bind(document.head);
      document.head.append = ((...nodes: (Node | string)[]) => {
        const script = nodes.find((node): node is HTMLScriptElement => node instanceof HTMLScriptElement && node.src.includes("/static/hls.min.js"));
        if (!script) return originalAppend(...nodes);
        Object.assign(window, {hlsSources: [], releaseHlsLoader: () => {
          class TestHls {
            static Events = {MANIFEST_PARSED: "manifest", LEVEL_SWITCHED: "level", ERROR: "error"};
            static ErrorTypes = {NETWORK_ERROR: "network", MEDIA_ERROR: "media"};
            static isSupported() { return true; }
            autoLevelEnabled = true; levels = [];
            on() {} loadSource(source: string) { (window as Window & {hlsSources: string[]}).hlsSources.push(source); } destroy() {}
            attachMedia(media: HTMLVideoElement | {media: HTMLVideoElement}) { ("media" in media ? media.media : media).src = "blob:stream"; }
          }
          Object.assign(window, {Hls: TestHls});
          script.onload?.(new Event("load"));
        }});
        return script;
      }) as typeof document.head.append;
    });
  }
  const hlsFixture = testInfo.title.includes("active HLS blob") || testInfo.title.includes("delayed direct probe") || testInfo.title.includes("rapid playback mode") ? `
    window.hlsSources = [];
    class TestHls {
      static Events = {MANIFEST_PARSED: "manifest", LEVEL_SWITCHED: "level", ERROR: "error"};
      static ErrorTypes = {NETWORK_ERROR: "network", MEDIA_ERROR: "media"};
      static isSupported() { return true; }
      autoLevelEnabled = true; levels = [];
      on() {} loadSource(source) { window.hlsSources.push(source); } destroy() {}
      attachMedia(media) { (media.media || media).src = "blob:stream"; }
    }
    window.Hls = TestHls;
  ` : "";
  const availabilityFixture = testInfo.title.includes("without the availability constructor")
    ? 'Object.defineProperty(window, "WebKitPlaybackTargetAvailabilityEvent", {configurable: true, value: undefined});'
    : "";
  await beforeSource?.(page, testInfo.title);
  await page.addScriptTag({ content: `${availabilityFixture}\n${hlsFixture}\n${playerScript}` });
});
}
