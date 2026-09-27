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
    mediaStage.classList.toggle("is-busy", state !== "error" && state !== "start");
    playerStatus.dataset.state = state;
    bufferedProgress.hidden = state !== "buffering";
    if (playerStart) playerStart.hidden = state !== "start";
    playerStatus.hidden = false;
    playerStatus.setAttribute("aria-busy", String(state !== "error" && state !== "start"));
    const loadingIndicator = playerStatus.querySelector(".buffer-skeleton");
    if (loadingIndicator) loadingIndicator.hidden = state === "error" || state === "start";
    playerMessage.textContent = message;
    bufferedPercent();
  };
  const hidePlayerState = () => {
    if (playerStatus.classList.contains("is-recovery")) return;
    mediaStage.classList.remove("is-busy");
    if (playerStart) playerStart.hidden = true;
    playerStatus.hidden = true;
    playerStatus.removeAttribute("aria-busy");
    playerStatus.dataset.state = "ready";
  };
  let seeking = false;
  let bufferingTimer;
  let playbackTime = player.currentTime;
  const clearBufferingTimer = () => { clearTimeout(bufferingTimer); bufferingTimer = undefined; };
  let hasPlayed = false;
  const showStartPrompt = () => {
    if (playerStart && player.paused && !player.error && !playerStatus.classList.contains("is-recovery")) showPlayerState("start", "Ready to play");
  };
  const appleTouch = /iPhone|iPad|iPod/.test(navigator.userAgent) || navigator.platform === "MacIntel" && navigator.maxTouchPoints > 1;
  const scheduleStartPrompt = () => {
    if (!appleTouch) return;
    setTimeout(() => {
      if (!hasPlayed && (player.currentSrc || player.getAttribute("src"))) showStartPrompt();
    }, 1500);
  };
  player.addEventListener("kinosail:play-needs-gesture", showStartPrompt);
  playerStart?.addEventListener("click", () => {
    showPlayerState("loading", "Starting video…");
    requestPlay("tap-to-play").catch((error) => {
      if (error?.name !== "NotAllowedError") showPlayerState("error", "Playback unavailable");
    });
  });
  player.addEventListener("loadstart", () => { if (!seeking) showPlayerState("loading", "Loading video…"); scheduleStartPrompt(); });
  scheduleStartPrompt();
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
  });
  player.addEventListener("loadedmetadata", bufferedPercent);
  player.addEventListener("timeupdate", () => {
    const advancing = player.currentTime > playbackTime;
    playbackTime = player.currentTime;
    if (advancing && !seeking && !player.paused) { clearBufferingTimer(); hidePlayerState(); }
  });
  for (const event of ["canplay", "playing"]) player.addEventListener(event, () => {
    clearBufferingTimer();
    seeking = false;
    if (event === "canplay" && player.paused && playerStatus.dataset.state === "start") return;
    if (event === "playing") hasPlayed = true;
    hidePlayerState();
  });
  player.addEventListener("seeked", () => {
    clearBufferingTimer();
    seeking = false;
    if (player.readyState >= 3) hidePlayerState();
    else showPlayerState("buffering", bufferedPercent() ? `Buffering · ${bufferedPercent()}% buffered` : "Buffering…");
  });
  player.addEventListener("error", () => {
    clearBufferingTimer();
    if (!playerStatus.classList.contains("is-recovery")) showPlayerState("error", "Playback unavailable");
  });
  bufferedPercent();
  if (!player.paused && player.readyState >= HTMLMediaElement.HAVE_CURRENT_DATA) hidePlayerState();
}
