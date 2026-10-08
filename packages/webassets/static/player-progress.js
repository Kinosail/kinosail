let progressRevision = 0, progressPlayedItem;
let queueSourceChanging = false, queueProgressReady = true;
// One page-owned pending position; never replay a closed page's session over newer state.
let pendingProgress, progressFlight, progressFailure = "", progressContinuation, progressNavigation;
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
const progressChanged = () => Boolean(progressItem()) && progressPlayedItem === progressItem();
const ownsProgress = (value) => value && value.profile === progressProfile() && value.item === progressItem() &&
  value.revision === progressRevision && player.dataset.castActive !== "true" && player.dataset.offline !== "true";
const clearProgress = (continueNavigation = false) => {
  if (!continueNavigation) cancelProgressNavigation();
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
      if (progressNavigation && !ownsProgressNavigation()) cancelProgressNavigation();
      const remaining = progressNavigation ? Math.min(8000, progressNavigation.deadline - performance.now()) : 8000;
      let timedOut = remaining <= 0;
      const timeout = setTimeout(() => { timedOut = true; controller.abort(); }, Math.max(0, remaining));
      let failure = "";
      if (progressNotice && !progressNotice.hidden) {
        progressNotice.setAttribute("aria-busy", "true");
        if (progressStatus) progressStatus.textContent = progressText("saving", "Saving progress…");
      }
      if (progressRetry) progressRetry.disabled = true;
      try {
        if (timedOut) await Promise.reject(new Error("progress deadline elapsed"));
        response = await fetch(player.dataset.progress, {
          method: "POST",
          headers: {"Content-Type": "application/x-www-form-urlencoded", "X-Playback-Session": playbackSession, ...(csrf ? {"X-Kinosail-CSRF": csrf} : {})},
          body: new URLSearchParams({seconds: observed.seconds, session: playbackSession, revision: observed.revision, watched: observed.watched}),
          keepalive: true, signal: controller.signal,
        });
        if (response.redirected || response.status === 401) failure = "authentication";
        else if (response.status === 408 || response.status === 429 || response.status >= 500) failure = "server";
        else if (response.status >= 400) failure = "policy";
        else if (response.status !== 204) failure = "response";
        if (!failure && progressNavigation && performance.now() >= progressNavigation.deadline) failure = "timeout";
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
      clearProgress(continuation === progressNavigation?.leave);
      if (continuation) await continuation();
      if (progressFlight === flight && ownsProgress(pendingProgress)) continue;
      break;
    }
    return response;
  })().finally(() => { if (progressFlight === flight) progressFlight = undefined; });
  progressFlight = flight;
  return flight;
};
const retryProgress = (explicit = false) => {
  if (!ownsProgress(pendingProgress)) { clearProgress(); return; }
  if (progressNavigation && !ownsProgressNavigation()) cancelProgressNavigation();
  if (progressNavigation) {
    if (explicit) progressNavigation.deadline = performance.now() + 8000;
    else if (performance.now() >= progressNavigation.deadline) return;
  }
  if (retryableProgress()) return sendProgress();
};
progressRetry?.addEventListener("click", () => retryProgress(true));
progressContinue?.addEventListener("click", () => {
  if (!ownsProgress(pendingProgress)) { clearProgress(); return; }
  const continuation = progressContinuation;
  clearProgress(continuation === progressNavigation?.leave);
  continuation?.();
});
addEventListener("online", () => retryProgress());
const save = (watched = false, closing = false) => {
  if (playbackPreparation || queueSourceChanging || !queueProgressReady) return Promise.resolve();
  if (!watched && !progressChanged()) return Promise.resolve({ok: true});
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
for (const event of ["playing", "kinosail:seek-intent"]) player.addEventListener(event, () => { if (!playbackPreparation && !queueSourceChanging && queueProgressReady && (event !== "playing" || !player.paused)) progressPlayedItem = progressItem(); });
player.addEventListener("seeking", () => { if (!managedSeek && !playbackPreparation && !queueSourceChanging && queueProgressReady) progressPlayedItem = progressItem(); });
const resumeFromSavedProgress = () => {
  if (player.dataset.offline === "true") return;
  const start = Number(player.dataset.start);
  if (start > 0 && start < player.duration) {
    if (Math.abs(player.currentTime - start) >= 0.1) setPlayerTime(start);
    if (player.hasAttribute("data-autoplay") && playbackTraceMethod !== "native-hls") requestPlay("resume-progress").catch(() => {});
  }
};
if (player.readyState) resumeFromSavedProgress();
else player.addEventListener("loadedmetadata", resumeFromSavedProgress, {once: true});
player.addEventListener("pause", () => {
  if (preparationPausePending) { preparationPausePending--; return; }
  if (player.readyState >= HTMLMediaElement.HAVE_METADATA && !player.ended) save(false);
});
player.addEventListener("seeked", () => {
  const prepared = preparationSeek;
  preparationSeek = undefined;
  if (prepared && prepared.source === (player.currentSrc || player.src) && Math.abs(prepared.seconds - player.currentTime) < 0.1) return;
  if (player.paused && player.readyState >= HTMLMediaElement.HAVE_METADATA && !player.ended) save(false);
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
player.addEventListener("kinosail:page-exit", () => {
  cancelProgressNavigation();
  if (player.readyState >= HTMLMediaElement.HAVE_METADATA && !player.ended) save(false, true);
});
setInterval(() => { if (!player.paused) save(); }, 10000);
setInterval(() => { if (!player.paused || player.readyState < HTMLMediaElement.HAVE_FUTURE_DATA) playbackTrace("heartbeat", "periodic"); flushPlaybackTrace(); }, 5000);

if (player.dataset.homeAssistant === "true") {
  let homeAssistantClosed = false;
  const homeAssistantStatus = document.createElement("div"), homeAssistantMessage = document.createElement("span");
  const homeAssistantLifetime = document.createElement("small"), homeAssistantRetry = document.createElement("button");
  homeAssistantStatus.className = "player-actions";
  homeAssistantStatus.dataset.homeAssistantStatus = "connecting";
  homeAssistantMessage.setAttribute("role", "status");
  homeAssistantMessage.setAttribute("aria-live", "polite");
  homeAssistantLifetime.dataset.homeAssistantLifetime = "";
  homeAssistantRetry.type = "button"; homeAssistantRetry.className = "quiet-button";
  homeAssistantRetry.style.minHeight = "44px";
  homeAssistantRetry.textContent = "Retry Home Assistant"; homeAssistantRetry.hidden = true;
  homeAssistantStatus.append(homeAssistantMessage, homeAssistantLifetime, homeAssistantRetry);
  (player.closest(".media-stage") || player).insertAdjacentElement("afterend", homeAssistantStatus);
  const boundedText = (value, limit) => {
    let result = "", size = 0;
    for (const character of String(value || "")) {
      const bytes = new TextEncoder().encode(character).length;
      if (size + bytes > limit) break;
      result += character; size += bytes;
    }
    return result;
  };
  addEventListener("pagehide", () => {homeAssistantClosed = true;});
  addEventListener("pageshow", event => {
    // The media owner must actually restore a source before document re-entry.
    if (event.persisted && player.isConnected && player.getAttribute("src")) homeAssistantClosed = false;
  });
  const homeAssistantClient = createHomeAssistantDocumentPlayer({
    profile: progressProfile, csrf: () => csrf, current: () => player.isConnected && !homeAssistantClosed,
    snapshot: () => ({source: `${progressItem() || ""}:${player.currentSrc || player.src}:${playbackSession}`, body: {
      name: boundedText(`Kinosail on ${navigator.userAgentData?.platform || navigator.platform || "web"}`, 80),
      state: playbackPreparation ? "buffering" : player.ended ? "idle" : player.paused ? "paused" : player.readyState < 3 ? "buffering" : "playing",
      title: boundedText(player.dataset.title, 256), itemId: boundedText(progressItem(), 128),
      position: Number.isFinite(player.currentTime) ? Math.max(0, Math.min(1e9, player.currentTime)) : 0,
      duration: Number.isFinite(player.duration) ? Math.max(0, Math.min(1e9, player.duration)) : 0,
      volume: player.volume, muted: player.muted,
    }}),
    diagnose: value => Object.assign(player.dataset, {homeAssistantOperation: value.operation, homeAssistantFailure: value.failure,
      homeAssistantLevel: value.level, homeAssistantGeneration: String(value.generation), homeAssistantRequestId: value.requestID}),
    status: value => {
      homeAssistantStatus.dataset.homeAssistantStatus = value.state;
      homeAssistantMessage.textContent = {connecting: "Connecting Home Assistant…", waiting: "Waiting for this Player’s previous connection…",
        connected: "Home Assistant can control this Player.", unavailable: "Home Assistant could not connect. Retry to reconnect this Player.",
        stopped: "Home Assistant control has stopped for this page."}[value.state];
      homeAssistantLifetime.textContent = !value.storageAvailable ? "Browser storage is unavailable. Reloading creates a new Home Assistant target." :
        value.forkedCandidate ? "This fresh page received its own target. Reloads keep it while browser storage is available." :
        "Each open Player page has its own target. Reloads keep it while browser storage is available.";
      homeAssistantRetry.hidden = !value.retry;
    },
    apply: async command => {
      if (command.command === "play") await requestPlay("home-assistant");
      if (command.command === "pause") requestPause();
      if (command.command === "stop") {requestPause(); setPlayerTime(0, true);}
      if (command.command === "seek") setPlayerTime(command.position, true);
      if (command.command === "volume") player.volume = command.volume;
      if (command.command === "mute") player.muted = command.muted;
      if (command.command === "play_media") location.assign(`/watch/${encodeURIComponent(command.itemId)}`);
    },
  });
  homeAssistantRetry.addEventListener("click", () => {void homeAssistantClient.retry();});
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
