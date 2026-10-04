// Queue metadata stays page-owned. Fresh canonical item reads authorize each move.
let audioQueue = [], queueItems = [], queueCursor = 0, queueFlight, queuedAudio, queueReadRequestID = "";
const queueControls = document.querySelector("[data-audio-queue-controls]");
const queueStatus = document.querySelector("[data-audio-queue-status]");
const queuePrevious = document.querySelector("[data-audio-previous]");
const queueNext = document.querySelector("[data-audio-next]");
const queueRetry = document.querySelector("[data-audio-queue-retry]");
const queueInitialItem = progressItem(), queueInitialProfile = progressProfile();
const queueID = value => typeof value === "string" && /^[a-zA-Z0-9_-]{1,128}$/.test(value);
const queueText = value => value === undefined || typeof value === "string" && value.length <= 65536;
const queuePath = (value, path) => {
  if (typeof value !== "string" || value.length > 256) return false;
  try { const url = new URL(value, location.href); return url.origin === location.origin && !url.username && !url.password && !url.search && !url.hash && url.pathname === path; }
  catch (_) { return false; }
};
const queueError = (failure, response) => ({failure, requestID: response?.headers?.get("X-Request-ID") || ""});
const queueItem = item => {
  if (!item || typeof item !== "object" || !queueID(item.id) || item.kind !== "audio" || typeof item.title !== "string" || !item.title ||
      ![item.title, item.artist, item.album, item.rating].every(queueText) || !queuePath(item.stream, `/media/${item.id}`) ||
      !queueText(item.artwork) || item.artwork && !queuePath(item.artwork, `/art/${item.id}`) ||
      item.progress !== undefined && (!item.progress || typeof item.progress !== "object" || Array.isArray(item.progress)) ||
      ![item.track ?? 0, item.year ?? 0].every(value => Number.isSafeInteger(value) && value >= 0 && value <= 100000) ||
      !Number.isFinite(item.progress?.seconds ?? 0) || (item.progress?.seconds ?? 0) < 0 || (item.progress?.seconds ?? 0) > 31536000) throw queueError("invalid");
  return {id: item.id, title: item.title, artist: item.artist || "", album: item.album || "", track: item.track || 0,
    year: item.year || 0, rating: item.rating || "", stream: `/media/${item.id}`, artwork: item.artwork ? `/art/${item.id}` : "",
    progress: {seconds: item.progress?.seconds || 0}};
};
const queueJSON = async path => {
  let response, reader;
  const signal = AbortSignal.timeout(10000);
  queueReadRequestID = "";
  try {
    response = await fetch(path, {cache: "no-store", credentials: "same-origin", signal});
    queueReadRequestID = response.headers.get("X-Request-ID") || "";
    if (response.redirected || response.status === 401) throw queueError("authentication", response);
    if (response.status === 403 || response.status === 404) throw queueError("policy", response);
    if (response.status === 408 || response.status === 429 || response.status >= 500) throw queueError("server", response);
    if (response.status !== 200 || !response.body) throw queueError("response", response);
    reader = response.body.getReader();
    const decoder = new TextDecoder("utf-8", {fatal: true});
    let length = 0, text = "";
    while (true) {
      const {value, done} = await reader.read();
      if (done) break;
      length += value.byteLength;
      if (length > 4 * 1024 * 1024) throw queueError("invalid", response);
      try { text += decoder.decode(value, {stream: true}); } catch (_) { throw queueError("invalid", response); }
    }
    try {
      const body = JSON.parse(text + decoder.decode());
      if (!body || typeof body !== "object" || Array.isArray(body)) throw queueError("invalid", response);
      return body;
    } catch (_) { throw queueError("invalid", response); }
  } catch (error) { throw error?.failure ? error : queueError(signal.aborted ? "timeout" : "network", response); }
  finally { await reader?.cancel().catch(() => {}); }
};
const queueOwned = () => progressProfile() === queueInitialProfile && player.dataset.castActive !== "true" && !player.dataset.room;
const queueContext = () => ({profile: progressProfile(), item: progressItem(), cursor: queueCursor, source: player.src});
const queueCurrent = context => queueOwned() && context.profile === progressProfile() && context.item === progressItem() &&
  context.cursor === queueCursor && context.source === player.src;
