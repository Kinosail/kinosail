// Progress lives with the verified, profile-owned download. Never create a job
// while saving progress: removal or a replacement transfer wins the race.
function offlineProgress(value, required = false) {
  if (!value || typeof value !== "object" || Array.isArray(value)) throw new Error("Saved progress is invalid.");
  if (Object.keys(value).sort().join(",") !== "revision,seconds,session,watched") throw new Error("Saved progress is invalid.");
  const {seconds, watched, session, revision} = value;
  if (!Number.isFinite(seconds) || seconds < 0 || seconds > 31536000 || typeof watched !== "boolean" || typeof session !== "string" || session.length > 128 || /[\x00-\x1f\x7f]/.test(session) || !Number.isSafeInteger(revision) || revision < 0 || (required && (!session || !revision))) throw new Error("Saved progress is invalid.");
  return {seconds, watched, session, revision};
}
function offlineServerProgress(value) {
  if (!value || typeof value !== "object" || Array.isArray(value) || Object.keys(value).some(key => !["seconds", "watched", "session", "revision", "updated", "dismissed", "readerOffset", "readerPage"].includes(key))) throw new Error("Progress response is invalid.");
  return offlineProgress({seconds: value.seconds ?? 0, watched: value.watched ?? false, session: value.session ?? "", revision: value.revision ?? 0});
}
function validOfflineProgressJob(job) {
  return job && /^[0-9a-f]{16}$/.test(job.id || "") && offlineItemID.test(job.itemID || "") && offlineProfileID.test(job.profileID || "") && /^[0-9a-f]{64}$/.test(job.sha256 || "") && Boolean(offlineTransferIDOf(job));
}
const offlineProgressMatches = (left, right) => left?.profileID === right.profileID && left.itemID === right.itemID && left.sha256 === right.sha256 && offlineTransferIDOf(left) && offlineTransferIDOf(left) === offlineTransferIDOf(right) && left.state === "ready" && left.readyOffline;
async function updateOfflineProgress(job, change) {
  if (!offlineItemID.test(job.itemID || "") || !offlineProfileID.test(job.profileID || "") || !/^[0-9a-f]{16}$/.test(job.id || "") || activeOfflineProfile() !== job.profileID) throw new Error("Open this Viewer Profile to save progress.");
  const database = await openOfflineDatabase();
  return new Promise((resolve, reject) => {
    const transaction = database.transaction("jobs", "readwrite", {durability: "strict"}), store = transaction.objectStore("jobs");
    let result;
    store.get(job.id).onsuccess = ({target}) => {
      const current = target.result;
      if (!offlineProgressMatches(current, job)) { transaction.abort(); return; }
      try { result = change(current); store.put(current); } catch (_) { transaction.abort(); }
    };
    transaction.oncomplete = () => resolve(result);
    transaction.onabort = () => reject(new Error("Could not save offline progress. Return to Downloads and try again."));
  });
}
async function beginOfflineTransfer(record, previousTransferID) {
  const database = await openOfflineDatabase();
  return new Promise((resolve, reject) => {
    const transaction = database.transaction("jobs", "readwrite", {durability: "strict"});
    const store = transaction.objectStore("jobs");
    store.get(record.id).onsuccess = ({target}) => {
      const current = target.result;
      if (current && (offlineTransferIDOf(current) !== previousTransferID || current.profileID !== record.profileID || current.itemID !== record.itemID || current.sha256 !== record.sha256)) { transaction.abort(); return; }
      try {
        if (current) for (const key of ["progressBaseline", "localProgress", "pendingProgress"]) {
          if (current[key]) record[key] = offlineProgress(current[key], key !== "progressBaseline");
          else if (key !== "progressBaseline") delete record[key];
        }
        store.put(record);
      } catch (_) { transaction.abort(); }
    };
    transaction.oncomplete = resolve;
    transaction.onabort = () => reject(new Error("Download changed. Refresh Downloads and try again."));
  });
}

