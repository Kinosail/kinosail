const resumeAfterSourceChange = (forcePlay = false, sourceReady = false, retainedTime) => {
  const previous = pendingResume;
  previous?.cancel();
  const seconds = Number.isFinite(retainedTime) ? retainedTime : previous?.seconds ?? (sourceReady || player.readyState ? player.currentTime : Number(player.dataset.start) || 0);
  const playing = forcePlay || previous?.playing || !player.paused;
  const resume = () => {
    if (pendingResume?.resume !== resume) return;
    const {seconds, playing} = pendingResume;
    pendingResume = undefined;
    if (Number.isFinite(seconds) && seconds >= 0 && seconds < player.duration && Math.abs(player.currentTime - seconds) >= 0.1) setPlayerTime(seconds);
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
  adaptiveOriginController?.abort();
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
  player.src = direct;
  player.load();
  if (quality) quality.value = "original";
  if (qualityState) qualityState.textContent = "Original";
  showPlaybackMode(false, true);
};
const streamOffset = (seconds) => Number.isFinite(seconds) && seconds >= 0.1 && seconds <= 604800 && (!fullDuration || seconds < fullDuration) ? Math.floor(seconds * 10) / 10 : 0;
const streamAt = (seconds) => {
  const offset = streamOffset(seconds);
  const source = new URL(stream, playbackURLBase);
  source.pathname = source.pathname.replace(/-o\d+(?=\/index\.m3u8$)/, "");
  if (offset) source.pathname = source.pathname.replace(/\/index\.m3u8$/, `-o${Math.round(offset * 1000)}/index.m3u8`);
  return {offset, source: `${source.pathname}${source.search}${source.hash}`};
};
const streamOrigin = (value, requested) => {
  if (value === null) return requested;
  if (!/^\d{1,6}(?:\.\d{1,9})?$/.test(value)) throw new Error("invalid-stream-origin");
  const origin = Number(value);
  if (!Number.isFinite(origin) || origin < 0 || origin > requested + .000001 || origin > 604800) throw new Error("invalid-stream-origin");
  return origin;
};
const streamClock = value => {
  if (value === null) return undefined;
  if (!/^\d(?:\.\d{1,9})?$/.test(value) || Number(value) > 1) throw new Error("invalid-stream-clock");
  return Number(value);
};
const nativeStreamAt = async (position, generation) => {
  const selected = streamAt(position);
  if (!selected.offset || !["remux", "audio-transcode"].includes(player.dataset.compatibilityMode)) return selected;
  adaptiveOriginController?.abort();
  const controller = new AbortController();
  adaptiveOriginController = controller;
  const timeout = setTimeout(() => controller.abort(), 30000);
  try {
    const response = await fetch(selected.source, {method: "HEAD", signal: controller.signal,
      headers: {"X-Playback-Session": playbackSession}});
    if (generation !== adaptiveGeneration || destroyed) return undefined;
    if (!response.ok) throw new Error("stream-origin-unavailable");
    selected.offset = streamOrigin(response.headers.get("X-Kinosail-HLS-Origin"), selected.offset);
    return selected;
  } finally {
    clearTimeout(timeout);
    if (adaptiveOriginController === controller) adaptiveOriginController = undefined;
  }
};
const useAdaptive = (preference = "auto", resume = false, target = resume ? pendingResume?.seconds ?? player.currentTime : Number(player.dataset.start) || 0) => {
  cancelNetworkRecovery();
  adaptiveOriginController?.abort();
  resumeAfterSourceChange(!resume && networkWantsPlay, true, target);
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
  let certifiedOffset;
  const bufferController = class extends Hls.DefaultConfig.bufferController {
    onBufferAppending(event, data) {
      if (certifiedOffset !== undefined && data.offset !== undefined && ["video", "audiovideo"].includes(data.type))
        data = {...data, offset: certifiedOffset};
      return super.onBufferAppending(event, data);
    }
  };
  hls = new Hls({
    bufferController,
    backBufferLength: 90,
    lowLatencyMode: false,
    startLevel: -1,
    testBandwidth: true,
    startFragPrefetch: true,
    capLevelToPlayerSize: true,
    capLevelOnFPSDrop: true,
    abrMaxWithRealBitrate: true,
    xhrSetup: (xhr, url) => {
      xhr.setRequestHeader("X-Playback-Session", playbackSession);
      if (new URL(url, playbackURLBase).pathname !== new URL(selected.source, playbackURLBase).pathname) return;
      xhr.addEventListener("readystatechange", () => {
        if (xhr.readyState !== 2 || hls !== activeHls) return;
        try {
          const sourceOrigin = xhr.getResponseHeader("X-Kinosail-HLS-Origin");
          const origin = streamOrigin(sourceOrigin, selected.offset);
          const clock = streamClock(xhr.getResponseHeader("X-Kinosail-HLS-Clock"));
          if ((clock === undefined) !== (sourceOrigin === null)) throw new Error("incomplete-stream-clock");
          certifiedOffset = clock === undefined ? undefined : origin - clock;
          adaptiveOffset = origin;
          activeHls.config.timelineOffset = origin;
          activeHls.config.startPosition = target - origin;
        } catch (_) {
          const retained = pendingResume?.seconds ?? target;
          playbackTrace("hls-error", "invalid-stream-metadata");
          activeHls.destroy();
          showFailure("Playback is unavailable. You can retry from this position.", "Retry playback", () => retryStalledPlayback(retained));
        }
      });
    },
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
  let initialTarget = pendingResume?.seconds ?? (Number(player.dataset.start) || 0);
  let explicitSeek = false, pendingAtExplicitSeek;
  const retainInitialSeek = () => {
    const pending = pendingResume;
    queueMicrotask(() => {
      if (generation !== adaptiveGeneration) return;
      initialTarget = player.currentTime;
      explicitSeek = true; pendingAtExplicitSeek = pending;
    });
  };
  const target = () => resume ? pendingResume?.seconds ?? player.currentTime :
    pendingResume && (!explicitSeek || pendingResume !== pendingAtExplicitSeek) ? pendingResume.seconds : initialTarget;
  if (!resume) player.addEventListener("kinosail:seek-intent", retainInitialSeek);
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
  const needsHls = typeof Hls === "undefined" && window.kinosailPlaybackCapabilities.needsAdapter(player);
  const adapter = needsHls ? loadHls() : Promise.resolve(true);
  try {
    const [, adapterLoaded] = await Promise.all([negotiation, adapter]);
    if (generation !== adaptiveGeneration) return;
    const preference = playerStorage.get(qualityPreference) || "auto";
    qualityControl.hidden = false;
    qualityState.textContent = "Auto";
    if (player.dataset.hls && typeof Hls !== "undefined" && Hls.isSupported()) {
      useAdaptive(preference === "original" ? "auto" : preference, resume, target());
    } else {
      if (typeof Hls !== "undefined" && Hls.isSupported()) useAdaptive(preference === "original" ? "auto" : preference, resume, target());
      else if (player.canPlayType("application/vnd.apple.mpegurl")) {
        let position, selected;
        do {
          position = target();
          selected = await nativeStreamAt(position, generation);
          if (!selected || generation !== adaptiveGeneration || destroyed) return;
        } while (streamOffset(position) !== streamOffset(target()));
        position = target();
        adaptiveActive = true;
        adaptiveSeekSwitch = resume;
        playbackTimelineOffset = selected.offset;
        playbackTimelineSeek = selected.offset || position === 0 ? position : undefined;
        playbackTraceMethod = "native-hls";
        playbackTrace("source-compatible", "native-hls");
        player.addEventListener("loadedmetadata", () => {
          if (generation !== adaptiveGeneration) return;
          adaptiveSeekSwitch = false; playbackTimelineSeek = undefined;
          const current = player.currentTime;
          if (resume && pendingResume?.seconds === position && Number.isFinite(current))
            pendingResume.seconds = Math.max(position, current);
        }, {once: true});
        resumeAfterSourceChange(!resume && networkWantsPlay, true, position);
        player.removeEventListener("loadedmetadata", resumeFromSavedProgress);
        if (player.readyState === HTMLMediaElement.HAVE_NOTHING) mediaTime.set.call(player, 0);
        player.src = selected.source;
        player.load();
      } else {
        const loaded = needsHls ? adapterLoaded : await loadHls();
        if (generation !== adaptiveGeneration) return;
        if (loaded && Hls.isSupported()) useAdaptive(preference === "original" ? "auto" : preference, resume, target());
        else if (player.dataset.fallback) location.replace(player.dataset.fallback);
        else qualityState.textContent = "Playback unavailable";
      }
    }
  } catch (_) {
    if (generation === adaptiveGeneration && !destroyed) {
      const retained = target();
      playbackTrace("hls-error", "stream-origin-unavailable");
      showFailure("Playback is unavailable. You can retry from this position.", "Retry playback", () => {
        clearRecovery();
        resumeAfterSourceChange(true, true, retained);
        startAdaptive(true);
      });
    }
  } finally {
    player.removeEventListener("kinosail:seek-intent", retainInitialSeek);
    if (generation === adaptiveGeneration) adaptiveStarting = false;
  }
};
