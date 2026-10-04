let progressRevision = 0;
// One page-owned pending position; never replay a closed page's session over newer state.
let pendingProgress, progressFlight, progressFailure = "", progressContinuation, queueSourceChanging = false;
const progressNotice = document.querySelector("[data-progress-notice]");
const progressStatus = document.querySelector("[data-progress-status]");
const progressRetry = document.querySelector("[data-progress-retry]");
const progressContinue = document.querySelector("[data-progress-continue]");
const progressText = (state, fallback) => progressStatus?.dataset[state] || fallback;
const progressItem = () => {
  try {
    const url = new URL(player.dataset.progress, location.href);
    return url.origin === location.origin ? url.pathname.match(/^\/progress\/([a-zA-Z0-9_-]{1,128})$/)?.[1] : undefined;
  } catch (_) { return undefined; }
};
const progressProfile = () => document.body.dataset.viewerProfile || "";
const ownsProgress = (value) => value && value.profile === progressProfile() && value.item === progressItem() &&
  value.revision === progressRevision && player.dataset.castActive !== "true" && player.dataset.offline !== "true";
const clearProgress = () => {
  pendingProgress = undefined; progressFailure = ""; progressContinuation = undefined;
  if (progressNotice) { progressNotice.hidden = true; progressNotice.removeAttribute("aria-busy"); }
  if (progressRetry) progressRetry.disabled = false;
  if (progressStatus) for (const key of ["progressFailure", "progressSession", "progressRevision", "progressRequestId"]) delete progressStatus.dataset[key];
};
const retryableProgress = () => ["network", "timeout", "server"].includes(progressFailure);
const showProgressFailure = () => {
  if (progressNotice) { progressNotice.hidden = false; progressNotice.removeAttribute("aria-busy"); }
  if (progressStatus) progressStatus.textContent = progressFailure === "authentication" ? progressText("authentication", "Your position is not saved. Reload this page to sign in again.") :
    progressFailure === "policy" || progressFailure === "invalid" ? progressText("policy", "Your position is not saved. Reload this page to check access to this title.") :
    progressFailure === "response" ? progressText("response", "Your position is not saved. Reload this page to reconnect to Kinosail Server.") :
    pendingProgress?.watched ? progressContinuation ? progressText("continue", "Watched status is not saved. Retry or continue without saving.") : progressText("watched", "Watched status is not saved. Retry while this page is open.") : progressText("unsaved", "Your latest position is not saved. Retry while this page is open.");
  if (progressRetry) { progressRetry.hidden = !retryableProgress(); progressRetry.disabled = false; }
  if (progressContinue) progressContinue.hidden = !progressContinuation;
};
const sendProgress = (closing = false) => {
  if (progressFlight && !closing) return progressFlight;
  // Unload cannot wait for an older response to dispatch the latest keepalive.
  let flight;
  flight = (async () => {
    let response;
    while (ownsProgress(pendingProgress)) {
      const observed = pendingProgress;
      const controller = new AbortController();
      let timedOut = false;
      const timeout = setTimeout(() => { timedOut = true; controller.abort(); }, 8000);
      let failure = "";
      if (progressNotice && !progressNotice.hidden) {
        progressNotice.setAttribute("aria-busy", "true");
        if (progressStatus) progressStatus.textContent = progressText("saving", "Saving progress…");
      }
      if (progressRetry) progressRetry.disabled = true;
      try {
        response = await fetch(player.dataset.progress, {
          method: "POST",
          headers: {"Content-Type": "application/x-www-form-urlencoded", "X-Playback-Session": playbackSession, ...(csrf ? {"X-Kinosail-CSRF": csrf} : {})},
          body: new URLSearchParams({seconds: observed.seconds, session: playbackSession, revision: observed.revision, ...(observed.watched ? {watched: true} : {})}),
          keepalive: true, signal: controller.signal,
        });
        if (response.redirected || response.status === 401) failure = "authentication";
        else if (response.status === 408 || response.status === 429 || response.status >= 500) failure = "server";
        else if (response.status >= 400) failure = "policy";
        else if (response.status !== 204) failure = "response";
      } catch (_) { failure = timedOut ? "timeout" : "network"; response = undefined; }
      finally { clearTimeout(timeout); }
      if (progressFlight !== flight) break;
      if (observed !== pendingProgress) continue;
      if (!ownsProgress(observed)) { clearProgress(); break; }
      if (failure) {
        if (progressStatus) {
          const requestID = response?.headers?.get("X-Request-ID") || "";
          Object.assign(progressStatus.dataset, {progressFailure: failure, progressRevision: String(observed.revision),
            progressSession: /^[a-zA-Z0-9_-]{8,64}$/.test(playbackSession) ? playbackSession : "",
            progressRequestId: /^[a-zA-Z0-9_-]{1,64}$/.test(requestID) ? requestID : ""});
        }
        progressFailure = failure; showProgressFailure(); response = {ok: false}; break;
      }
      const continuation = progressContinuation;
      clearProgress();
      if (continuation) await continuation();
      break;
    }
    return response;
  })().finally(() => { if (progressFlight === flight) progressFlight = undefined; });
  progressFlight = flight;
  return flight;
};
const retryProgress = () => {
  if (!ownsProgress(pendingProgress)) { clearProgress(); return; }
  if (retryableProgress()) return sendProgress();
};
progressRetry?.addEventListener("click", retryProgress);
progressContinue?.addEventListener("click", () => {
  if (!ownsProgress(pendingProgress)) { clearProgress(); return; }
  const continuation = progressContinuation;
  clearProgress();
  continuation?.();
});
addEventListener("online", retryProgress);
const save = (watched = false, closing = false) => {
  if (playbackPreparation || queueSourceChanging) return Promise.resolve();
  if (player.dataset.castActive === "true" || player.dataset.offline === "true") {
    clearProgress();
    return player.dataset.offline === "true" ? window.KinosailOfflineMedia?.saveProgress(player, watched) : Promise.resolve();
  }
  if (pendingProgress && !ownsProgress(pendingProgress)) clearProgress();
  if (pendingProgress && progressFailure && !retryableProgress() && !watched) return Promise.resolve({ok: false});
  if (pendingProgress?.watched && !watched) return closing ? sendProgress(true) : progressFlight || Promise.resolve({ok: false});
  const seconds = watched ? 0 : player.currentTime;
  if (!progressItem() || !Number.isFinite(seconds) || seconds < 0 || seconds > 31536000 || progressProfile().length > 128) {
    progressFailure = "invalid"; showProgressFailure(); return Promise.resolve({ok: false});
  }
  pendingProgress = {profile: progressProfile(), item: progressItem(), seconds, watched, revision: ++progressRevision};
  if (progressFailure && !retryableProgress()) { showProgressFailure(); return Promise.resolve({ok: false}); }
  return sendProgress(closing);
};
player.addEventListener("play", () => { if (pendingProgress?.watched) clearProgress(); });
const resumeFromSavedProgress = () => {
  if (player.dataset.offline === "true") return;
  const start = Number(player.dataset.start);
  if (start > 0 && start < player.duration - 10) {
    if (Math.abs(player.currentTime - start) >= 0.1) setPlayerTime(start);
    if (player.hasAttribute("data-autoplay") && playbackTraceMethod !== "native-hls") requestPlay("resume-progress").catch(() => {});
  }
};
if (player.readyState) resumeFromSavedProgress();
else player.addEventListener("loadedmetadata", resumeFromSavedProgress, {once: true});
player.addEventListener("pause", () => {
  if (preparationPausePending) { preparationPausePending--; return; }
  save(false);
});
document.addEventListener("visibilitychange", () => {
  if (document.visibilityState === "hidden") {
    if (player.readyState >= HTMLMediaElement.HAVE_METADATA && !player.ended) save(false);
    flushPlaybackTrace();
  }
});
player.addEventListener("ended", async () => {
  if (playbackPreparation) return;
  if (player.dataset.castActive === "true") return;
  const continuePlayback = async () => {
    if (player.dataset.queue && await advanceQueue()) return;
    if (player.dataset.next) location.assign(player.dataset.next);
  };
  if (player.dataset.offline === "true") {
    if ((await save(true))?.ok) await continuePlayback();
    return;
  }
  progressContinuation = audioQueue.length || player.dataset.next ? continuePlayback : undefined;
  await save(true);
});
addEventListener("pagehide", () => {
  if (player.readyState >= HTMLMediaElement.HAVE_METADATA && !player.ended) save(false, true);
});
setInterval(() => { if (!player.paused) save(); }, 10000);
setInterval(() => { if (!player.paused || player.readyState < HTMLMediaElement.HAVE_FUTURE_DATA) playbackTrace("heartbeat", "periodic"); flushPlaybackTrace(); }, 5000);