async function offlineProgressJSON(response) {
  if (!response.body) throw new Error("The Server response is empty.");
  const reader = response.body.getReader();
  const bytes = new Uint8Array(65536);
  let length = 0;
  try {
    for (;;) {
      const {done, value} = await reader.read();
      if (done) break;
      if (length + value.byteLength > bytes.length) throw new Error("The Server response is too large.");
      bytes.set(value, length);
      length += value.byteLength;
    }
    return JSON.parse(new TextDecoder("utf-8", {fatal: true}).decode(bytes.subarray(0, length)));
  } finally { await reader.cancel().catch(() => {}); }
}
async function downloadProgressBaseline(itemID, profileID, signal) {
  if (!offlineItemID.test(itemID || "") || !offlineProfileID.test(profileID || "")) throw new Error("Download identity is invalid.");
  const response = await fetch(`/api/v1/items/${encodeURIComponent(itemID)}`, {credentials: "same-origin", cache: "no-store", signal: signal ? AbortSignal.any([signal, AbortSignal.timeout(10000)]) : AbortSignal.timeout(10000)});
  if (!response.ok) throw new Error("Could not read progress before downloading. Try again.");
  const body = await offlineProgressJSON(response);
  if (body.profileId !== profileID || body.item?.id !== itemID) throw new Error("Download identity changed. Try again.");
  return offlineServerProgress(body.item.progress);
}
let syncingOfflineProgress = false;
async function syncOfflineProgress() {
  // Only an online, Server-rendered page supplies an authoritative profile and
  // CSRF token. The cached offline shell must never pick a sync identity.
  const profile = currentOfflineProfile(), csrf = document.querySelector('meta[name="kinosail-csrf"]')?.content;
  if (syncingOfflineProgress || !profile || !hasOfflineStorage || !navigator.onLine) return;
  syncingOfflineProgress = true;
  try {
    const records = await getOfflineJobs();
    if (!Array.isArray(records) || records.length > 1000) return;
    const legacy = records.filter(job => validOfflineProgressJob(job) && job.profileID === profile && !job.progressBaseline && job.localProgress && job.state === "ready" && job.readyOffline).slice(0, 50);
    for (const job of legacy) {
      const baseline = await downloadProgressBaseline(job.itemID, job.profileID);
      await updateOfflineProgress(job, current => {
        if (current.progressBaseline) return;
        current.progressBaseline = baseline;
        const local = offlineProgress(current.localProgress, true);
        if (local.seconds > baseline.seconds) current.pendingProgress = local;
      });
    }
    const jobs = (await getOfflineJobs()).filter(job => validOfflineProgressJob(job) && job.profileID === profile && job.pendingProgress && job.state === "ready" && job.readyOffline).slice(0, 50);
    for (const job of jobs) {
      if (currentOfflineProfile() !== profile) break;
      let progress, expected;
      try { progress = offlineProgress(job.pendingProgress, true); expected = offlineProgress(job.progressBaseline); } catch (_) { continue; }
      const response = await fetch(`/api/v1/items/${encodeURIComponent(job.itemID)}/progress/sync`, {
        method: "PUT", credentials: "same-origin", signal: AbortSignal.timeout(10000),
        headers: {"Content-Type": "application/json", "X-Kinosail-Viewer-Profile": profile, ...(csrf ? {"X-Kinosail-CSRF": csrf} : {})},
        body: JSON.stringify({progress, expected, playbackToken: ""}),
      });
      if (![200, 409].includes(response.status)) continue;
      const body = await offlineProgressJSON(response), saved = offlineServerProgress(response.status === 409 ? body.progress : body);
      await updateOfflineProgress(job, current => {
        if (current.pendingProgress?.session !== progress.session || current.pendingProgress?.revision !== progress.revision) return;
        current.progressBaseline = saved;
        // Match native reconciliation: keep a further local position queued,
        // retrying against the fresh server baseline on the next pass.
        if (response.status === 409 && progress.seconds > saved.seconds) return;
        current.localProgress = saved;
        delete current.pendingProgress;
      });
    }
  } catch (_) { /* Local progress remains queued for the next connection. */ }
  finally { syncingOfflineProgress = false; }
}
function bindOfflineProgress(media, job, status, enabled = () => true) {
  let saved;
  try { saved = offlineProgress(job.localProgress ?? job.progressBaseline); }
  catch (_) { saved = {seconds: 0, watched: false, session: "", revision: 0}; }
  const session = `web-offline-${crypto.randomUUID()}`;
  let revision = 0, restored = false, lastSaved = 0, writes = Promise.resolve();
  const message = value => { if (status) status.textContent = value; };
  const restore = () => {
    if (!enabled()) return;
    if (!restored && !saved.watched && saved.seconds > 0 && saved.seconds < media.duration - 1) media.currentTime = saved.seconds;
    restored = true;
  };
  const save = (watched = false) => {
    if (!enabled() || !restored || !Number.isFinite(media.currentTime)) return Promise.resolve({ok: false});
    const progress = offlineProgress({seconds: media.currentTime, watched, session, revision: ++revision}, true);
    writes = writes.catch(() => {}).then(() => updateOfflineProgress(job, current => {
      current.localProgress = progress;
      // Old downloads resume locally until an online page captures their baseline.
      if (current.progressBaseline) {
        offlineProgress(current.progressBaseline);
        current.pendingProgress = progress;
      }
    }));
    return writes.then(() => {
      message(offlineMessage("offlineProgressSaved", "Progress saved on this device. Open Server downloads to sync."));
      return {ok: true};
    }, () => {
      message(offlineMessage("offlineProgressFailed", "Could not save progress. Return to Downloads and try again."));
      return {ok: false};
    });
  };
  const time = () => { if (performance.now() - lastSaved > 5000 && !media.paused) { lastSaved = performance.now(); void save(false); } };
  const pause = () => { if (!media.ended) void save(false); };
  const ended = () => { void save(true); };
  const leave = () => { if (enabled()) { media.pause(); void save(media.ended); } };
  const visibility = () => { if (document.visibilityState === "hidden") void save(media.ended); };
  const error = () => message(offlineMessage("offlinePlaybackFailed", "This browser could not play the saved file. Reconnect to the Server and choose a compatible download quality."));
  const listeners = [["loadedmetadata", restore], ["timeupdate", time], ["pause", pause], ["ended", ended], ["error", error]];
  for (const [name, listener] of listeners) media.addEventListener(name, listener);
  addEventListener("pagehide", leave, {once: true});
  document.addEventListener("visibilitychange", visibility);
  return {save, detach: () => {
    for (const [name, listener] of listeners) media.removeEventListener(name, listener);
    removeEventListener("pagehide", leave);
    document.removeEventListener("visibilitychange", visibility);
  }};
}
addEventListener("online", () => void syncOfflineProgress());
