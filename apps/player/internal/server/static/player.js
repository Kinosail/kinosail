let playbackTimelineSeek;
const mediaTime = Object.getOwnPropertyDescriptor(HTMLMediaElement.prototype, "currentTime");
const mediaDuration = Object.getOwnPropertyDescriptor(HTMLMediaElement.prototype, "duration");
if (!Object.hasOwn(player, "currentTime") && mediaTime && mediaDuration) Object.defineProperties(player, {
  currentTime: {configurable: true, get: () => Number.isFinite(playbackTimelineSeek) ? playbackTimelineSeek : mediaTime.get.call(player) + playbackTimelineOffset, set: (seconds) => {
    const current = mediaTime.get.call(player) + playbackTimelineOffset;
    playbackTimelineSeek = playbackTimelineOffset && Math.abs(seconds - current) >= 0.1 ? seconds : undefined;
    mediaTime.set.call(player, Math.max(0, seconds - playbackTimelineOffset));
  }},
  duration: {configurable: true, get: () => { if (adaptiveActive && !hls && fullDuration) return fullDuration; const value = mediaDuration.get.call(player); return Number.isFinite(value) ? value + playbackTimelineOffset : value; }},
});
// Resume reporting the decoder's time once the requested native HLS seek completes.
player.addEventListener("seeked", () => { if (!adaptiveSeekSwitch) playbackTimelineSeek = undefined; });
const direct = player.dataset.direct || (player.tagName === "VIDEO" && !player.dataset.hls && !player.dataset.adaptive ? player.getAttribute("src") : "");
let stream = player.dataset.hls || player.dataset.adaptive;
const expectedDuration = Number(player.dataset.duration);
const fullDuration = Number.isFinite(expectedDuration) && expectedDuration > 0 && expectedDuration <= 31622400 ? expectedDuration : 0;
const qualityControl = document.querySelector("[data-quality-control]");
const quality = document.querySelector("[data-quality]");
const qualityState = document.querySelector("[data-quality-state]");
const playbackMode = document.querySelector("[data-playback-mode]");
const playbackChoice = (value) => playbackMode?.querySelector(`input[value="${value}"]`);
const setPlaybackMode = (value) => { const choice = playbackChoice(value); if (choice) choice.checked = true; };
const playbackModeStatus = document.querySelector("[data-playback-mode-status]");
const playbackReason = document.querySelector("[data-playback-reason]");
const playbackDetail = document.querySelector("[data-playback-method-detail]");
const playbackRecovery = document.querySelector("[data-playback-recovery]");
const recoveryMessage = document.querySelector("[data-playback-recovery-message]");
const fallbackButtons = document.querySelectorAll("[data-player-fallback]");
const qualityPreference = "kinosail.stream-quality";
const policyPreference = "kinosail.playback-policy-v2";
const serverPolicy = {automatic: "direct-first", direct: "direct-only", compatible: "compatible"}[player.dataset.playbackPolicy] || "direct-first";
const explicitPolicy = player.dataset.playbackOverride === "true";
let playbackPolicy = explicitPolicy ? serverPolicy : playerStorage.get(policyPreference) || serverPolicy;
if (!["direct-first", "direct-only", "compatible"].includes(playbackPolicy)) playbackPolicy = serverPolicy;
if ((!stream && !player.dataset.compatibleUrl) || playbackChoice(playbackPolicy)?.disabled) playbackPolicy = "direct-only";
if (explicitPolicy) playerStorage.set(policyPreference, playbackPolicy);
setPlaybackMode(playbackPolicy);
let hls;
let adaptiveActive = false;
let adaptiveStarting = false;
let adaptiveGeneration = 0;
let adaptiveOffset = 0;
let adaptiveSeekSwitch = false;
let mediaRecoveries = 0;
let hlsLoader;
let streamNegotiated = false;
let destroyed = false;
let pendingResume;
const codecCapabilities = window.kinosailPlaybackCapabilities;
const playbackMediaFacts = {width: player.dataset.mediaWidth, height: player.dataset.mediaHeight,
  bitrate: player.dataset.mediaBitrate, framerate: player.dataset.mediaFramerate};
const mediaVideo = (contentType, compatible = false) => codecCapabilities.video(playbackMediaFacts, contentType, compatible);
if (player.dataset.directType && navigator.mediaCapabilities?.decodingInfo) navigator.mediaCapabilities.decodingInfo({type: "file", video: mediaVideo(player.dataset.directType)})
  .then((result) => playbackTrace("capability-direct", result.supported ? result.smooth ? "smooth" : "supported" : "unsupported"))
  .catch(() => {});
