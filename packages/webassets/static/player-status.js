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
    if (player.paused && !player.error && (appleNativePlayback && !applePlaybackRequested || readyForPlay() || needsGesture)) hidePlayerState();
  };
  let startupPaused = false;
  let startupFinished = false;
  const preparePlayback = () => {
    if (appleNativePlayback) return revealPlayControl();
    if (!appleTouch || player.tagName !== "VIDEO" || player.dataset.room || startupPaused || startupFinished || hasPlayed || playbackPreparation || !player.paused || player.error ||
      !(player.currentSrc || player.getAttribute("src"))) return;
    // Muted inline playback lets Safari fetch media before the viewer's Play tap.
    player.autoplay = false;
    delete player.dataset.autoplay;
    if (readyForPlay()) return revealPlayControl();
    const preparation = {muted: player.muted, position: player.currentTime || Number(player.dataset.start) || 0, stop: (restorePosition = true) => {
      if (playbackPreparation !== preparation) return;
      if (!player.paused) preparationPausePending++;
      player.pause();
      let position = preparation.position;
      // Copied HLS video can begin after the requested timestamp.
      for (let index = 0; index < player.buffered.length; index++) {
        const start = player.buffered.start(index) + playbackTimelineOffset;
        const end = player.buffered.end(index) + playbackTimelineOffset;
        if (start <= player.currentTime && end >= player.currentTime) { position = Math.max(position, start); break; }
      }
      if (restorePosition && player.readyState && Math.abs(player.currentTime - position) >= 0.1) {
        preparationSeek = {source: player.currentSrc || player.src, seconds: position};
        setPlayerTime(position);
      }
      player.muted = preparation.muted;
      playbackPreparation = undefined;
    }};
    playbackPreparation = preparation;
    player.muted = true;
    const failed = (error) => {
      if (playbackPreparation !== preparation) return;
      preparation.stop();
      if (error?.name === "NotAllowedError") { needsGesture = true; revealPlayControl(); }
    };
    try { Promise.resolve(player.play()).catch(failed); } catch (error) { failed(error); }
  };
  const finishPreparation = () => {
    if (playbackPreparation && readyForPlay()) {
      needsGesture = false;
      playbackPreparation.stop();
    }
    if (!playbackPreparation) revealPlayControl();
  };
  player.addEventListener("kinosail:playback-intent", ({detail}) => {
    startupFinished = true;
    startupPaused = detail.playing === false;
    if (startupPaused) playbackPreparation?.stop();
  });
  player.addEventListener("kinosail:play-needs-gesture", () => { needsGesture = true; revealPlayControl(); });
  player.addEventListener("loadstart", () => {
    playbackPreparation?.stop(false);
    hasPlayed = false; needsGesture = false;
    if (!seeking) showPlayerState("loading", "Loading video…");
    preparePlayback();
  });
  preparePlayback();
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
    if (appleNativePlayback) revealPlayControl();
  });
  player.addEventListener("seeking", () => {
    clearBufferingTimer();
    seeking = true;
    playbackTime = player.currentTime;
    if (playbackPreparation) playbackPreparation.position = player.currentTime;
    if (appleNativePlayback && !applePlaybackRequested && player.paused) return revealPlayControl();
    showPlayerState("seeking", "Seeking…");
  });
  player.addEventListener("progress", () => {
    const percent = bufferedPercent();
    if (playerStatus.dataset.state === "buffering") playerMessage.textContent = percent ? `Buffering · ${percent}% buffered` : "Buffering…";
    if (!hasPlayed) finishPreparation();
  });
  player.addEventListener("suspend", () => { if (!hasPlayed) { finishPreparation(); preparePlayback(); } });
  player.addEventListener("loadedmetadata", bufferedPercent);
  player.addEventListener("timeupdate", () => {
    const advancing = player.currentTime > playbackTime;
    playbackTime = player.currentTime;
    if (playbackPreparation) return finishPreparation();
    if (advancing && !player.seeking && !player.paused && !player.error) { seeking = false; clearBufferingTimer(); hidePlayerState(); }
  });
  for (const event of ["canplay", "playing"]) player.addEventListener(event, () => {
    if (event === "playing" && player.paused) return;
    clearBufferingTimer();
    seeking = false;
    if (playbackPreparation) return finishPreparation();
    if (event === "playing") hasPlayed = true;
    if (hasPlayed || readyForPlay()) hidePlayerState();
  });
  player.addEventListener("seeked", () => {
    clearBufferingTimer();
    seeking = false;
    if (appleNativePlayback && !applePlaybackRequested && player.paused) return revealPlayControl();
    if (player.paused && needsGesture) return revealPlayControl();
    if (hasPlayed ? player.readyState >= HTMLMediaElement.HAVE_FUTURE_DATA : readyForPlay()) hidePlayerState();
    else showPlayerState("buffering", bufferedPercent() ? `Buffering · ${bufferedPercent()}% buffered` : "Buffering…");
  });
  player.addEventListener("error", () => {
    playbackPreparation?.stop();
    clearBufferingTimer();
    if (!playerStatus.classList.contains("is-recovery")) showPlayerState("error", "Playback unavailable");
  });
  bufferedPercent();
  // Parser-started media can emit loadstart before this deferred script runs.
  if (!player.error && player.networkState === HTMLMediaElement.NETWORK_LOADING && player.readyState === HTMLMediaElement.HAVE_NOTHING) showPlayerState("loading", "Loading video…");
  if (player.hasAttribute("data-native-controls")) revealPlayControl();
  if (appleNativePlayback) revealPlayControl();
  if (!playbackPreparation && !player.paused && player.readyState >= HTMLMediaElement.HAVE_CURRENT_DATA) hidePlayerState();
}
