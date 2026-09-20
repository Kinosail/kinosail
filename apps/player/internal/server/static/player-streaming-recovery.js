const clearRecovery = () => {
  if (playbackRecovery) playbackRecovery.hidden = true;
  fallbackButtons.forEach((button) => { button.hidden = true; });
  document.querySelector("[data-player-status]")?.classList.remove("is-recovery");
};
let recoveryAction;
const showFailure = (message, action = "", recover) => {
  playbackTrace("fallback-offered", action ? "action-available" : "no-action");
  const state = document.querySelector("[data-player-status]");
  const playerMessage = document.querySelector("[data-player-message]");
  if (state) { state.hidden = false; state.classList.add("is-recovery"); }
  if (playerMessage) playerMessage.textContent = message;
  if (playbackModeStatus) {
    playbackModeStatus.textContent = adaptiveActive ? "Playback interrupted" : "Direct Play stopped";
    playbackModeStatus.setAttribute("aria-label", `${playbackModeStatus.textContent}. Open playback settings.`);
  }
  if (playbackReason) playbackReason.textContent = adaptiveActive ? "Playback was interrupted. Your position is saved." : "The original file stopped playing. No conversion is running.";
  if (!action) return;
  recoveryAction = recover;
  if (playbackRecovery) playbackRecovery.hidden = false;
  if (recoveryMessage) recoveryMessage.textContent = message;
  fallbackButtons.forEach((button) => { button.hidden = false; button.textContent = action; });
};
const useCompatibleFallback = () => {
  playbackTrace("fallback-selected", player.dataset.compatibilityMode || "compatible");
  clearRecovery();
  if (stream) startAdaptive(true);
  else location.assign(player.dataset.compatibleUrl);
};
fallbackButtons.forEach((button) => button.addEventListener("click", () => recoveryAction?.()));
let directSeeking = false;
player.addEventListener("seeking", () => { directSeeking = true; });
player.addEventListener("seeking", () => {
  if (!adaptiveActive || !fullDuration || adaptiveSeekSwitch) return;
  if (!hls) {
    const target = player.currentTime;
    if (streamOffset(target) !== playbackTimelineOffset) {
      adaptiveSeekSwitch = true;
      adaptiveActive = false;
      startAdaptive(true);
    }
    return;
  }
  const details = hls.latestLevelDetails;
  const target = player.currentTime;
  const first = details?.fragments?.[0]?.start ?? adaptiveOffset;
  const edge = details?.edge ?? adaptiveOffset + 30;
  if (target < first - 1 || target > edge + 1) {
    adaptiveSeekSwitch = true;
    useAdaptive(playerStorage.get(qualityPreference) || "auto", true, target);
  }
});
const showReadyPlaybackMode = () => {
  if (directSeeking) directSeeking = false;
  clearRecovery();
  showPlaybackMode(adaptiveActive, false);
};
player.addEventListener("canplay", showReadyPlaybackMode);
player.addEventListener("playing", showReadyPlaybackMode);
let networkRetryTimer, reachabilityController, networkRetryAction;
let networkRetryCount = 0, networkStableSince = 0;
let networkPosition = player.currentTime || Number(player.dataset.start) || 0;
let networkWantsPlay = !player.paused || player.autoplay || player.hasAttribute("data-autoplay");
const cancelNetworkRecovery = (clearAction = true) => {
  if (clearAction) networkRetryAction = undefined;
  clearTimeout(networkRetryTimer);
  networkRetryTimer = undefined;
  reachabilityController?.abort();
  reachabilityController = undefined;
};
player.addEventListener("kinosail:playback-intent", ({detail}) => {
  networkWantsPlay = detail.playing === true;
  if (pendingResume) pendingResume.playing = networkWantsPlay;
  if (!networkWantsPlay) {
    cancelNetworkRecovery(false);
    if (networkRetryAction) showFailure("Connection interrupted. Your position is retained.", "Retry playback", networkRetryAction);
  } else if (networkRetryAction) networkRetryAction();
});
player.addEventListener("play", () => { networkWantsPlay = true; if (networkRetryAction) networkRetryAction(); });
player.addEventListener("pause", () => {
  networkStableSince = 0;
  if (player.paused && !player.error && !pendingResume && player.dataset.offlineLoading !== "true") { networkWantsPlay = false; cancelNetworkRecovery(false); }
});
player.addEventListener("seeking", () => { networkPosition = player.currentTime; if (pendingResume) pendingResume.seconds = networkPosition; });
player.addEventListener("timeupdate", () => {
  if (!player.error && player.readyState && Number.isFinite(player.currentTime)) networkPosition = player.currentTime;
  if (player.paused || player.error || player.readyState < HTMLMediaElement.HAVE_FUTURE_DATA) { networkStableSince = 0; return; }
  if (!networkStableSince) networkStableSince = performance.now();
  if (performance.now() - networkStableSince >= 30000) networkRetryCount = 0;
});
addEventListener("pagehide", cancelNetworkRecovery);
const scheduleNetworkRetry = (retry, manual) => {
  if (networkRetryTimer !== undefined || destroyed || player.dataset.offline === "true") return;
  networkStableSince = 0;
  const generation = adaptiveGeneration;
  const source = player.currentSrc || player.src;
  const action = () => {
    if (destroyed || generation !== adaptiveGeneration || player.dataset.offline === "true" || (player.currentSrc || player.src) !== source) { cancelNetworkRecovery(); return; }
    cancelNetworkRecovery(); networkWantsPlay = true;
    if (manual) manual(); else { networkRetryCount = 0; retry(); }
  };
  networkRetryAction = action;
  if (networkRetryCount >= 3 || !networkWantsPlay) return showFailure("Connection interrupted. Your position is retained.", "Retry playback", action);
  const delay = 1000 * 2 ** networkRetryCount++ + Math.random() * 500;
  const status = document.querySelector("[data-player-message]");
  if (status) status.textContent = "Connection interrupted. Reconnecting…";
  networkRetryTimer = setTimeout(() => {
    networkRetryTimer = undefined;
    if (destroyed || generation !== adaptiveGeneration || player.dataset.offline === "true" || (player.currentSrc || player.src) !== source || !networkWantsPlay) return;
    networkRetryAction = undefined;
    retry();
  }, delay);
};
const directIsReachable = async () => {
  reachabilityController?.abort();
  const controller = new AbortController();
  reachabilityController = controller;
  const timeout = setTimeout(() => controller.abort(), 5000);
  let response;
  try {
    response = await fetch(direct, {headers: {Range: "bytes=0-0"}, cache: "no-store", redirect: "error", signal: controller.signal});
    return response.ok;
  } catch (_) { return false; }
  finally {
    clearTimeout(timeout);
    controller.abort();
    await response?.body?.cancel().catch(() => {});
    if (reachabilityController === controller) reachabilityController = undefined;
  }
};
const recoverDirectFailure = async (code = player.error?.code || 0, verifySource = true) => {
  if (destroyed || player.dataset.offline === "true") return;
  if (adaptiveActive) {
    if (!hls && code === MediaError.MEDIA_ERR_NETWORK) scheduleNetworkRetry(() => { resumeAfterSourceChange(networkWantsPlay, true, networkPosition); player.load(); });
    return;
  }
  const generation = adaptiveGeneration;
  const failedSource = player.currentSrc || player.src;
  const retryDirect = () => useOriginal(true);
  if (code === MediaError.MEDIA_ERR_ABORTED) return;
  const retryNetwork = () => scheduleNetworkRetry(() => useOriginal(networkWantsPlay, pendingResume?.seconds ?? networkPosition), () => { networkRetryCount = 0; useOriginal(true, pendingResume?.seconds ?? networkPosition); });
  if (code === MediaError.MEDIA_ERR_NETWORK) return retryNetwork();
  if (playbackPolicy === "direct-only") return showFailure("Direct Play only is selected. Kinosail Player did not fall back.", "Retry Direct Play", retryDirect);
  const label = player.dataset.compatibilityLabel || "Compatibility";
  const action = player.dataset.compatibilityMode === "transcode" ? "Start video transcode" : `Use ${label}`;
  if (code !== MediaError.MEDIA_ERR_DECODE && code !== MediaError.MEDIA_ERR_SRC_NOT_SUPPORTED) return showFailure("Playback stopped without a confirmed format problem. Try playing the original file again.", action, useCompatibleFallback);
  if (verifySource) {
    const reachable = await directIsReachable();
    if (destroyed || generation !== adaptiveGeneration || player.dataset.offline === "true" || (player.currentSrc || player.src) !== failedSource) return;
    if (!reachable) return retryNetwork();
  }
  if (player.dataset.compatibilityMode === "transcode") return showFailure(`${player.dataset.compatibilityDescription || "This device cannot play the original video."} Start video transcoding?`, action, useCompatibleFallback);
  useCompatibleFallback();
};
player.addEventListener("error", () => recoverDirectFailure());
quality?.addEventListener("change", () => {
  cancelNetworkRecovery(); networkRetryCount = 0;
  playbackTrace("policy-change", "quality", quality.value === "original" ? "original" : quality.value === "auto" ? "auto" : quality.selectedOptions[0].textContent);
  if (quality.value === "original") {
    playbackPolicy = "direct-only";
    setPlaybackMode(playbackPolicy);
    return useOriginal();
  }
  const preference = quality.value === "auto" ? "auto" : quality.selectedOptions[0].textContent;
  playerStorage.set(qualityPreference, preference);
  qualityState.textContent = preference === "auto" ? "Auto" : preference;
  if (!hls) return useAdaptive(preference, true);
  if (preference === "auto") hls.loadLevel = -1;
  else hls.nextLevel = Number(quality.value.slice(6));
});
playbackMode?.addEventListener("change", () => {
  cancelNetworkRecovery(); networkRetryCount = 0;
  playbackPolicy = playbackMode.querySelector("input:checked")?.value || playbackPolicy;
  playbackTrace("policy-change", playbackPolicy);
  playerStorage.set(policyPreference, playbackPolicy);
  clearRecovery();
  document.querySelector(".more-player-actions")?.removeAttribute("open");
  if (playbackPolicy !== "compatible") {
    if (direct) useOriginal();
    else location.assign(player.dataset.directUrl);
  } else {
    playerStorage.set(qualityPreference, "auto");
    if (stream) startAdaptive(true);
    else location.assign(player.dataset.compatibleUrl);
  }
});
playbackModeStatus?.addEventListener("click", () => document.querySelector("[data-player-settings]")?.click());
const directType = player.dataset.directType;
const directSupport = directType ? player.canPlayType(directType) : "unknown";
playbackTrace("capability", directSupport || "none", `${navigator.vendor || "unknown"}:${directType || "unknown"}`);
const directTypeUnsupported = directType && navigator.vendor.includes("Apple") && /^video\/(x-)?matroska(?:;|$)/i.test(directType);
if (player.dataset.hls) {
  if (playbackPolicy !== "compatible" && direct) {
    if (player.getAttribute("src") !== direct) useOriginal();
    else showPlaybackMode(false, true);
  }
  else startAdaptive(false);
} else {
  showPlaybackMode(false, true);
  if (playbackPolicy === "compatible") {
    if (stream) startAdaptive(false);
    else location.assign(player.dataset.compatibleUrl);
  } else if (playbackPolicy === "direct-first" && directTypeUnsupported) {
    if (player.dataset.compatibilityMode === "transcode") recoverDirectFailure(MediaError.MEDIA_ERR_SRC_NOT_SUPPORTED, false);
    else startAdaptive(false);
  } else if (playbackPolicy === "direct-first" && player.dataset.compatibilityMode === "audio-transcode") {
    // Browsers can render a video stream while silently dropping an unsupported audio codec.
    // The server has already confirmed that the audio needs conversion, so do not wait for a
    // media error that may never arrive before starting the audio-only compatible rendition.
    startAdaptive(false);
  } else if (direct && !player.getAttribute("src")) player.src = player.dataset.direct;
}
if (player.readyState >= HTMLMediaElement.HAVE_CURRENT_DATA) showPlaybackMode(adaptiveActive, false);