const showPlaybackMode = (compatible, pending = false) => {
  if (player.dataset.offline === "true") {
    const offlineLabel = document.body.dataset.offlineCopy || "Offline copy";
    const offlineDescription = document.body.dataset.offlineDescription || "Verified file stored on this device.";
    const playbackMethod = document.body.dataset.playbackMethod || "Playback method";
    const openSettings = document.body.dataset.openPlaybackSettings || "Open playback settings.";
    const status = pending ? `${offlineLabel}…` : offlineLabel;
    if (playbackModeStatus) {
      playbackModeStatus.textContent = status;
      playbackModeStatus.setAttribute("aria-label", `${playbackMethod}: ${status}. ${openSettings}`);
    }
    if (playbackReason) playbackReason.textContent = pending ? status : `${offlineLabel} · ${offlineDescription}`;
    if (playbackDetail) playbackDetail.textContent = status;
    return;
  }
  const label = compatible ? player.dataset.compatibilityLabel || "Compatibility" : player.dataset.playbackLabel || "Direct Play";
  const description = compatible ? player.dataset.compatibilityDescription || "Uses the smallest compatible change." : player.dataset.playbackDescription || "Original video and audio. No conversion.";
  const status = label;
  if (playbackModeStatus) {
    playbackModeStatus.textContent = status;
    playbackModeStatus.setAttribute("aria-label", `Playback method: ${status}. Open playback settings.`);
  }
  if (playbackReason) playbackReason.textContent = `${label} · ${description}`;
  if (playbackDetail) playbackDetail.textContent = label;
};
const codecTypes = codecCapabilities.codecs;
const supportsCodec = (codec) => codecCapabilities.supports(player, playbackMediaFacts, codec);
const negotiateStream = async (generation, position, seekRequired = false) => {
  if (generation !== adaptiveGeneration || destroyed) return;
  const copied = ["remux", "audio-transcode"].includes(player.dataset.compatibilityMode);
  if (seekRequired && !copied) return;
  const unavailable = () => { throw new Error("Compatible seek plan is unavailable"); };
  if (!stream || !player.dataset.playbackApi) { if (seekRequired) unavailable(); return; }
  // These plans copy the original video (or contain only audio). Detecting
  // alternative video encoders cannot improve them and delays first playback.
  if (copied && !seekRequired) return;
  const controller = new AbortController();
  let expire;
  const budget = new Promise((resolve) => { expire = setTimeout(() => { controller.abort(); resolve(null); }, 1500); });
  try {
    const origin = new URL(playbackURLBase).origin;
    const current = new URL(stream, origin), api = new URL(player.dataset.playbackApi, origin);
    const recipePattern = /^([rat])-a(0|[1-9]\d{0,2})-s(0|[1-9]\d{0,2})-(none|text|image|external)-t[01]-b(0|[1-9]\d{0,8})(?:-c(?:av1|hevc|vp9))?(?:-z[1-9]\d{1,3}x[1-9]\d{1,3})?(?:-k[0-9a-z]{1,10}_[0-9a-z]{1,10}(?:\.[0-9a-z]{1,10}_[0-9a-z]{1,10}){0,31})?(?:-o[1-9]\d{0,8})?(?:-e[1-3])?$/;
    const match = current.pathname.match(/^\/hls\/([a-f0-9]{16})\/p\/([rat]-[A-Za-z0-9_.-]{1,2048})\/index\.m3u8$/);
    if (!match || stream.length > 2048 || current.origin !== origin || current.hash || current.username || current.password ||
      api.origin !== origin || api.pathname !== `/api/v1/items/${match[1]}/playback` || api.search || api.hash || api.username || api.password) unavailable();
    const selectedRecipe = match[2].match(recipePattern);
    if (!selectedRecipe || seekRequired && (!Number.isFinite(position) || Object.is(position, -0) || position < 0 || position > 604800 || Math.abs(position * 10 - Math.round(position * 10)) > 0.0000001)) unavailable();
    const codecs = copied ? ["h264"] : await Promise.race([Promise.all(codecTypes.map(async (codec) => await supportsCodec(codec) ? codec[0] : "")), budget]);
    if (!codecs || controller.signal.aborted || generation !== adaptiveGeneration || destroyed) return;
    const supported = codecs.filter(Boolean);
    if (!seekRequired && !supported.some((codec) => codec !== "h264")) return;
    api.searchParams.set("videoCodecs", supported.join(","));
    if (seekRequired) {
      api.searchParams.set("position", String(position));
      api.searchParams.set("recipe", match[2].replace(/-o\d+(?=-e[1-3]$|$)/, ""));
    }
    const response = await fetch(api.href, {signal: controller.signal, credentials: "same-origin", redirect: "error"});
    if (!response.ok || !response.body || response.url !== api.href || !/^application\/json(?:\s*;|$)/i.test(response.headers.get("content-type") || "")) unavailable();
    const reader = response.body.getReader();
    const chunks = [];
    let size = 0;
    try {
      while (true) {
        const {done, value} = await reader.read();
        if (done) break;
        size += value.byteLength;
        if (size > 1048576) { await reader.cancel(); unavailable(); }
        chunks.push(value);
      }
    } finally { reader.releaseLock(); }
    if (controller.signal.aborted) { if (seekRequired) unavailable(); return; }
    if (generation !== adaptiveGeneration || destroyed) return;
    const bytes = new Uint8Array(size);
    let offset = 0;
    for (const chunk of chunks) { bytes.set(chunk, offset); offset += chunk.byteLength; }
    const raw = new TextDecoder("utf-8", {fatal: true}).decode(bytes);
    const keys = []; let tokens = 0;
    for (const token of raw.matchAll(/"(?:\\.|[^"\\])*"|[{}\[\]]/g)) {
      if (++tokens > 30000) unavailable();
      const value = token[0];
      if (value === "{" || value === "[") { keys.push(value === "{" ? new Set() : null); if (keys.length > 8) unavailable(); }
      else if (value === "}" || value === "]") keys.pop();
      else if (/^\s*:/.test(raw.slice(token.index + value.length))) {
        const currentKeys = keys.at(-1), key = JSON.parse(value);
        if (!currentKeys || currentKeys.has(key) || currentKeys.size >= 128) unavailable();
        currentKeys.add(key);
      }
    }
    const result = JSON.parse(raw);
    const compatible = result?.compatible;
    const plan = result?.compatiblePlan;
    const responseKeys = new Set(["policy", "media", "plan", "compatiblePlan", "compatibleLabel", "compatibleDescription", "qualities", "directAllowed", "direct", "compatibleDuration", "compatibleProgressToken", "compatible", "download", "directType", "summary", "duration", "start", "audio", "chapters", "markers", "autoSkip", "subtitles", "subtitleLanguage", "subtitlePickerLimited", "next", "downloadNext", "trickplay", "progressToken", "replayGain"]);
    const planKeys = new Set(["allowed", "mode", "reason", "container", "videoCodec", "audioCodec", "subtitleMode", "colorMode", "audioIndex", "subtitleIndex", "subtitleSourceIndex", "subtitleText", "subtitleExternal", "subtitleExternalIndex", "maxBitrate", "width", "height", "adaptive", "qualities", "markerMode", "timeline", "audioCompatibilityRequired"]);
    if (!result || Array.isArray(result) || Object.keys(result).some((key) => !responseKeys.has(key)) || !plan || Array.isArray(plan) || Object.keys(plan).some((key) => !planKeys.has(key))) unavailable();
    if (typeof compatible !== "string" || compatible.length > 2048 || !compatible.startsWith("/hls/") || /[\\\u0000-\u0020]/.test(compatible) ||
      !plan || plan.allowed !== true || !["remux", "audio-transcode", "transcode"].includes(plan.mode) ||
      typeof result.compatibleLabel !== "string" || result.compatibleLabel.length > 128 ||
      typeof result.compatibleDescription !== "string" || result.compatibleDescription.length > 1024) unavailable();
    const url = new URL(compatible, playbackURLBase);
    const mode = {remux: "r", "audio-transcode": "a", transcode: "t"}[plan.mode];
    const returned = url.pathname.match(/^\/hls\/([a-f0-9]{16})\/p\/([^/]+)\/index\.m3u8$/);
    const returnedRecipe = returned?.[2].match(recipePattern);
    if (url.origin !== origin || returned?.[1] !== match[1] || !returnedRecipe || returnedRecipe[1] !== mode ||
      url.username || url.password || url.hash || url.search || !Number.isInteger(plan.audioIndex) || plan.audioIndex < 0 || plan.audioIndex > 255 ||
      Number(returnedRecipe[2]) !== plan.audioIndex || returnedRecipe[2] !== selectedRecipe[2] || returnedRecipe[3] !== selectedRecipe[3] ||
      returnedRecipe[4] !== selectedRecipe[4] || returnedRecipe[5] !== selectedRecipe[5] || /-e[1-3]$/.exec(returned[2])?.[0] !== /-e[1-3]$/.exec(match[2])?.[0] ||
      result.compatibleLabel !== {remux: "Remux", "audio-transcode": "Transcoding audio", transcode: "Transcoding video"}[plan.mode]) unavailable();
    stream = withPlaybackSession(compatible);
    player.dataset.compatibilityMode = plan.mode;
    if (typeof result.compatibleLabel === "string" && result.compatibleLabel.length <= 128) player.dataset.compatibilityLabel = result.compatibleLabel;
    if (typeof result.compatibleDescription === "string" && result.compatibleDescription.length <= 1024) player.dataset.compatibilityDescription = result.compatibleDescription;
  } catch (_) { if (seekRequired) unavailable(); } finally { clearTimeout(expire); }
};
const loadHls = () => {
  if (typeof Hls !== "undefined") return Promise.resolve(true);
  if (hlsLoader) return hlsLoader;
  hlsLoader = new Promise((resolve) => {
    const script = document.createElement("script");
    const finish = (loaded) => {
      clearTimeout(timeout);
      script.onload = script.onerror = null;
      if (!loaded) { script.remove(); hlsLoader = undefined; }
      resolve(loaded);
    };
    const timeout = setTimeout(() => finish(false), 5000);
    script.src = "/static/hls.min.js?v=1.7.1";
    script.onload = () => finish(typeof Hls !== "undefined");
    script.onerror = () => finish(false);
    document.head.append(script);
  });
  return hlsLoader;
};
const qualityLabel = (level) => {
  const url = Array.isArray(level?.url) ? level.url[0] : level?.url;
  return url?.match(/\/([1-9]\d{2,3}p)\/index\.m3u8(?:\?|$)/)?.[1] || (level?.height ? `${level.height}p` : "Auto");
};
