        playerStorage.set(qualityPreference, "auto");
      }
      if (data.type === Hls.ErrorTypes.NETWORK_ERROR && networkRecoveries++ < 2) hls.startLoad();
      else if (data.type === Hls.ErrorTypes.MEDIA_ERROR && mediaRecoveries++ < 1) hls.recoverMediaError();
      else if (direct && !player.dataset.adaptive) useOriginal();
      else qualityState.textContent = "Playback unavailable";
    });
    hls.loadSource(selected.source);
    hls.attachMedia(fullDuration ? {media: player, overrides: {duration: fullDuration}} : player);
  };
  const startAdaptive = async (resume = false) => {
    if (adaptiveActive || adaptiveStarting || !stream) return;
    adaptiveStarting = true;
    showPlaybackMode(true, true);
    let negotiation = Promise.resolve();
    if (player.dataset.adaptive && !streamNegotiated) {
      streamNegotiated = true;
      negotiation = negotiateStream();
    } else if (player.dataset.hls && !streamNegotiated) {
      streamNegotiated = true;
      negotiation = negotiateStream();
    }
    await negotiation;
    const preference = playerStorage.get(qualityPreference) || "auto";
    qualityControl.hidden = false;
    qualityState.textContent = "Auto";
    if (player.dataset.hls && typeof Hls !== "undefined" && Hls.isSupported()) {
      useAdaptive(preference === "original" ? "auto" : preference, resume);
    } else {
      if (typeof Hls !== "undefined" && Hls.isSupported()) useAdaptive(preference === "original" ? "auto" : preference, resume);
      else if (player.canPlayType("application/vnd.apple.mpegurl")) {
        if (resume) resumeAfterSourceChange();
        adaptiveActive = true;
        playbackTraceMethod = "native-hls";
        playbackTrace("source-compatible", "native-hls");
        player.src = stream;
        player.load();
      } else if (await loadHls() && Hls.isSupported()) useAdaptive(preference === "original" ? "auto" : preference, resume);
      else if (player.dataset.fallback) location.replace(player.dataset.fallback);
      else qualityState.textContent = "Playback unavailable";
    }
    adaptiveStarting = false;
  };
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
      playbackModeStatus.textContent = "Direct Play stopped";
      playbackModeStatus.setAttribute("aria-label", "Playback method: Direct Play stopped. Open playback settings.");
    }
    if (playbackReason) playbackReason.textContent = "The original file stopped playing. No conversion is running.";
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
    if (!adaptiveActive || !hls || !fullDuration || adaptiveSeekSwitch) return;
    const details = hls.latestLevelDetails;
    const target = player.currentTime;
    const first = details?.fragments?.[0]?.start ?? adaptiveOffset;
    const edge = details?.edge ?? adaptiveOffset + 30;
    if (target < first - 1 || target > edge + 1) {
      adaptiveSeekSwitch = true;
      useAdaptive(playerStorage.get(qualityPreference) || "auto", true, target);
    }
  });
  player.addEventListener("loadedmetadata", () => { if (!adaptiveActive) showPlaybackMode(false, false); });
  const showReadyPlaybackMode = () => {
    if (directSeeking) directSeeking = false;
    clearRecovery();
    showPlaybackMode(adaptiveActive, false);
  };
  player.addEventListener("canplay", showReadyPlaybackMode);
  player.addEventListener("playing", showReadyPlaybackMode);
  const directIsReachable = async () => {
    try {
      return (await fetch(direct, {headers: {Range: "bytes=0-0"}, cache: "no-store"})).ok;
    } catch (_) {
      return false;
    }
  };
  const recoverDirectFailure = async (code = player.error?.code || 0, verifySource = true) => {
    if (adaptiveActive) return;
    const retryDirect = () => useOriginal(true);
    if (playbackPolicy === "direct-only") return showFailure("Direct Play only is selected. Kinosail Subtitles did not fall back.", "Retry Direct Play", retryDirect);
    if (code === MediaError.MEDIA_ERR_ABORTED) return;
    if (code === MediaError.MEDIA_ERR_NETWORK) return showFailure("The connection interrupted playback. Check your connection and try again.", "Retry Direct Play", retryDirect);
    const label = player.dataset.compatibilityLabel || "Compatibility";
    const action = player.dataset.compatibilityMode === "transcode" ? "Start video transcode" : `Use ${label}`;
    if (code !== MediaError.MEDIA_ERR_DECODE && code !== MediaError.MEDIA_ERR_SRC_NOT_SUPPORTED) return showFailure("Playback stopped without a confirmed format problem. Try playing the original file again.", action, useCompatibleFallback);
    if (verifySource && !await directIsReachable()) return showFailure("The connection interrupted playback. Check your connection and try again.", "Retry Direct Play", retryDirect);
    if (player.dataset.compatibilityMode === "transcode") return showFailure(`${player.dataset.compatibilityDescription || "This device cannot play the original video."} Start video transcoding?`, action, useCompatibleFallback);
    useCompatibleFallback();
  };
  player.addEventListener("error", () => recoverDirectFailure());
  quality?.addEventListener("change", () => {
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
    playbackPolicy = playbackMode.querySelector("input:checked")?.value || playbackPolicy;
    playbackTrace("policy-change", playbackPolicy);
    playerStorage.set(policyPreference, playbackPolicy);
    clearRecovery();
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
    } else if (direct && !player.getAttribute("src")) player.src = player.dataset.direct;
  }
  if (player.readyState >= HTMLMediaElement.HAVE_CURRENT_DATA) showPlaybackMode(adaptiveActive, false);
  const itemID = decodeURIComponent(location.pathname.startsWith("/watch/") ? location.pathname.slice("/watch/".length) : "");
  window.KinosailOfflineMedia?.source(itemID).then((source) => {
    if (!source) return;
    hls?.destroy();
    hls = undefined;
    playbackTraceMethod = "offline";
    playbackTrace("source-compatible", "offline");
    player.dataset.offline = "true";
    player.src = source;
    player.load();
    const state = document.querySelector("[data-player-status]");
    const message = document.querySelector("[data-player-message]");
    if (state && message) { state.hidden = false; message.textContent = "Ready offline on this device"; }
  }).catch(() => {});
  return {destroy: () => hls?.destroy()};
})();
