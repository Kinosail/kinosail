if (playerStatus) {
  const mediaStage = playerStatus.closest(".media-stage");
  const bufferedPercent = () => {
    const expected = Number(player.dataset.duration);
    const duration = Number.isFinite(player.duration) && player.duration > 0 ? player.duration : expected;
    let end = 0;
    for (let index = 0; index < player.buffered.length; index++) end = Math.max(end, player.buffered.end(index) + (typeof playbackTimelineOffset === "number" ? playbackTimelineOffset : 0));
    const percent = Number.isFinite(duration) && duration > 0 ? Math.min(100, Math.round(end / duration * 100)) : 0;
    bufferedProgress.value = percent;
    bufferedProgress.textContent = `${percent}%`;
    bufferedProgress.setAttribute("aria-valuetext", `${percent}% buffered`);
    return percent;
  };
  const showPlayerState = (state, message) => {
    if (playerStatus.classList.contains("is-recovery")) return;
    mediaStage.classList.toggle("is-busy", state !== "error");
    playerStatus.dataset.state = state;
    bufferedProgress.hidden = state !== "buffering";
    playerStatus.hidden = false;
    playerStatus.setAttribute("aria-busy", String(state !== "error"));
    const loadingIndicator = playerStatus.querySelector(".buffer-skeleton");
    if (loadingIndicator) loadingIndicator.hidden = state === "error";
    playerMessage.textContent = message;
    bufferedPercent();
  };
  const hidePlayerState = () => {
    if (playerStatus.classList.contains("is-recovery")) return;
    mediaStage.classList.remove("is-busy");
    playerStatus.hidden = true;
    playerStatus.removeAttribute("aria-busy");
    playerStatus.dataset.state = "ready";
  };
  let seeking = false;
  let bufferingTimer;
  let playbackTime = player.currentTime;
  const clearBufferingTimer = () => { clearTimeout(bufferingTimer); bufferingTimer = undefined; };
  let hasPlayed = false;
  let needsGesture = false;
  const readyForPlay = () => player.readyState >= HTMLMediaElement.HAVE_FUTURE_DATA &&
    bufferedAhead() >= Math.min(2, Number.isFinite(player.duration) ? Math.max(0, player.duration - player.currentTime) : 2);
  const revealPlayControl = () => {
    // iOS can suspend preloading until a real Play gesture. Keep that escape only
    // after the browser rejects play, never while it is still fetching video.
    const gestureRequired = needsGesture && player.networkState === HTMLMediaElement.NETWORK_IDLE;
    if (player.paused && !player.error && (readyForPlay() || gestureRequired)) hidePlayerState();
  };
  const appleTouch = /iPhone|iPad|iPod/.test(navigator.userAgent) || navigator.platform === "MacIntel" && navigator.maxTouchPoints > 1;
  let gestureTimer;
  let startupPaused = false;
  player.addEventListener("kinosail:playback-intent", ({detail}) => { startupPaused = detail.playing === false; if (startupPaused) clearTimeout(gestureTimer); });
  const schedulePlayControl = () => {
    if (!appleTouch) return;
    clearTimeout(gestureTimer);
    gestureTimer = setTimeout(() => {
      if (!startupPaused && !hasPlayed && player.paused && !player.error && player.networkState === HTMLMediaElement.NETWORK_IDLE &&
        (player.currentSrc || player.getAttribute("src"))) requestPlay("startup-gesture-check").catch(() => {});
    }, 1500);
  };
  player.addEventListener("kinosail:play-needs-gesture", () => { needsGesture = true; revealPlayControl(); });
  player.addEventListener("loadstart", () => { hasPlayed = false; needsGesture = false; if (!seeking) showPlayerState("loading", "Loading video…"); schedulePlayControl(); });
  schedulePlayControl();
  player.addEventListener("play", () => { if (!hasPlayed && !readyForPlay()) showPlayerState("loading", "Loading video…"); });
  player.addEventListener("waiting", () => {
    clearBufferingTimer();
    const waitingAt = player.currentTime;
    bufferingTimer = setTimeout(() => {
      if (seeking || player.paused || player.currentTime > waitingAt) return;
      const percent = bufferedPercent();
      if (percent >= 99) return;
      showPlayerState("buffering", percent ? `Buffering · ${percent}% buffered` : "Buffering…");
    }, 500);
  });
  for (const event of ["pause", "ended"]) player.addEventListener(event, () => {
    clearBufferingTimer();
    if (playerStatus.dataset.state === "buffering") hidePlayerState();
  });
  player.addEventListener("seeking", () => {
    clearBufferingTimer();
    seeking = true;
    showPlayerState("seeking", "Seeking…");
  });
  player.addEventListener("progress", () => {
    const percent = bufferedPercent();
    if (playerStatus.dataset.state === "buffering") playerMessage.textContent = percent ? `Buffering · ${percent}% buffered` : "Buffering…";
    if (!hasPlayed) revealPlayControl();
  });
  player.addEventListener("suspend", () => { if (!hasPlayed) revealPlayControl(); });
  player.addEventListener("loadedmetadata", bufferedPercent);
  player.addEventListener("timeupdate", () => {
    const advancing = player.currentTime > playbackTime;
    playbackTime = player.currentTime;
    if (advancing && !seeking && !player.paused) { clearBufferingTimer(); hidePlayerState(); }
  });
  for (const event of ["canplay", "playing"]) player.addEventListener(event, () => {
    clearBufferingTimer();
    seeking = false;
    if (event === "playing") hasPlayed = true;
    if (hasPlayed || readyForPlay()) hidePlayerState();
  });
  player.addEventListener("seeked", () => {
    clearBufferingTimer();
    seeking = false;
    if (hasPlayed ? player.readyState >= HTMLMediaElement.HAVE_FUTURE_DATA : readyForPlay()) hidePlayerState();
    else showPlayerState("buffering", bufferedPercent() ? `Buffering · ${bufferedPercent()}% buffered` : "Buffering…");
  });
  player.addEventListener("error", () => {
    clearBufferingTimer();
    if (!playerStatus.classList.contains("is-recovery")) showPlayerState("error", "Playback unavailable");
  });
  bufferedPercent();
  if (!player.paused && player.readyState >= HTMLMediaElement.HAVE_CURRENT_DATA) hidePlayerState();
}
