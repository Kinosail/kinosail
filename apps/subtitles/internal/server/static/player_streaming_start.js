const streaming = (() => {
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
  if (playbackChoice(playbackPolicy)?.disabled) playbackPolicy = "direct-only";
  if (explicitPolicy) playerStorage.set(policyPreference, playbackPolicy);
  setPlaybackMode(playbackPolicy);
  let hls;
  let adaptiveActive = false;
  let adaptiveStarting = false;
  let adaptiveOffset = 0;
  let adaptiveSeekSwitch = false;
  let networkRecoveries = 0;
  let mediaRecoveries = 0;
  let hlsLoader;
  let streamNegotiated = false;
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
    const label = compatible ? player.dataset.compatibilityLabel || "Compatibility" : player.dataset.playbackLabel || "Direct Play";
    const description = compatible ? player.dataset.compatibilityDescription || "Uses the smallest compatible change." : player.dataset.playbackDescription || "Original video and audio. No conversion.";
    const status = pending ? `Starting ${label}` : label;
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
  const negotiateStream = async () => {
    if (!stream || !player.dataset.playbackApi) return;
    try {
      const supported = (await Promise.all(codecTypes.map(async (codec) => await supportsCodec(codec) ? codec[0] : ""))).filter(Boolean);
      if (!supported.some((codec) => codec !== "h264")) return;
      const response = await fetch(`${player.dataset.playbackApi}?videoCodecs=${encodeURIComponent(supported.join(","))}`);
      if (!response.ok) return;
      const result = await response.json();
      const compatible = result.compatible;
      if (typeof compatible === "string" && compatible.startsWith("/hls/")) stream = withPlaybackSession(compatible);
      if (result.compatiblePlan && ["remux", "audio-transcode", "transcode"].includes(result.compatiblePlan.mode)) player.dataset.compatibilityMode = result.compatiblePlan.mode;
      if (typeof result.compatibleLabel === "string") player.dataset.compatibilityLabel = result.compatibleLabel;
      if (typeof result.compatibleDescription === "string") player.dataset.compatibilityDescription = result.compatibleDescription;
    } catch (_) {}
  };
  const loadHls = () => {
    if (typeof Hls !== "undefined") return Promise.resolve(true);
    if (hlsLoader) return hlsLoader;
    hlsLoader = new Promise((resolve) => {
      const script = document.createElement("script");
      script.src = "/static/hls.min.js?v=1.7.1";
      script.onload = () => resolve(typeof Hls !== "undefined");
      script.onerror = () => resolve(false);
      document.head.append(script);
    });
    return hlsLoader;
  };
  const qualityLabel = (level) => {
    const url = Array.isArray(level?.url) ? level.url[0] : level?.url;
    return url?.match(/\/([1-9]\d{2,3}p)\/index\.m3u8(?:\?|$)/)?.[1] || (level?.height ? `${level.height}p` : "Auto");
  };
  const resumeAfterSourceChange = (forcePlay = false) => {
    const seconds = player.currentTime;
    const playing = forcePlay || !player.paused;
    player.addEventListener("loadedmetadata", () => {
      if (Number.isFinite(seconds) && seconds >= 0 && seconds < player.duration) player.currentTime = seconds;
      if (playing) requestPlay("source-change").catch(() => {});
    }, {once: true});
  };
  const showQuality = (level) => {
    const label = qualityLabel(level);
    qualityState.textContent = hls?.autoLevelEnabled ? `Auto · ${label}` : label;
  };
  const useOriginal = (forcePlay = false) => {
    if (!direct) return;
    resumeAfterSourceChange(forcePlay);
    hls?.destroy();
    hls = undefined;
    adaptiveActive = false;
    playbackTraceMethod = "direct";
    playbackTrace("source-direct", forcePlay ? "manual-retry" : "selected");
    player.src = player.dataset.direct;
    player.load();
    if (quality) quality.value = "original";
    if (qualityState) qualityState.textContent = "Original";
    showPlaybackMode(false, true);
  };
  const streamAt = (seconds) => {
    const offset = fullDuration && seconds >= 30 && seconds < fullDuration ? Math.floor(seconds / 30) * 30 : 0;
    const source = new URL(stream, playbackURLBase);
    source.pathname = source.pathname.replace(/-o\d+(?=\/index\.m3u8$)/, "");
    if (offset) source.pathname = source.pathname.replace(/\/index\.m3u8$/, `-o${offset * 1000}/index.m3u8`);
    return {offset, source: `${source.pathname}${source.search}${source.hash}`};
  };
  const useAdaptive = (preference = "auto", resume = false, target = resume ? player.currentTime : Number(player.dataset.start) || 0) => {
    if (resume) resumeAfterSourceChange();
    hls?.destroy();
    adaptiveActive = true;
    playbackTraceMethod = player.dataset.compatibilityMode || "transcode";
    playbackTrace("source-compatible", resume ? "resume" : "selected");
    showPlaybackMode(true, true);
    const selected = streamAt(target);
    adaptiveOffset = selected.offset;
    hls = new Hls({
      startLevel: -1,
      testBandwidth: true,
      startFragPrefetch: true,
      capLevelToPlayerSize: true,
      capLevelOnFPSDrop: true,
      abrMaxWithRealBitrate: true,
      xhrSetup: (xhr) => xhr.setRequestHeader("X-Playback-Session", playbackSession),
      ...(selected.offset ? {startPosition: target - selected.offset, timelineOffset: selected.offset} : {}),
    });
    networkRecoveries = 0;
    mediaRecoveries = 0;
    hls.on(Hls.Events.MANIFEST_PARSED, (_, {levels}) => {
      playbackTrace("hls-manifest", "parsed", `${levels.length}-levels`);
      adaptiveSeekSwitch = false;
      quality.replaceChildren(new Option("Auto", "auto"));
      levels.forEach((level, index) => quality.add(new Option(qualityLabel(level), `level:${index}`)));
      if (direct) quality.add(new Option("Original", "original"));
      qualityControl.hidden = false;
      const selected = levels.findIndex((level) => qualityLabel(level) === preference);
      if (selected >= 0) {
        hls.nextLevel = selected;
        quality.value = `level:${selected}`;
      } else {
        hls.loadLevel = -1;
        quality.value = "auto";
      }
    });
    hls.on(Hls.Events.LEVEL_SWITCHED, (_, {level}) => {
      showQuality(hls.levels[level]);
      playbackTrace("hls-level", "switched", qualityLabel(hls.levels[level]));
    });
    hls.on(Hls.Events.ERROR, (_, data) => {
      playbackTrace("hls-error", `${data.type || "unknown"}:${data.details || "unknown"}`);
      if (!data.fatal) return;
      if (!hls.autoLevelEnabled) {
        hls.loadLevel = -1;
        quality.value = "auto";
        qualityState.textContent = "Auto · recovered";
