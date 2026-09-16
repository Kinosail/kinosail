const resumeAfterSourceChange = (forcePlay = false, sourceReady = false, retainedTime) => {
  const previous = pendingResume;
  previous?.cancel();
  const seconds = Number.isFinite(retainedTime) ? retainedTime : previous?.seconds ?? (sourceReady || player.readyState ? player.currentTime : Number(player.dataset.start) || 0);
  const playing = forcePlay || previous?.playing || !player.paused;
  const resume = () => {
    if (pendingResume?.resume !== resume) return;
    const {seconds, playing} = pendingResume;
    pendingResume = undefined;
    if (Number.isFinite(seconds) && seconds >= 0 && seconds < player.duration && Math.abs(player.currentTime - seconds) >= 0.1) player.currentTime = seconds;
    if (playing) requestPlay("source-change").catch(() => {});
  };
  const cancel = () => {
    player.removeEventListener("loadedmetadata", resume);
    if (pendingResume?.resume === resume) pendingResume = undefined;
  };
  pendingResume = {seconds, playing, resume, cancel};
  player.addEventListener("loadedmetadata", resume, {once: true});
  return cancel;
};
const showQuality = (level) => {
  const label = qualityLabel(level);
  qualityState.textContent = hls?.autoLevelEnabled ? `Auto · ${label}` : label;
};
const useOriginal = (forcePlay = false, retainedTime) => {
  if (!direct) return;
  cancelNetworkRecovery();
  adaptiveGeneration += 1;
  adaptiveStarting = false;
  resumeAfterSourceChange(forcePlay, Number.isFinite(retainedTime), retainedTime);
  delete player.dataset.offline;
  hls?.destroy();
  hls = undefined;
  adaptiveActive = false;
  playbackTimelineOffset = 0;
  playbackTimelineSeek = undefined;
  playbackTraceMethod = "direct";
  playbackTrace("source-direct", forcePlay ? "manual-retry" : "selected");
  if (qualityControl && stream) qualityControl.hidden = false;
  player.src = player.dataset.direct;
  player.load();
  if (quality) quality.value = "original";
  if (qualityState) qualityState.textContent = "Original";
  showPlaybackMode(false, true);
};
const streamOffset = (seconds) => fullDuration && seconds >= 0.1 && seconds < fullDuration ? Math.floor(seconds * 10) / 10 : 0;
const streamAt = (seconds) => {
  const offset = streamOffset(seconds);
  const source = new URL(stream, playbackURLBase);
  source.pathname = source.pathname.replace(/-o\d+(?=\/index\.m3u8$)/, "");
  if (offset) source.pathname = source.pathname.replace(/\/index\.m3u8$/, `-o${Math.round(offset * 1000)}/index.m3u8`);
  return {offset, source: `${source.pathname}${source.search}${source.hash}`};
};
const useAdaptive = (preference = "auto", resume = false, target = resume ? pendingResume?.seconds ?? player.currentTime : Number(player.dataset.start) || 0) => {
  cancelNetworkRecovery();
  if (resume) resumeAfterSourceChange(false, true, target);
  delete player.dataset.offline;
  hls?.destroy();
  playbackTimelineOffset = 0;
  adaptiveActive = true;
  playbackTraceMethod = player.dataset.compatibilityMode || "transcode";
  playbackTrace("source-compatible", resume ? "resume" : "selected");
  if (qualityControl) qualityControl.hidden = false;
  showPlaybackMode(true, true);
  const selected = streamAt(target);
  adaptiveOffset = selected.offset;
  playbackTimelineSeek = undefined;
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
  const activeHls = hls;
  mediaRecoveries = 0;
  hls.on(Hls.Events.MANIFEST_PARSED, (_, {levels}) => {
    if (hls !== activeHls) return;
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
    if (hls !== activeHls) return;
    showQuality(hls.levels[level]);
    playbackTrace("hls-level", "switched", qualityLabel(hls.levels[level]));
  });
  hls.on(Hls.Events.ERROR, (_, data) => {
    if (hls !== activeHls) return;
    playbackTrace("hls-error", `${data.type || "unknown"}:${data.details || "unknown"}`);
    if (!data.fatal) return;
    if (!hls.autoLevelEnabled) {
      hls.loadLevel = -1;
      quality.value = "auto";
      qualityState.textContent = "Auto · recovered";
      playerStorage.set(qualityPreference, "auto");
    }
    if (data.type === Hls.ErrorTypes.NETWORK_ERROR) {
      const failed = hls;
      scheduleNetworkRetry(() => { if (hls === failed) failed.startLoad(); }, () => { networkRetryCount = 0; if (hls === failed) failed.startLoad(); });
    }
    else if (data.type === Hls.ErrorTypes.MEDIA_ERROR && mediaRecoveries++ < 1) hls.recoverMediaError();
    else if (direct && !player.dataset.adaptive) useOriginal();
    else qualityState.textContent = "Playback unavailable";
  });
  hls.loadSource(selected.source);
  hls.attachMedia(fullDuration ? {media: player, overrides: {duration: fullDuration}} : player);
};
const startAdaptive = async (resume = false) => {
  if (adaptiveActive || adaptiveStarting || !stream) return;
  const generation = ++adaptiveGeneration;
  adaptiveStarting = true;
  showPlaybackMode(true, true);
  let negotiation = Promise.resolve();
  if (player.dataset.adaptive && !streamNegotiated) {
    streamNegotiated = true;
    negotiation = negotiateStream(generation);
  } else if (player.dataset.hls && !streamNegotiated) {
    streamNegotiated = true;
    negotiation = negotiateStream(generation);
  }
  // Load the decoder adapter alongside negotiation, only after compatible playback was selected.
  const needsHls = typeof Hls === "undefined" && !player.canPlayType("application/vnd.apple.mpegurl");
  const adapter = needsHls ? loadHls() : Promise.resolve(true);
  const [, adapterLoaded] = await Promise.all([negotiation, adapter]);
  if (generation !== adaptiveGeneration) return;
  const preference = playerStorage.get(qualityPreference) || "auto";
  qualityControl.hidden = false;
  qualityState.textContent = "Auto";
  if (player.dataset.hls && typeof Hls !== "undefined" && Hls.isSupported()) {
    useAdaptive(preference === "original" ? "auto" : preference, resume);
  } else {
    if (typeof Hls !== "undefined" && Hls.isSupported()) useAdaptive(preference === "original" ? "auto" : preference, resume);
    else if (player.canPlayType("application/vnd.apple.mpegurl")) {
      const target = resume ? pendingResume?.seconds ?? player.currentTime : Number(player.dataset.start) || 0;
      if (resume) resumeAfterSourceChange(false, true, target);
      const selected = streamAt(target);
      adaptiveActive = true;
      playbackTimelineOffset = selected.offset;
      playbackTimelineSeek = undefined;
      playbackTraceMethod = "native-hls";
      playbackTrace("source-compatible", "native-hls");
      player.addEventListener("loadedmetadata", () => { adaptiveSeekSwitch = false; }, {once: true});
      player.src = selected.source;
      player.load();
    } else {
      const loaded = needsHls ? adapterLoaded : await loadHls();
      if (generation !== adaptiveGeneration) return;
      if (loaded && Hls.isSupported()) useAdaptive(preference === "original" ? "auto" : preference, resume);
      else if (player.dataset.fallback) location.replace(player.dataset.fallback);
      else qualityState.textContent = "Playback unavailable";
    }
  }
  if (generation === adaptiveGeneration) adaptiveStarting = false;
};
