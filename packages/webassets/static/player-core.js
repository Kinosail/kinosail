const player = document.querySelector("video,audio");
if (!player) throw new Error("playable media element is missing");
const playerStorage = {
  get: (key) => { try { return localStorage.getItem(key) || ""; } catch (_) { return ""; } },
  set: (key, value) => { try { localStorage.setItem(key, value); } catch (_) {} },
};
const playbackSession = player.dataset.playbackSession || crypto.randomUUID?.() || `${Date.now()}-${Math.random()}`;
const csrf = document.querySelector('meta[name="kinosail-csrf"]')?.content;
const playbackTraceStarted = performance.now();
let playbackTraceSequence = 0;
let playbackTraceMethod = player.dataset.hls ? player.dataset.compatibilityMode || "native-hls" : "direct";
let playbackTimelineOffset = 0;
const traceNumber = (value) => Number.isFinite(value) && value >= 0 ? Math.min(31622400000, Math.round(value)) : 0;
const traceToken = (value) => String(value || "").replace(/[^a-zA-Z0-9_.:-]/g, "_").slice(0, 64);
const bufferedAhead = () => {
  const current = player.currentTime - playbackTimelineOffset;
  for (let index = 0; index < player.buffered.length; index++) {
    if (player.buffered.start(index) <= current && player.buffered.end(index) >= current) return player.buffered.end(index) - current;
  }
  return 0;
};
const playbackTraceQueue = [];
const flushPlaybackTrace = () => playbackTraceQueue.splice(0).forEach((body) => fetch(player.dataset.playbackTrace, {
  method: "POST",
  headers: {"Content-Type": "application/json", ...(csrf ? {"X-Kinosail-CSRF": csrf} : {})},
  body,
  keepalive: true,
}).catch(() => undefined));
const playbackTrace = (event, detail = "", quality = "") => {
  if (!player.dataset.playbackTrace) return;
  const frames = player.getVideoPlaybackQuality?.();
  if (playbackTraceQueue.length >= 32) flushPlaybackTrace();
  playbackTraceQueue.push(JSON.stringify({session: playbackSession, event, sequence: ++playbackTraceSequence, elapsedMs: traceNumber(performance.now() - playbackTraceStarted), positionMs: traceNumber(player.currentTime * 1000), durationMs: traceNumber(player.duration * 1000), bufferedAheadMs: traceNumber(bufferedAhead() * 1000), readyState: player.readyState, networkState: player.networkState, paused: player.paused, method: playbackTraceMethod, detail: traceToken(detail), quality: traceToken(quality), visibility: document.visibilityState, errorCode: player.error?.code || 0, droppedFrames: frames?.droppedVideoFrames || 0, totalFrames: frames?.totalVideoFrames || 0}));
  if (["playing", "waiting", "stalled", "error", "hls-error", "play-rejected"].includes(event)) setTimeout(flushPlaybackTrace);
};
const requestPause = () => { player.dispatchEvent(new CustomEvent("kinosail:playback-intent", {detail: {playing: false}})); player.pause(); };
const requestPlay = (detail) => {
  player.dispatchEvent(new CustomEvent("kinosail:playback-intent", {detail: {playing: true}}));
  playbackTrace("play-request", detail);
  const rejected = (error) => { playbackTrace("play-rejected", `${detail}:${error?.name || "Error"}`); throw error; };
  try { return Promise.resolve(player.play()).catch(rejected); } catch (error) { return Promise.reject(error).catch(rejected); }
};
const playbackURLBase = location.origin === "null" ? "https://kinosail.invalid/" : location.href;
const withPlaybackSession = (source) => {
  if (!source) return source;
  const url = new URL(source, playbackURLBase);
  url.searchParams.set("playbackSession", playbackSession);
  return `${url.pathname}${url.search}${url.hash}`;
};
for (const event of ["loadstart", "loadedmetadata", "canplay", "play", "playing", "waiting", "stalled", "seeking", "seeked", "pause", "ended", "progress"]) player.addEventListener(event, () => playbackTrace(event, "media-event"));
player.addEventListener("error", () => playbackTrace("error", "media-error"));
if (player.requestVideoFrameCallback) {
  let frameRequest;
  let firstFrame = true;
  let previousMediaTime;
  let movingFrame = false;
  let afterSeek = false;
  let seekTarget;
  const traceFrame = () => {
    if (frameRequest !== undefined) player.cancelVideoFrameCallback?.(frameRequest);
    frameRequest = player.requestVideoFrameCallback((_, metadata) => {
      frameRequest = undefined;
      if (player.seeking) return;
      if (afterSeek && Number.isFinite(seekTarget) && Math.abs(metadata.mediaTime - seekTarget) > 1) { traceFrame(); return; }
      if (firstFrame) {
        playbackTrace("first-frame", `presented-${metadata.presentedFrames || 0}`);
        firstFrame = false;
      }
      if (afterSeek) {
        playbackTrace("frame-after-seek", `presented-${metadata.presentedFrames || 0}`);
        afterSeek = false;
      }
      if (!movingFrame && Number.isFinite(previousMediaTime) && metadata.mediaTime > previousMediaTime) {
        playbackTrace("first-moving-frame", `presented-${metadata.presentedFrames || 0}`);
        movingFrame = true;
      }
      previousMediaTime = metadata.mediaTime;
      if (!movingFrame) traceFrame();
    });
  };
  player.addEventListener("loadstart", () => { firstFrame = true; movingFrame = false; afterSeek = false; seekTarget = undefined; previousMediaTime = undefined; traceFrame(); });
  player.addEventListener("playing", () => { if (firstFrame || !movingFrame) traceFrame(); });
  player.addEventListener("seeking", () => { previousMediaTime = undefined; if (frameRequest !== undefined) player.cancelVideoFrameCallback?.(frameRequest); frameRequest = undefined; });
  player.addEventListener("seeked", () => { afterSeek = true; seekTarget = player.currentTime - playbackTimelineOffset; traceFrame(); });
  window.addEventListener("pagehide", () => { if (frameRequest !== undefined) player.cancelVideoFrameCallback?.(frameRequest); });
  traceFrame();
}
if (player.autoplay || player.hasAttribute("data-autoplay")) player.addEventListener("canplay", () => { if (player.paused) requestPlay("autoplay-canplay").catch(() => {}); }, {once: true});
playbackTrace("session-start", player.dataset.playbackPolicy || "automatic");
const isPictureInPicture = () => document.pictureInPictureElement === player || player.webkitPresentationMode === "picture-in-picture";
