const playerStatus = document.querySelector("[data-player-status]");
const bufferedProgress = document.querySelector("[data-buffered]");
const playerMessage = document.querySelector("[data-player-message]");
const theaterButton = document.querySelector("[data-theater]");
const settingsButton = document.querySelector("[data-player-settings]");
const settingsPanel = document.querySelector(".player-settings");
const controls = document.querySelector("[data-player-controls]");
const formatTime = (seconds) => {
  if (!Number.isFinite(seconds) || seconds < 0) return "0:00";
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor(seconds % 3600 / 60);
  const remaining = Math.floor(seconds % 60).toString().padStart(2, "0");
  return hours ? `${hours}:${minutes.toString().padStart(2, "0")}:${remaining}` : `${minutes}:${remaining}`;
};
if (controls && player.tagName === "VIDEO") {
  const stage = player.closest(".media-stage");
  stage.tabIndex = 0;
  stage.setAttribute("role", "region");
  stage.setAttribute("aria-label", "Video player");
  stage.setAttribute("aria-keyshortcuts", "Space K ArrowLeft ArrowRight");
  const feedback = document.createElement("p");
  feedback.className = "player-control-feedback";
  feedback.setAttribute("role", "status");
  feedback.hidden = true;
  stage.append(feedback);
  let feedbackTimer;
  const reportControlFailure = (message) => {
    clearTimeout(feedbackTimer);
    feedback.textContent = message;
    feedback.hidden = false;
    feedbackTimer = setTimeout(() => { feedback.hidden = true; }, 8000);
  };
  const seek = controls.querySelector("[data-player-seek]");
  const seekPreview = controls.querySelector("[data-seek-preview]");
  const previewFrame = seekPreview?.querySelector("[data-seek-frame]");
  const previewImage = previewFrame ? new Image() : null;
  if (previewImage) { previewImage.alt = ""; previewImage.hidden = true; previewFrame.append(previewImage); }
  const previewTime = seekPreview?.querySelector("[data-preview-time]");
  let previewTimer;
  let previewSecond = -1;
  const hideSeekPreview = () => {
    clearTimeout(previewTimer);
    previewSecond = -1;
    if (seekPreview) seekPreview.hidden = true;
  };
  const showSeekPreview = (position) => {
    const duration = Number(seek.max);
    if (!seekPreview || !Number.isFinite(position) || !(duration > 0)) return;
    const target = Math.max(0, Math.min(position, duration));
    seekPreview.hidden = false;
    previewTime.textContent = formatTime(target);
    seekPreview.style.setProperty("--preview-progress", `${target / duration * 100}%`);
    if (target > 43200) { previewImage.hidden = true; previewSecond = -1; clearTimeout(previewTimer); return; }
    const second = Math.floor(target / 10) * 10;
    if (second === previewSecond || !seek.dataset.trickplay) return;
    previewSecond = second;
    previewImage.hidden = true;
    previewImage.removeAttribute("src");
    clearTimeout(previewTimer);
    previewTimer = setTimeout(() => { previewImage.src = seek.dataset.trickplay.replace("{second}", String(second)); }, 120);
  };
  previewImage?.addEventListener("load", () => { previewImage.hidden = false; });
  previewImage?.addEventListener("error", () => { previewImage.hidden = true; });
  const volume = controls.querySelector("[data-player-volume]");
  const time = controls.querySelector("[data-player-time]");
  const mute = controls.querySelector("[data-player-mute]");
  const captions = controls.querySelector("[data-player-captions]");
  const pictureInPicture = controls.querySelector("[data-player-pip]");
  const standardPictureInPicture = document.pictureInPictureEnabled === true && typeof player.requestPictureInPicture === "function";
  const webkitPictureInPicture = typeof player.webkitSetPresentationMode === "function" && typeof player.webkitSupportsPresentationMode === "function" && player.webkitSupportsPresentationMode("picture-in-picture");
  const pictureInPictureSupported = standardPictureInPicture || webkitPictureInPicture;
  const syncPictureInPicture = () => {
    if (!pictureInPicture) return;
    const active = isPictureInPicture();
    pictureInPicture.hidden = !pictureInPictureSupported;
    pictureInPicture.setAttribute("aria-label", active ? "Exit Picture-in-Picture" : "Picture-in-Picture");
    pictureInPicture.setAttribute("aria-pressed", String(active));
  };
  const togglePictureInPicture = async () => {
    if (isPictureInPicture()) {
      if (document.pictureInPictureElement === player) await document.exitPictureInPicture?.();
      else player.webkitSetPresentationMode?.("inline");
    } else if (standardPictureInPicture) await player.requestPictureInPicture();
    else player.webkitSetPresentationMode?.("picture-in-picture");
  };
  let scrubPosition;
  const syncControls = () => {
    const duration = Number.isFinite(player.duration) ? player.duration : Number(player.dataset.duration) || 0;
    seek.max = duration || 100;
    seek.value = Math.min(scrubPosition ?? player.currentTime ?? 0, duration || 100);
    seek.style.setProperty("--player-progress", `${duration ? seek.value / duration * 100 : 0}%`);
    time.textContent = `${formatTime(scrubPosition ?? player.currentTime)} / ${formatTime(duration)}`;
    seek.setAttribute("aria-valuetext", `${formatTime(Number(seek.value))} of ${formatTime(duration)}`);
    controls.querySelectorAll("[data-player-toggle]").forEach((button) => {
      button.setAttribute("aria-label", player.paused ? "Play" : "Pause");
      button.classList.toggle("is-paused", !player.paused);
    });
    mute.setAttribute("aria-label", player.muted || !player.volume ? "Unmute" : "Mute");
    mute.setAttribute("aria-pressed", String(player.muted || !player.volume));
    volume.value = player.muted ? 0 : player.volume;
  };
  controls.hidden = false;
  player.controls = false;
  controls.querySelectorAll("[data-player-toggle]").forEach((button) => button.addEventListener("click", () => player.paused ? requestPlay("control").catch(() => {}) : requestPause()));
  controls.querySelectorAll("[data-player-back]").forEach((button) => button.addEventListener("click", () => { player.currentTime = Math.max(0, player.currentTime - 10); }));
  controls.querySelectorAll("[data-player-forward]").forEach((button) => button.addEventListener("click", () => { player.currentTime = Math.min(player.duration || Infinity, player.currentTime + 10); }));
  seek.addEventListener("input", () => { scrubPosition = Number(seek.value); syncControls(); showSeekPreview(scrubPosition); });
  seek.addEventListener("pointermove", (event) => {
    const bounds = seek.getBoundingClientRect();
    if (bounds.width > 0) showSeekPreview(Number(seek.max) * (event.clientX - bounds.left) / bounds.width);
  });
  seek.addEventListener("pointerleave", () => { if (scrubPosition === undefined) hideSeekPreview(); });
  seek.addEventListener("focus", () => showSeekPreview(Number(seek.value)));
  seek.addEventListener("change", () => {
    const position = Number(seek.value);
    scrubPosition = undefined;
    if (Number.isFinite(position) && position >= 0 && position <= Number(seek.max)) player.currentTime = position;
    syncControls();
    hideSeekPreview();
  });
  for (const event of ["pointercancel", "blur"]) seek.addEventListener(event, () => { scrubPosition = undefined; syncControls(); hideSeekPreview(); });
  volume.addEventListener("input", () => { player.muted = false; player.volume = Number(volume.value); syncControls(); });
  mute.addEventListener("click", () => { player.muted = !player.muted; syncControls(); });
  const subtitleSelect = document.querySelector("[data-subtitles]");
  let lastSubtitle = subtitleSelect?.value !== "off" ? subtitleSelect?.value : "0";
  const syncSubtitles = () => {
    const available = Boolean(subtitleSelect && subtitleSelect.options.length > 1);
    captions.disabled = !available;
    captions.title = available ? "Toggle subtitles" : "No subtitles available";
    captions.setAttribute("aria-label", available ? "Subtitles" : "No subtitles available");
    const index = [...player.textTracks].findIndex((track) => track.mode === "showing");
    const selected = index < 0 ? "off" : String(index);
    if (subtitleSelect && [...subtitleSelect.options].some((option) => option.value === selected)) subtitleSelect.value = selected;
    if (index >= 0) lastSubtitle = selected;
    captions.setAttribute("aria-pressed", String(index >= 0));
  };
  player.textTracks?.addEventListener?.("change", syncSubtitles);
  if (subtitleSelect) new MutationObserver(syncSubtitles).observe(subtitleSelect, { childList: true });
  syncSubtitles();
  captions.addEventListener("click", () => {
    const select = document.querySelector("[data-subtitles]");
    if (!select || select.options.length < 2) return;
    select.value = [...player.textTracks].some((track) => track.mode === "showing") ? "off" : lastSubtitle || "0";
    select.dispatchEvent(new Event("change", {bubbles: true}));
    syncSubtitles();
  });
  const fullscreen = document.querySelector("[data-player-fullscreen]");
  const fullscreenSupported = Boolean((document.fullscreenEnabled && stage.requestFullscreen) || player.webkitEnterFullscreen);
  if (fullscreen) {
    fullscreen.disabled = !fullscreenSupported;
    fullscreen.title = fullscreenSupported ? "Fullscreen" : "Fullscreen is unavailable in this browser";
  }
  fullscreen?.addEventListener("click", async () => {
    try {
      if (document.fullscreenElement) await document.exitFullscreen();
      else if (document.fullscreenEnabled && stage.requestFullscreen) await stage.requestFullscreen();
      else if (player.webkitEnterFullscreen) player.webkitEnterFullscreen();
    } catch (_) { reportControlFailure("Fullscreen could not open. Try again, or use Theater mode."); }
  });
  pictureInPicture?.addEventListener("click", () => togglePictureInPicture().catch(() => reportControlFailure("Picture-in-Picture could not open. Start the video, then try again.")));
  document.addEventListener("keydown", (event) => {
    if (event.defaultPrevented || event.metaKey || event.ctrlKey || event.altKey || event.shiftKey ||
        document.querySelector("dialog[open]") ||
        (event.target !== document.body && !stage.contains(event.target)) ||
        event.target.closest?.("input,select,textarea,button,a,summary,[contenteditable]:not([contenteditable=false]),[role=button]")) return;
    const key = event.key.toLowerCase();
    if (key === " " || key === "k") {
      event.preventDefault();
      if (player.paused) requestPlay("keyboard").catch(() => reportControlFailure("Playback could not start. Try Play again."));
      else requestPause();
    } else if (["arrowleft", "arrowright"].includes(key)) {
      const duration = Number.isFinite(player.duration) ? player.duration : Number(player.dataset.duration);
      if (!(duration > 0)) return;
      event.preventDefault();
      player.currentTime = Math.max(0, Math.min(duration, player.currentTime + (key === "arrowright" ? 10 : -10)));
    }
  });
  if (settingsPanel) {
    const help = document.createElement("p");
    help.textContent = "Keyboard: Space or K to play/pause, left/right arrows to seek 10 seconds, T for Theater, Esc to close.";
    settingsPanel.append(help);
  }
  for (const event of ["enterpictureinpicture", "leavepictureinpicture", "webkitpresentationmodechanged"]) player.addEventListener(event, syncPictureInPicture);
  syncPictureInPicture();
  player.addEventListener("click", () => player.paused ? requestPlay("media-element").catch(() => {}) : requestPause());
  for (const event of ["loadedmetadata", "durationchange", "timeupdate", "play", "pause", "volumechange"]) player.addEventListener(event, syncControls);
  document.addEventListener("fullscreenchange", () => {
    const button = document.querySelector("[data-player-fullscreen]");
    button?.setAttribute("aria-label", document.fullscreenElement ? "Exit fullscreen" : "Enter fullscreen");
  });
  syncControls();
}
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
  player.addEventListener("loadstart", () => { if (!seeking) showPlayerState("loading", "Loading video…"); });
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
