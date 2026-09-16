let playbackTimelineSeek;
const mediaTime = Object.getOwnPropertyDescriptor(HTMLMediaElement.prototype, "currentTime");
const mediaDuration = Object.getOwnPropertyDescriptor(HTMLMediaElement.prototype, "duration");
if (!Object.hasOwn(player, "currentTime") && mediaTime && mediaDuration) Object.defineProperties(player, {
  currentTime: {configurable: true, get: () => Number.isFinite(playbackTimelineSeek) ? playbackTimelineSeek : mediaTime.get.call(player) + playbackTimelineOffset, set: (seconds) => {
    const current = mediaTime.get.call(player) + playbackTimelineOffset;
    playbackTimelineSeek = playbackTimelineOffset && Math.abs(seconds - current) >= 0.1 ? seconds : undefined;
    mediaTime.set.call(player, Math.max(0, seconds - playbackTimelineOffset));
  }},
  duration: {configurable: true, get: () => { const value = mediaDuration.get.call(player); return Number.isFinite(value) ? value + playbackTimelineOffset : value; }},
});
const direct = player.dataset.direct;
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
const mediaVideo = (contentType) => ({
  contentType,
  width: Number(player.dataset.mediaWidth) || 1920,
  height: Number(player.dataset.mediaHeight) || 1080,
  bitrate: Number(player.dataset.mediaBitrate) || 8000000,
  framerate: Number(player.dataset.mediaFramerate) || 30,
});
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
const codecTypes = [
  ["av1", 'video/mp4; codecs="av01.0.08M.08"'],
  ["hevc", 'video/mp4; codecs="hvc1.1.6.L123.B0"'],
  ["vp9", 'video/mp4; codecs="vp09.00.10.08"'],
  ["h264", 'video/mp4; codecs="avc1.64002a"'],
];
const supportsCodec = async ([codec, contentType]) => {
  try {
    if (navigator.mediaCapabilities?.decodingInfo) {
      const result = await navigator.mediaCapabilities.decodingInfo({type: "media-source", video: mediaVideo(contentType)});
      return result.supported && result.smooth && (codec === "h264" || result.powerEfficient);
    }
  } catch (_) {}
  return (typeof MediaSource !== "undefined" && MediaSource.isTypeSupported(contentType)) || player.canPlayType(contentType) !== "";
};
const negotiateStream = async (generation) => {
  if (!stream || !player.dataset.playbackApi) return;
  // These plans copy the original video (or contain only audio). Detecting
  // alternative video encoders cannot improve them and delays first playback.
  if (["remux", "audio-transcode"].includes(player.dataset.compatibilityMode)) return;
  const controller = new AbortController();
  let expire;
  const budget = new Promise((resolve) => { expire = setTimeout(() => { controller.abort(); resolve(null); }, 1500); });
  try {
    const codecs = await Promise.race([Promise.all(codecTypes.map(async (codec) => await supportsCodec(codec) ? codec[0] : "")), budget]);
    if (!codecs || controller.signal.aborted || generation !== adaptiveGeneration || destroyed) return;
    const supported = codecs.filter(Boolean);
    if (!supported.some((codec) => codec !== "h264")) return;
    const response = await fetch(`${player.dataset.playbackApi}?videoCodecs=${encodeURIComponent(supported.join(","))}`, {signal: controller.signal});
    if (!response.ok || !response.body) return;
    const reader = response.body.getReader();
    const chunks = [];
    let size = 0;
    try {
      while (true) {
        const {done, value} = await reader.read();
        if (done) break;
        size += value.byteLength;
        if (size > 1048576) { await reader.cancel(); return; }
        chunks.push(value);
      }
    } finally { reader.releaseLock(); }
    if (controller.signal.aborted || generation !== adaptiveGeneration || destroyed) return;
    const bytes = new Uint8Array(size);
    let offset = 0;
    for (const chunk of chunks) { bytes.set(chunk, offset); offset += chunk.byteLength; }
    const result = JSON.parse(new TextDecoder("utf-8", {fatal: true}).decode(bytes));
    const compatible = result?.compatible;
    const plan = result?.compatiblePlan;
    if (typeof compatible !== "string" || compatible.length > 2048 || !compatible.startsWith("/hls/") || /[\\\u0000-\u0020]/.test(compatible) ||
      !plan || plan.allowed !== true || !["remux", "audio-transcode", "transcode"].includes(plan.mode)) return;
    const url = new URL(compatible, playbackURLBase);
    if (url.origin !== new URL(playbackURLBase).origin || !url.pathname.startsWith("/hls/") || url.username || url.password || url.hash) return;
    stream = withPlaybackSession(compatible);
    player.dataset.compatibilityMode = plan.mode;
    if (typeof result.compatibleLabel === "string" && result.compatibleLabel.length <= 128) player.dataset.compatibilityLabel = result.compatibleLabel;
    if (typeof result.compatibleDescription === "string" && result.compatibleDescription.length <= 1024) player.dataset.compatibilityDescription = result.compatibleDescription;
  } catch (_) {} finally { clearTimeout(expire); }
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
