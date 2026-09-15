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
  seek.addEventListener("input", () => { scrubPosition = Number(seek.value); syncControls(); });
  seek.addEventListener("change", () => {
    const position = Number(seek.value);
    scrubPosition = undefined;
    if (Number.isFinite(position) && position >= 0 && position <= Number(seek.max)) player.currentTime = position;
    syncControls();
  });
  for (const event of ["pointercancel", "blur"]) seek.addEventListener(event, () => { scrubPosition = undefined; syncControls(); });
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
let theaterState;
const setTheater = (enabled) => {
  const stage = player.closest(".media-stage");
  if (!stage) return;
  if (enabled && !theaterState) {
    theaterState = {
      focus: document.activeElement === document.body ? theaterButton : document.activeElement,
      inert: [],
      attributes: ["role", "aria-modal", "tabindex"].map((name) => [name, stage.getAttribute(name)]),
    };
    // Isolate only siblings of the stage and its ancestors; preserve existing inert state.
    for (let branch = stage; branch && branch !== document.body; branch = branch.parentElement) {
      for (const sibling of branch.parentElement.children) {
        if (sibling !== branch && !sibling.matches("script,style,dialog") && !sibling.inert) {
          theaterState.inert.push(sibling);
          sibling.inert = true;
        }
      }
    }
    stage.setAttribute("role", "dialog");
    stage.setAttribute("aria-modal", "true");
    stage.tabIndex = -1;
  }
  document.body.classList.toggle("player-theater", enabled);
  theaterButton?.setAttribute("aria-pressed", String(enabled));
  theaterButton?.setAttribute("aria-label", enabled ? "Exit theater" : "Theater");
  if (enabled) (theaterButton || stage).focus({ preventScroll: true });
  else if (theaterState) {
    for (const sibling of theaterState.inert) sibling.inert = false;
    for (const [name, value] of theaterState.attributes) {
      if (value === null) stage.removeAttribute(name);
      else stage.setAttribute(name, value);
    }
    const focus = theaterState.focus;
    theaterState = undefined;
    if (focus?.isConnected) focus.focus({ preventScroll: true });
  }
};
document.addEventListener("keydown", (event) => {
  if (!theaterState || event.key !== "Tab" || document.querySelector("dialog[open]")) return;
  const stage = player.closest(".media-stage");
  const targets = [...stage.querySelectorAll('button,a[href],input,select,textarea,[tabindex]')].filter((node) =>
    !node.disabled && node.tabIndex >= 0 && !node.closest('[hidden],[inert]') && node.getClientRects().length && getComputedStyle(node).visibility !== "hidden");
  const index = targets.indexOf(document.activeElement);
  if (!targets.length || index < 0 || (event.shiftKey ? index === 0 : index === targets.length - 1)) {
    event.preventDefault();
    (targets[event.shiftKey ? targets.length - 1 : 0] || stage).focus();
  }
});
const setSettings = (open) => {
  if (!settingsPanel || !settingsButton) return;
  settingsPanel.hidden = !open;
  settingsButton.setAttribute("aria-expanded", String(open));
  settingsPanel.closest(".media-stage")?.classList.toggle("has-settings", open);
  if (open) settingsPanel.querySelector("input:not([type=hidden]),select,button")?.focus();
};
settingsButton?.addEventListener("click", () => setSettings(settingsPanel.hidden));
document.querySelector("[data-player-settings-close]")?.addEventListener("click", () => { setSettings(false); settingsButton.focus(); });
document.querySelector("[data-subtitles]")?.addEventListener("change", ({target}) => {
  [...player.textTracks].forEach((track, index) => { track.mode = String(index) === target.value ? "showing" : "disabled"; });
  document.querySelector("[data-player-captions]")?.setAttribute("aria-pressed", String(target.value !== "off"));
});
if (theaterButton) {
  const mediaStage = theaterButton.closest(".media-stage");
  const theaterToolbar = mediaStage.querySelector(".player-stage-toolbar");
  let theaterIdle;
  const revealTheater = () => {
    clearTimeout(theaterIdle);
    theaterToolbar.hidden = false;
    controls?.classList.remove("is-idle");
    if (mediaStage.classList.contains("is-playing") && !mediaStage.classList.contains("has-settings")) theaterIdle = setTimeout(() => {
      if (!mediaStage.contains(document.activeElement)) {
        theaterToolbar.hidden = true;
        controls?.classList.add("is-idle");
      }
    }, 2200);
  };
  const setTheaterPlaying = (playing) => {
    mediaStage.classList.toggle("is-playing", playing);
    clearTimeout(theaterIdle);
    theaterToolbar.hidden = playing && !mediaStage.classList.contains("has-settings");
    controls?.classList.toggle("is-idle", playing && !mediaStage.classList.contains("has-settings"));
    if (playing) revealTheater();
  };
  player.addEventListener("playing", () => setTheaterPlaying(true));
  player.addEventListener("timeupdate", () => { if (!player.paused && player.currentTime > 0 && !mediaStage.classList.contains("is-playing")) setTheaterPlaying(true); });
  for (const event of ["pause", "ended", "error"]) player.addEventListener(event, () => setTheaterPlaying(false));
  mediaStage.addEventListener("pointermove", revealTheater);
  mediaStage.addEventListener("pointerdown", revealTheater);
  mediaStage.addEventListener("focusin", revealTheater);
  theaterButton.addEventListener("focus", () => { clearTimeout(theaterIdle); theaterToolbar.hidden = false; });
  theaterButton.addEventListener("blur", revealTheater);
  theaterButton.addEventListener("click", () => setTheater(!document.body.classList.contains("player-theater")));
  document.addEventListener("keydown", (event) => {
    if (event.defaultPrevented || document.querySelector("dialog[open]")) return;
    const editing = event.target.closest?.("input,select,textarea,[contenteditable]:not([contenteditable=false])");
    if (!editing && !event.metaKey && !event.ctrlKey && !event.altKey && event.key.toLowerCase() === "t") {
      event.preventDefault();
      setTheater(!document.body.classList.contains("player-theater"));
    } else if (event.key === "Escape" && !settingsPanel?.hidden) {
      setSettings(false);
      settingsButton?.focus();
    } else if (event.key === "Escape" && document.body.classList.contains("player-theater")) setTheater(false);
  });
  if (!player.paused) setTheaterPlaying(true);
}
if ("mediaSession" in navigator) {
  navigator.mediaSession.metadata = new MediaMetadata({
    title: player.dataset.title,
    artist: player.dataset.artist,
    album: player.dataset.album,
    artwork: player.dataset.artwork ? [{src: player.dataset.artwork}] : [],
  });
  const actions = {
    play: () => requestPlay("media-session"), pause: () => requestPause(),
    seekbackward: ({seekOffset = 10}) => { player.currentTime = Math.max(0, player.currentTime - seekOffset); },
    seekforward: ({seekOffset = 10}) => { player.currentTime = Math.min(player.duration, player.currentTime + seekOffset); },
    seekto: ({seekTime}) => { player.currentTime = seekTime; },
    stop: () => { requestPause(); player.currentTime = 0; },
  };
  if (player.dataset.next) actions.nexttrack = () => location.assign(player.dataset.next);
  for (const [action, handler] of Object.entries(actions)) {
    try { navigator.mediaSession.setActionHandler(action, handler); } catch (_) {}
  }
  const updateMediaPosition = () => {
    if (Number.isFinite(player.duration) && player.duration > 0) navigator.mediaSession.setPositionState({duration: player.duration, playbackRate: player.playbackRate, position: Math.min(player.currentTime, player.duration)});
  };
  player.addEventListener("loadedmetadata", updateMediaPosition);
  player.addEventListener("timeupdate", updateMediaPosition);
  player.addEventListener("play", () => { navigator.mediaSession.playbackState = "playing"; });
  player.addEventListener("pause", () => { navigator.mediaSession.playbackState = "paused"; });
}
let wakeLock;
const requestWakeLock = async () => {
  try { wakeLock = await navigator.wakeLock?.request("screen"); } catch (_) {}
};
const releaseWakeLock = () => { wakeLock?.release(); wakeLock = undefined; };
addEventListener("pagehide", () => {
  playbackTrace("session-end", "pagehide");
  flushPlaybackTrace();
  if (isPictureInPicture()) {
    releaseWakeLock();
    return;
  }
  streaming.destroy();
  requestPause();
  player.removeAttribute("src");
  player.load();
  releaseWakeLock();
});
player.addEventListener("play", requestWakeLock);
player.addEventListener("pause", releaseWakeLock);
player.addEventListener("ended", releaseWakeLock);
document.addEventListener("visibilitychange", () => {
  playbackTrace(document.visibilityState === "visible" ? "visibility-visible" : "visibility-hidden", "visibility-change");
  if (document.visibilityState === "visible" && !player.paused) requestWakeLock();
});
