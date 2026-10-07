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
  const nativeControls = appleNativePlayback || player.hasAttribute("data-native-controls");
  if (appleNativePlayback) {
    controls.classList.add("player-native-controls");
    const toolbar = stage.querySelector(".player-stage-toolbar") || stage.nextElementSibling?.querySelector(".player-stage-toolbar");
    if (settingsButton && toolbar && !toolbar.contains(settingsButton)) toolbar.append(settingsButton);
    for (const child of controls.children) if (!child.matches(".player-center-control[data-player-toggle]")) child.hidden = true;
    if (theaterButton) theaterButton.hidden = true;
    player.addEventListener("webkitendfullscreen", () => {
      requestPause();
      player.controls = false;
      controls.hidden = Boolean(player.error);
    });
  }
  if (nativeControls) {
    let options = stage.nextElementSibling;
    if (!options?.classList.contains("player-native-options")) {
      options = document.createElement("div");
      options.className = "player-native-options";
      stage.after(options);
      options.append(stage.querySelector(".player-stage-toolbar"), settingsPanel);
    }
  }
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
  for (const event of ["error", "playing"]) player.addEventListener(event, () => { clearTimeout(feedbackTimer); feedback.hidden = true; });
  const reportControlFailure = (message) => {
    if (player.error) return;
    clearTimeout(feedbackTimer);
    feedback.textContent = message;
    feedback.hidden = false;
    feedbackTimer = setTimeout(() => { feedback.hidden = true; }, 8000);
  };
  const reportFullscreenFailure = (error) => {
    const failure = ["NotAllowedError", "InvalidStateError", "NotSupportedError", "TypeError"].includes(error?.name) ? error.name : "Error";
    playbackTrace("error", `fullscreen:${failure}:${appleNativePlayback ? "paused-for-retry" : "playback-retained"}`);
    reportControlFailure(appleNativePlayback ? "Apple playback could not open. Try Play again." : "Fullscreen could not open. Try again using the video's fullscreen control.");
  };
  const restoreAppleLauncher = () => {
    requestPause();
    player.controls = false;
    try { if (player.webkitDisplayingFullscreen) player.webkitExitFullscreen?.(); } catch (error) { reportFullscreenFailure(error); }
  };
  const seek = document.querySelector("[data-player-seek]");
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
  const time = document.querySelector("[data-player-time]");
  const mute = controls.querySelector("[data-player-mute]");
  const captions = controls.querySelector("[data-player-captions]");
  if (settingsPanel) {
    const label = document.createElement("label");
    const title = document.createElement("span"); title.textContent = "Playback speed";
    const rate = document.createElement("select"); rate.setAttribute("aria-label", "Playback speed");
    const rates = [0.5, 0.75, 1, 1.25, 1.5, 1.75, 2, 2.5, 3];
    for (const value of rates) {
      const option = document.createElement("option");
      option.value = String(value);
      option.textContent = value === 1 ? "Normal" : `${value}×`;
      rate.append(option);
    }
    rate.value = rates.includes(player.playbackRate) ? String(player.playbackRate) : "1";
    rate.addEventListener("change", () => {
      const value = Number(rate.value);
      if (rates.includes(value)) player.playbackRate = value;
    });
    player.addEventListener("ratechange", () => {
      if (rates.includes(player.playbackRate)) rate.value = String(player.playbackRate);
    });
    label.append(title, rate);
    const footer = settingsPanel.querySelector(":scope > p");
    if (footer) footer.before(label);
    else settingsPanel.append(label);
  }
  const pictureInPicture = document.querySelector("[data-player-pip]");
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
  let nativeStarted = !player.paused && !playbackPreparation;
  const syncControls = (event) => {
    if (nativeControls) {
      if (event?.type === "playing" && !playbackPreparation) nativeStarted = true;
      controls.hidden = (!appleNativePlayback && nativeStarted) || !player.paused || Boolean(player.error);
      const timeline = settingsPanel?.querySelector("[data-native-timeline]");
      const compatible = ["remux", "audio-transcode", "transcode", "native-hls"].includes(playbackTraceMethod);
      if (timeline) timeline.hidden = !compatible;
      if (!seek || !compatible) return;
    }
    const duration = nativeControls ? Number(seek.max) : Number.isFinite(player.duration) ? player.duration : Number(player.dataset.duration) || 0;
    seek.max = duration || 100;
    seek.value = Math.min(scrubPosition ?? player.currentTime ?? 0, duration || 100);
    seek.style.setProperty("--player-progress", `${duration ? seek.value / duration * 100 : 0}%`);
    time.textContent = `${formatTime(scrubPosition ?? player.currentTime)} / ${formatTime(duration)}`;
    seek.setAttribute("aria-valuetext", `${formatTime(Number(seek.value))} of ${formatTime(duration)}`);
    if (nativeControls) return;
    controls.querySelectorAll("[data-player-toggle]").forEach((button) => {
      button.setAttribute("aria-label", player.paused ? "Play" : "Pause");
      button.classList.toggle("is-paused", !player.paused);
    });
    mute.setAttribute("aria-label", player.muted || !player.volume ? "Unmute" : "Mute");
    mute.setAttribute("aria-pressed", String(player.muted || !player.volume));
    volume.value = player.muted ? 0 : player.volume;
  };
  controls.hidden = false;
  player.controls = nativeControls && !appleNativePlayback;
  controls.querySelectorAll("[data-player-toggle]").forEach((button) => button.addEventListener("click", () => {
    if (appleNativePlayback) {
      if (!applePhone && player.readyState < HTMLMediaElement.HAVE_METADATA) {
        player.load();
        reportControlFailure("Video is preparing. Tap Play again when ready.");
        return;
      }
      try {
        // Before metadata, play without playsinline lets iPhone open Apple when ready.
        // Never retry fullscreen from a readiness callback without a fresh gesture.
        if (!player.webkitDisplayingFullscreen && player.readyState >= HTMLMediaElement.HAVE_METADATA) player.webkitEnterFullscreen();
        requestPlay("apple-play").catch(() => { restoreAppleLauncher(); reportControlFailure("Playback could not start. Try Play again."); });
      } catch (error) { restoreAppleLauncher(); reportFullscreenFailure(error); }
      return;
    }
    if (!player.paused && !nativeControls) return requestPause();
    requestPlay("control").catch(() => {});
    if (navigator.maxTouchPoints > 0 && !document.fullscreenElement && !player.webkitDisplayingFullscreen) enterFullscreen().catch(reportFullscreenFailure);
  }));
  controls.querySelectorAll("[data-player-back]").forEach((button) => button.addEventListener("click", () => { setPlayerTime(Math.max(0, player.currentTime - 10), true); }));
  controls.querySelectorAll("[data-player-forward]").forEach((button) => button.addEventListener("click", () => { setPlayerTime(Math.min(player.duration || Infinity, player.currentTime + 10), true); }));
  seek?.addEventListener("input", () => { scrubPosition = Number(seek.value); syncControls(); showSeekPreview(scrubPosition); });
  seek?.addEventListener("pointermove", (event) => {
    const bounds = seek.getBoundingClientRect();
    if (bounds.width > 0) showSeekPreview(Number(seek.max) * (event.clientX - bounds.left) / bounds.width);
  });
  seek?.addEventListener("pointerleave", () => { if (scrubPosition === undefined) hideSeekPreview(); });
  seek?.addEventListener("focus", () => showSeekPreview(Number(seek.value)));
  seek?.addEventListener("change", () => {
    const position = Number(seek.value);
    scrubPosition = undefined;
    if (Number.isFinite(position) && position >= 0 && position <= Number(seek.max)) setPlayerTime(position, true);
    syncControls();
    hideSeekPreview();
  });
  for (const event of ["pointercancel", "blur"]) seek?.addEventListener(event, () => { scrubPosition = undefined; syncControls(); hideSeekPreview(); });
  volume?.addEventListener("input", () => { player.muted = false; player.volume = Number(volume.value); syncControls(); });
  mute?.addEventListener("click", () => { player.muted = !player.muted; syncControls(); });
  const subtitleSelect = document.querySelector("[data-subtitles]");
  const selectableTracks = player.dataset.subtitlePickerLimited === "true" ? [...player.querySelectorAll("track[data-subtitle-source]")].map((element) => element.track) : null;
  const visibleTracks = () => selectableTracks || [...player.textTracks];
  let lastSubtitle = subtitleSelect?.value !== "off" ? subtitleSelect?.value : "0";
  const syncSubtitles = () => {
    const available = Boolean(subtitleSelect && subtitleSelect.options.length > 1);
    if (captions) captions.disabled = !available;
    if (captions) captions.title = available ? "Toggle subtitles" : "No subtitles available";
    captions?.setAttribute("aria-label", available ? "Subtitles" : "No subtitles available");
    const index = visibleTracks().findIndex((track) => track.mode === "showing");
    const selected = index < 0 ? "off" : String(index);
    if (subtitleSelect && [...subtitleSelect.options].some((option) => option.value === selected)) subtitleSelect.value = selected;
    if (index >= 0) lastSubtitle = selected;
    captions?.setAttribute("aria-pressed", String(index >= 0));
  };
  player.textTracks?.addEventListener?.("change", syncSubtitles);
  if (subtitleSelect) new MutationObserver(syncSubtitles).observe(subtitleSelect, { childList: true });
  syncSubtitles();
  captions?.addEventListener("click", () => {
    const select = document.querySelector("[data-subtitles]");
    if (!select || select.options.length < 2) return;
    select.value = visibleTracks().some((track) => track.mode === "showing") ? "off" : lastSubtitle || "0";
    select.dispatchEvent(new Event("change", {bubbles: true}));
    syncSubtitles();
  });
  const fullscreen = document.querySelector("[data-player-fullscreen]");
  const fullscreenTarget = nativeControls ? player : stage;
  const elementFullscreen = document.fullscreenEnabled && fullscreenTarget.requestFullscreen;
  const preferNativeFullscreen = Boolean(player.webkitEnterFullscreen && (nativeControls || navigator.maxTouchPoints > 0));
  const fullscreenSupported = Boolean(elementFullscreen || player.webkitEnterFullscreen);
  if (fullscreen) {
    fullscreen.disabled = !fullscreenSupported;
    fullscreen.title = !fullscreenSupported ? "Fullscreen is unavailable in this browser"
      : (preferNativeFullscreen || !elementFullscreen) && player.dataset.subtitlePickerLimited === "true" ? "Fullscreen uses Safari’s native player and subtitle menu" : "Fullscreen";
  }
  const enterFullscreen = async () => {
    if (document.fullscreenElement) await document.exitFullscreen();
    else if (player.webkitDisplayingFullscreen) player.webkitExitFullscreen?.();
    else if (preferNativeFullscreen) player.webkitEnterFullscreen();
    else if (document.fullscreenEnabled && fullscreenTarget.requestFullscreen) await fullscreenTarget.requestFullscreen({navigationUI: "hide"});
    else if (player.webkitEnterFullscreen) player.webkitEnterFullscreen();
  };
  fullscreen?.addEventListener("click", () => enterFullscreen().catch(reportFullscreenFailure));
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
      setPlayerTime(Math.max(0, Math.min(duration, player.currentTime + (key === "arrowright" ? 10 : -10))), true);
    }
  });
  if (settingsPanel) {
    const help = document.createElement("p");
    help.textContent = `Keyboard: Space or K to play/pause, left/right arrows to seek 10 seconds, ${nativeControls ? "" : "T for Theater, "}Esc to close.`;
    settingsPanel.append(help);
  }
  for (const event of ["enterpictureinpicture", "leavepictureinpicture", "webkitpresentationmodechanged"]) player.addEventListener(event, syncPictureInPicture);
  syncPictureInPicture();
  let pictureTouchedAt = -Infinity;
  if (!nativeControls) player.addEventListener("touchend", () => { pictureTouchedAt = performance.now(); }, {passive: true});
  if (!nativeControls) player.addEventListener("click", (event) => {
    if (event.pointerType === "touch" || performance.now() - pictureTouchedAt < 1000) return;
    if (player.paused) requestPlay("media-element").catch(() => {});
    else requestPause();
  });
  for (const event of ["loadedmetadata", "durationchange", "timeupdate", "play", "playing", "pause", "volumechange", "error"]) player.addEventListener(event, syncControls);
  const syncFullscreen = () => fullscreen?.setAttribute("aria-label", document.fullscreenElement || player.webkitDisplayingFullscreen ? "Exit fullscreen" : "Enter fullscreen");
  document.addEventListener("fullscreenchange", syncFullscreen);
  for (const event of ["webkitbeginfullscreen", "webkitendfullscreen"]) player.addEventListener(event, syncFullscreen);
  syncControls();
}
