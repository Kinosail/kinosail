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
