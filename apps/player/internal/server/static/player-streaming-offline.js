let detachOfflineProgress;
const itemID = decodeURIComponent(location.pathname.startsWith("/watch/") ? location.pathname.slice("/watch/".length) : "");
window.KinosailOfflineMedia?.source(itemID).then(async (source) => {
  if (destroyed || !source) return;
  const detach = await window.KinosailOfflineMedia.bindProgress(player, source, document.querySelector("[data-player-message]"));
  if (destroyed) { detach(); return; }
  detachOfflineProgress = detach;
  cancelNetworkRecovery();
  adaptiveGeneration += 1;
  adaptiveStarting = false;
  const fallbackUsedHls = Boolean(hls);
  const retainedSource = [player.currentSrc, player.src].find((candidate) => candidate && !candidate.startsWith("blob:"));
  const fallbackSource = retainedSource || direct;
  const fallbackAdaptive = Boolean(stream) && (Boolean(retainedSource) || !direct) && (adaptiveActive || playbackTraceMethod !== "direct");
  const fallbackTraceMethod = fallbackAdaptive ? playbackTraceMethod : "direct";
  const fallbackTimelineOffset = fallbackAdaptive ? playbackTimelineOffset : 0;
  const fallbackTime = pendingResume?.seconds ?? (player.readyState ? player.currentTime : Number(player.dataset.start) || 0);
  const offlineJob = decodeURIComponent(source.split("/").at(-1) || "");
  pendingResume?.cancel();
  const audioChoice = document.querySelector("[data-audio-track]");
  const audioStatus = document.querySelector("[data-audio-status]");
  if (audioChoice) audioChoice.disabled = true;
  if (audioStatus) audioStatus.textContent = document.body.dataset.offlineAudio || "Audio is fixed in this downloaded copy.";
  hls?.destroy();
  hls = undefined;
  adaptiveActive = false;
  playbackTimelineOffset = 0;
  playbackTimelineSeek = undefined;
  playbackTraceMethod = "offline";
  playbackTrace("source-compatible", "offline");
  player.dataset.offline = "true";
  player.dataset.offlineLoading = "true";
  if (qualityControl) qualityControl.hidden = true;
  let offlineLoaded = false;
  let failed;
  const loaded = () => {
    if (destroyed || player.dataset.offline !== "true" || player.getAttribute("src") !== source) return;
    offlineLoaded = true;
    delete player.dataset.offlineLoading;
    if (networkWantsPlay) requestPlay("offline-source").catch(() => {});
    showPlaybackMode(false, false);
    const state = document.querySelector("[data-player-status]");
    const message = document.querySelector("[data-player-message]");
    if (state && message) { state.hidden = false; message.textContent = document.body.dataset.offlineReady || "Ready offline on this device"; }
  };
  failed = async () => {
    if (destroyed || player.dataset.offline !== "true") return;
    player.removeEventListener("loadeddata", loaded);
    if (audioChoice) audioChoice.disabled = false;
    if (audioStatus) audioStatus.textContent = "";
    const recoveryTime = offlineLoaded ? player.currentTime : fallbackTime;
    const recoveryWasPlaying = networkWantsPlay;
    detachOfflineProgress?.();
    detachOfflineProgress = undefined;
    delete player.dataset.offlineLoading;
    delete player.dataset.offline;
    if (qualityControl && stream) qualityControl.hidden = false;
    adaptiveActive = fallbackAdaptive;
    playbackTraceMethod = fallbackTraceMethod;
    playbackTimelineOffset = fallbackTimelineOffset;
    playbackTimelineSeek = undefined;
    if (fallbackUsedHls && !fallbackSource && stream && typeof Hls !== "undefined" && Hls.isSupported()) {
      adaptiveActive = false;
      resumeAfterSourceChange(recoveryWasPlaying, true, recoveryTime);
      useAdaptive(playerStorage.get(qualityPreference) || "auto", false, recoveryTime);
    } else if (fallbackAdaptive && stream && !fallbackSource) {
      adaptiveActive = false;
      resumeAfterSourceChange(recoveryWasPlaying, true, recoveryTime);
      startAdaptive(true);
    } else if (fallbackSource === direct && !fallbackAdaptive) {
      useOriginal(recoveryWasPlaying, recoveryTime);
    } else if (fallbackSource) {
      resumeAfterSourceChange(recoveryWasPlaying, true, recoveryTime);
      player.src = fallbackSource;
      player.load();
    }
    showPlaybackMode(fallbackAdaptive, false);
    const state = document.querySelector("[data-player-status]");
    const message = document.querySelector("[data-player-message]");
    if (state && message) { state.hidden = false; message.textContent = document.body.dataset.offlineUnavailable || "Offline copy is unavailable."; }
    try { await window.KinosailOfflineMedia?.remove?.(offlineJob); }
    catch (_) {}
  };
  player.addEventListener("loadeddata", loaded, {once: true});
  player.addEventListener("error", failed, {once: true});
  player.src = source;
  player.load();
  showPlaybackMode(false, true);
}).catch(() => {});
const streaming = {destroy: () => {
  if (player.dataset.offline === "true") void window.KinosailOfflineMedia?.saveProgress(player, player.ended);
  detachOfflineProgress?.();
  destroyed = true;
  adaptiveGeneration += 1;
  adaptiveStarting = false;
  pendingResume?.cancel();
  hls?.destroy();
  hls = undefined;
}};