if (player.dataset.homeAssistant === "true") {
  const storageKey = `kinosail-home-assistant-player:${document.body.dataset.viewerProfile || document.querySelector("[data-nav-profile]")?.dataset.navProfile || ""}`;
  let playerID = playerStorage.get(storageKey);
  if (!playerID) {
    playerID = crypto.randomUUID();
    playerStorage.set(storageKey, playerID);
  }
  const sendHomeAssistantState = async () => {
    const response = await fetch(`/api/v1/home-assistant/players/${encodeURIComponent(playerID)}`, {
      method: "PUT",
      headers: {"Content-Type": "application/json", ...(csrf ? {"X-Kinosail-CSRF": csrf} : {})},
      body: JSON.stringify({
        name: `Kinosail on ${navigator.userAgentData?.platform || navigator.platform || "web"}`,
        state: playbackPreparation ? "buffering" : player.ended ? "idle" : player.paused ? "paused" : player.readyState < 3 ? "buffering" : "playing",
        title: player.dataset.title || "",
        itemId: player.dataset.progress?.split("/").at(-1)?.split("?")[0] || "",
        position: Number.isFinite(player.currentTime) ? player.currentTime : 0,
        duration: Number.isFinite(player.duration) ? player.duration : 0,
        volume: player.volume,
        muted: player.muted,
      }),
    });
    if (!response.ok) return;
    const command = await response.json();
    if (command.command === "play") await requestPlay("home-assistant");
    if (command.command === "pause") requestPause();
    if (command.command === "stop") { requestPause(); setPlayerTime(0); }
    if (command.command === "seek") setPlayerTime(command.position);
    if (command.command === "volume") player.volume = command.volume;
    if (command.command === "mute") player.muted = command.muted;
    if (command.command === "play_media") location.assign(`/watch/${encodeURIComponent(command.itemId)}`);
  };
  sendHomeAssistantState().catch(() => {});
  if ("EventSource" in window) {
    const events = new EventSource("/api/v1/events");
    events.addEventListener("home-assistant.command", ({data}) => {
      try {
        if (JSON.parse(data).resource === `/api/v1/home-assistant/players/${playerID}`) sendHomeAssistantState().catch(() => {});
      } catch (_) {}
    });
    addEventListener("pagehide", () => events.close(), {once: true});
  }
  setInterval(() => sendHomeAssistantState().catch(() => {}), 5000);
}

const audioChoice = document.querySelector('[data-audio-track]');
if (audioChoice) {
  const current = new URL(location.href).searchParams.get('audio');
  if (current !== null && [...audioChoice.options].some(option => option.value === current)) audioChoice.value = current;
  audioChoice.addEventListener('change', async () => {
    const value = audioChoice.value, status = document.querySelector('[data-audio-status]');
    if (!/^(0|[1-9][0-9]{0,4})$/.test(value)) return;
    audioChoice.disabled = true;
    try {
      const response = await save(false);
      if (!response?.ok) throw new Error();
      const target = new URL(location.href);
      target.searchParams.delete('direct');
      target.searchParams.set('compatible', '1');
      target.searchParams.set('audio', value);
      location.assign(target.pathname + target.search);
    } catch (_) {
      audioChoice.disabled = false;
      if (status) status.textContent = 'Could not save your position. Try changing the audio again.';
    }
  });
}