const queuePositionText = () => `${queueStatus?.dataset.trackLabel || "Track"} ${queueCursor + 1} ${queueStatus?.dataset.ofLabel || "of"} ${queueItems.length}`;
const queueHandlers = () => {
  for (const [action, handler] of [["previoustrack", queueCursor > 0 && queueOwned() ? () => moveAudioQueue(-1) : null],
    ["nexttrack", audioQueue.length && queueOwned() ? () => moveAudioQueue(1) : null]]) {
    try { navigator.mediaSession?.setActionHandler(action, handler); } catch (_) { /* Page controls remain usable. */ }
  }
};
const refreshQueueControls = () => {
  const enabled = queueOwned() && !queueFlight && !queueSourceChanging;
  if (queuePrevious) queuePrevious.disabled = !enabled || queueCursor <= 0;
  if (queueNext) queueNext.disabled = !enabled || !audioQueue.length;
  if (queueFlight || queueSourceChanging) queueControls?.setAttribute("aria-busy", "true");
  else queueControls?.removeAttribute("aria-busy");
  queueHandlers();
};
const reportQueueFailure = error => {
  if (!queueStatus) return;
  const failure = error?.failure || "network", requestID = failure === "progress" ? progressStatus?.dataset.progressRequestId || "" : error?.requestID || queueReadRequestID;
  Object.assign(queueStatus.dataset, {queueFailure: failure,
    queueOperation: failure === "progress" ? "progress-save" : ["playback", "media"].includes(failure) ? "playback" : queueItems.length ? "item-read" : "queue-read",
    queueSession: /^[a-zA-Z0-9_-]{8,64}$/.test(playbackSession) ? playbackSession : "",
    queueRequestId: /^[a-zA-Z0-9_-]{1,64}$/.test(requestID) ? requestID : ""});
  queueStatus.textContent = failure === "authentication" ? "Could not load this track. Reload to sign in again." :
    failure === "ownership" ? "Playback ownership changed. Reload to check this queue." :
    failure === "policy" || failure === "invalid" || failure === "response" ? "Could not load this track. Reload to check access to this queue." :
    failure === "progress" ? "Your position could not be saved. Retry saving, then choose the track again." :
    failure === "media" ? "This track could not play. Open its details to check the file." :
    failure === "playback" ? "Press Play to start this track." : "Could not load this track. Try the track control again.";
};
const clearQueueFailure = () => {
  if (queueStatus) {
    for (const key of ["queueFailure", "queueSession", "queueRequestId", "queueOperation"]) delete queueStatus.dataset[key];
    queueStatus.textContent = player.dataset.room ? "Use room controls to change the shared track." : player.dataset.castActive === "true" ? "Use the receiver controls while casting." : queuePositionText();
  }
};
const warmAudio = () => {
  if (!audioQueue.length || !queueOwned()) return;
  queuedAudio = new Audio(withPlaybackSession(audioQueue[0].stream)); queuedAudio.preload = "auto";
};
const scopeCurrentTrackActions = item => {
  document.querySelectorAll(".primary-player-actions:not([data-progress-notice]),.owner-tools,.media-information,.cast-section,details.chapters,.title-tagline,.title-summary,.title-facts,.curation-menu,.playback-tools").forEach(element => element.hidden = true);
  const actions = document.querySelector("[data-current-track-actions]"), details = document.querySelector("[data-current-track-details]");
  if (details) details.setAttribute("href", `/watch/${item.id}`);
  if (actions) actions.hidden = false;
};
const commitAudioQueueItem = (item, cursor) => {
  // Dispatch old buffered events before the endpoint changes. No asynchronous
  // work separates the validated source, all identities, and displayed metadata.
  flushPlaybackTrace(); queueSourceChanging = true; queueProgressReady = false;
  window.KinosailOfflineMedia?.unbindProgress(player); delete player.dataset.offline;
  Object.assign(player.dataset, {progress: `/progress/${item.id}`, title: item.title, artist: item.artist, album: item.album,
    track: String(item.track), artwork: item.artwork, start: String(item.progress.seconds),
    playbackTrace: `/api/v1/items/${item.id}/playback-events`, queuePosition: String(cursor + 1), queueTotal: String(queueItems.length)});
  if (player.dataset.castApi) player.dataset.castApi = `/api/v1/items/${item.id}/cast`;
  playbackTimelineOffset = 0; playbackTraceMethod = "direct";
  queueCursor = cursor; queueItems[cursor] = item; audioQueue = queueItems.slice(cursor + 1);
  player.src = withPlaybackSession(item.stream);
  updateNowPlaying(item); scopeCurrentTrackActions(item); clearQueueFailure();
  try { navigator.mediaSession?.setPositionState(); } catch (_) {}
  player.addEventListener("loadedmetadata", () => {
    if (progressItem() !== item.id) return;
    queueSourceChanging = false; queueProgressReady = true;
    const start = item.progress.seconds;
    if (start > 0 && start < player.duration - 10) setPlayerTime(start);
    refreshQueueControls();
  }, {once: true});
  player.load(); warmAudio();
};
const moveAudioQueue = (delta, saveCurrent = true) => {
  const cursor = queueCursor + delta;
  if (queueFlight || queueSourceChanging || !queueOwned() || cursor < 0 || cursor >= queueItems.length) return Promise.resolve(false);
  const context = queueContext();
  let flight;
  flight = (async () => {
    try {
      queueControls?.setAttribute("aria-busy", "true");
      if (queueStatus) queueStatus.textContent = "Loading track…";
      const target = queueItems[cursor];
      const body = await queueJSON(`/api/v1/items/${target.id}`);
      if (!queueCurrent(context)) throw queueError("ownership");
      if (body.profileId !== context.profile || body.item?.id !== target.id) throw queueError("invalid");
      const item = queueItem(body.item);
      // Rejected authorization never writes the current track's progress.
      if (saveCurrent && queueProgressReady && !(await save(player.ended))?.ok) throw queueError("progress");
      if (!queueCurrent(context)) throw queueError("ownership");
      commitAudioQueueItem(item, cursor);
      try { await requestPlay("queue-advance"); } catch (_) { reportQueueFailure(queueError(player.error ? "media" : "playback")); }
      return true;
    } catch (error) { reportQueueFailure(error); return false; }
    finally { if (queueFlight === flight) { queueFlight = undefined; refreshQueueControls(); } }
  })();
  queueFlight = flight; refreshQueueControls(); return flight;
};
const advanceQueue = () => moveAudioQueue(1, false);
const loadAudioQueue = () => {
  if (queueFlight || !queueID(queueInitialItem) || progressItem() !== queueInitialItem || progressProfile() !== queueInitialProfile) return;
  if (queueInitialProfile.length > 128) { reportQueueFailure(queueError("invalid")); refreshQueueControls(); return; }
  if (!queuePath(player.dataset.queue, `/api/v1/audio/${queueInitialItem}/queue`)) { reportQueueFailure(queueError("invalid")); refreshQueueControls(); return; }
  let flight;
  flight = (async () => {
    try {
      queueControls?.setAttribute("aria-busy", "true");
      const {items} = await queueJSON(player.dataset.queue);
      if (progressItem() !== queueInitialItem || progressProfile() !== queueInitialProfile) throw queueError("ownership");
      if (!Array.isArray(items) || !items.length || items.length > 10000 || items[0]?.id !== queueInitialItem) throw queueError("invalid");
      const validated = items.map(queueItem);
      if (new Set(validated.map(item => item.id)).size !== validated.length) throw queueError("invalid");
      queueItems = validated; audioQueue = validated.slice(1);
      Object.assign(player.dataset, {queuePosition: "1", queueTotal: String(validated.length)});
      if (queueRetry) queueRetry.hidden = true;
      clearQueueFailure(); warmAudio();
    } catch (error) { reportQueueFailure(error); if (queueRetry) queueRetry.hidden = false; }
    finally { if (queueFlight === flight) { queueFlight = undefined; refreshQueueControls(); } }
  })();
  queueFlight = flight; refreshQueueControls();
};
queuePrevious?.addEventListener("click", () => moveAudioQueue(-1));
queueNext?.addEventListener("click", () => moveAudioQueue(1));
queueRetry?.addEventListener("click", loadAudioQueue);
player.addEventListener("error", () => { if (queueSourceChanging) { queueSourceChanging = false; reportQueueFailure(queueError("media")); refreshQueueControls(); } });
if (player.dataset.queue) {
  new MutationObserver(() => { clearQueueFailure(); refreshQueueControls(); }).observe(player, {attributes: true, attributeFilter: ["data-cast-active", "data-room"]});
  loadAudioQueue();
}
