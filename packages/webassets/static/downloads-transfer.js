const offlineItemID = /^[0-9a-f]{16}$/;
const offlineProfileID = /^[A-Za-z0-9_-]{1,256}$/;
const offlineExtension = /^(?:|\.[A-Za-z0-9]{1,15})$/;
const offlineQualities = new Set(["original", "compatible", "1080p", "720p", "480p", "audio"]);
function validOfflineManifest(job, expected) {
  return job && Object.getPrototypeOf(job) === Object.prototype &&
    /^[0-9a-f]{16}$/.test(job.id) && job.id === expected.jobID &&
    typeof job.itemId === "string" && offlineItemID.test(job.itemId) && job.itemId === expected.itemID &&
    typeof job.profileId === "string" && offlineProfileID.test(job.profileId) && job.profileId === expected.profileID &&
    typeof job.title === "string" && job.title.length > 0 && job.title.length <= 512 && job.title === expected.title &&
    typeof job.quality === "string" && offlineQualities.has(job.quality) && job.quality === expected.quality &&
    job.state === "ready" && job.readyOffline === true &&
    typeof job.extension === "string" && offlineExtension.test(job.extension) &&
    /^[0-9a-f]{64}$/.test(job.sha256) && Number.isSafeInteger(job.size) && job.size > 0 && job.size <= chunkSize * 16384;
}

const offlineStatuses = new Map();
function showOfflineStatus(jobID, detail) {
  if (!/^[0-9a-f]{16}$/.test(jobID || "")) return;
  offlineStatuses.set(jobID, detail);
  if (offlineStatuses.size > 50) offlineStatuses.delete(offlineStatuses.keys().next().value);
  document.dispatchEvent(new CustomEvent("kinosail:offline-download", {detail: {jobID, ...detail}}));
  const article = document.querySelector(`[data-download-job="${CSS.escape(jobID)}"]`);
  const status = article?.querySelector("[data-download-device-status]");
  const play = article?.querySelector("[data-download-play]");
  if (play) play.hidden = detail.state !== "ready";
  if (!status) return;
  status.textContent = detail.state === "ready"
    ? detail.playbackVerified ? offlineMessage("offlineReady", "Ready offline on this device") : offlineMessage("offlineVerified", "Saved and verified. Play to check compatibility.")
    : detail.error || `${detail.percent || 0}% ${offlineMessage("offlineStored", "stored on this device")}${detail.estimate ? ` · ${detail.estimate}` : ""}`;
  if (detail.state === "transferring") status.textContent += ` · ${offlineMessage("offlineKeepOpen", "Keep this page open")}`;
}

function notifyOffline(jobID, detail) {
  showOfflineStatus(jobID, detail);
  offlineChannel?.postMessage({jobID, detail});
}

async function offlineFile(jobID) {
  return (await navigator.storage.getDirectory()).getFileHandle(jobID, {create: true});
}

async function readOfflineFile(jobID, offset, length) {
  const file = await (await (await navigator.storage.getDirectory()).getFileHandle(jobID)).getFile();
  return file.slice(offset, offset + length).arrayBuffer();
}

async function offlineFileSize(jobID) {
  try { return (await (await (await navigator.storage.getDirectory()).getFileHandle(jobID)).getFile()).size; }
  catch (error) { if (error?.name === "NotFoundError") return 0; throw error; }
}

async function writeOfflineFile(jobID, offset, data) {
  const writable = await (await offlineFile(jobID)).createWritable({keepExistingData: true});
  try { await writable.write({type: "write", position: offset, data}); await writable.close(); }
  catch (error) { await writable.abort().catch(() => {}); throw error; }
}

async function truncateOfflineFile(jobID, size) {
  const writable = await (await offlineFile(jobID)).createWritable({keepExistingData: true});
  try { await writable.truncate(size); await writable.close(); }
  catch (error) { await writable.abort().catch(() => {}); throw error; }
}

async function ensureOfflineCapacity(size, jobID) {
  const estimate = await navigator.storage?.estimate?.();
  const locks = await navigator.locks.query();
  const held = new Set(locks.held.map((lock) => lock.name));
  const reserved = (await getOfflineJobs()).reduce((total, job) => total + (job.id !== jobID && job.state === "transferring" && held.has(offlineJobLockName(job.id)) && Number.isSafeInteger(job.reservedBytes) && job.reservedBytes > 0 && job.reservedBytes <= chunkSize * 16384 ? job.reservedBytes : 0), 0);
  if (estimate?.quota && estimate.usage + size + reserved > estimate.quota) throw new Error(offlineMessage("offlineCapacityError", "This device does not have enough storage for this download"));
}

async function validStoredOfflineChunk(job, record, offset, length, writer) {
  const stored = await getOfflineChunk(`${job.id}:${offset}`);
  if (!stored || stored.jobID !== job.id || stored.offset !== offset || stored.length !== length || !/^[0-9a-f]{64}$/.test(stored.sha256)) return;
  const data = record.storage === "opfs" ? await (writer ? writer.read(offset, length) : readOfflineFile(job.id, offset, length)).catch(() => undefined) : stored.data;
  if (!data || data.byteLength !== length || await offlineDigest(data) !== stored.sha256) return;
  return data;
}

async function indexedDBOfflineAllocation(job, record) {
  if (record?.storage !== "indexeddb") return 0;
  let bytes = 0;
  for (let offset = 0; offset < job.size; offset += chunkSize) {
    const stored = await getOfflineChunk(`${job.id}:${offset}`);
    const length = stored?.data?.byteLength;
    if (Number.isSafeInteger(length) && length > 0) bytes = Math.min(job.size, bytes + length);
  }
  return bytes;
}

// Retry only transport failures and temporary server responses. Each attempt is
// bounded in memory and has an idle deadline, including while reading the body.
async function fetchOfflineChunk(job, offset, length, signal) {
  for (let attempt = 0; ; attempt++) {
    signal?.throwIfAborted();
    const controller = new AbortController();
    const abort = () => controller.abort();
    signal?.addEventListener("abort", abort, {once: true});
    let timeout;
    const touch = () => { clearTimeout(timeout); timeout = setTimeout(() => controller.abort(), 30000); };
    let response;
    let reader;
    try {
      touch();
      response = await fetch(`/api/v1/downloads/${encodeURIComponent(job.id)}/file`, {
        headers: {Range: `bytes=${offset}-${offset + length - 1}`},
        credentials: "same-origin", cache: "no-store", redirect: "error", signal: controller.signal,
      });
      if ([408, 429, 500, 502, 503, 504].includes(response.status)) throw new TypeError("Temporary download failure");
      const range = response.headers.get("Content-Range")?.match(/^bytes (\d+)-(\d+)\/(\d+)$/);
      if (response.status !== 206 || !range || Number(range[1]) !== offset || Number(range[2]) !== offset + length - 1 || Number(range[3]) !== job.size || !response.body) throw new Error(offlineMessage("offlineVerificationError", "The download could not be verified"));
      const data = new Uint8Array(length);
      reader = response.body.getReader();
      let received = 0;
      for (;;) {
        const {done, value} = await reader.read();
        if (done) break;
        if (received + value.byteLength > length) throw new Error(offlineMessage("offlineVerificationError", "The download could not be verified"));
        data.set(value, received);
        received += value.byteLength;
        if (value.byteLength) touch();
      }
      if (received !== length) throw new TypeError("Download interrupted");
      if (await offlineDigest(data.buffer) !== contentDigest(response.headers.get("Content-Digest"))) throw new Error(offlineMessage("offlineVerificationError", "The download could not be verified"));
      signal?.throwIfAborted();
      return data.buffer;
    } catch (error) {
      signal?.throwIfAborted();
      if (attempt >= 5 || !(error instanceof TypeError || controller.signal.aborted)) throw error;
    } finally {
      clearTimeout(timeout);
      signal?.removeEventListener("abort", abort);
      controller.abort();
      await (reader ? reader.cancel() : response?.body?.cancel())?.catch(() => {});
    }
    notifyOffline(job.id, {state: "waiting", error: offlineMessage("offlineRetrying", "Connection interrupted. Retrying automatically…")});
    const retryAfter = response?.headers.get("Retry-After");
    const serverDelay = /^\d{1,6}$/.test(retryAfter || "") ? Number(retryAfter) * 1000 : Date.parse(retryAfter || "") - Date.now();
    if (serverDelay > 60000) throw new Error(offlineMessage("offlineBusy", "The Server is busy. Your progress is saved; try again later."));
    const delay = Math.min(60000, Math.max(1000 * 2 ** attempt + Math.random() * 500, Number.isFinite(serverDelay) ? serverDelay : 0));
    await offlineDelay(delay, signal);
  }
}

async function transferOfflineJob(job, expected, owner) {
  if (!hasOfflineStorage || !("Worker" in window)) throw new Error(offlineMessage("offlineStorageUnsupported", "This browser cannot store offline media safely"));
  if (!validOfflineManifest(job, expected)) throw new Error(offlineMessage("offlineVerificationError", "The download could not be verified"));
  const controller = new AbortController();
  const signal = controller.signal;
  const abort = () => controller.abort();
  owner?.controller.signal.throwIfAborted();
  owner?.controller.signal.addEventListener("abort", abort, {once: true});
  let record, writer, digest, pending, admitted = false;
  try {
    await requireOfflineServiceWorker();
    signal.throwIfAborted();
    record = await getOfflineJob(job.id);
    if (record && (record.integrityVersion !== offlineIntegrityVersion || record.sha256 !== job.sha256 || record.size !== job.size || record.itemID !== job.itemId || record.profileID !== job.profileId || !["opfs", "indexeddb"].includes(record.storage))) {
      await removeOfflineJob(job.id);
      record = undefined;
    }
    const previousTransferID = offlineTransferIDOf(record);
    const requiredCapacity = record?.storage === "opfs"
      ? Math.max(0, job.size - await offlineFileSize(job.id))
      : Math.max(0, job.size - await indexedDBOfflineAllocation(job, record));
    signal.throwIfAborted();
    if (!record || record.storage === "opfs") {
      try { writer = await openOfflineWriter(job.id, job.size); }
      catch (error) { if (record) throw error; }
    }
    record = record || {id: job.id, integrityVersion: offlineIntegrityVersion, profileID: job.profileId, itemID: job.itemId, title: job.title, quality: job.quality, extension: job.extension || "", sha256: job.sha256, size: job.size, storage: writer ? "opfs" : "indexeddb"};
    signal.throwIfAborted();
    if (!record.progressBaseline) record.progressBaseline = await downloadProgressBaseline(job.itemId, job.profileId, signal);
    record.extension = job.extension || record.extension || "";
    record.transferID = owner?.transferID || [...crypto.getRandomValues(new Uint8Array(16))].map((byte) => byte.toString(16).padStart(2, "0")).join("");
    record.transferStartedAt = owner?.startedAt || offlineNow();
    record.state = "transferring";
    record.readyOffline = false;
    delete record.playbackAgent;
    record.error = "";
    record.bytes = 0;
    record.reservedBytes = requiredCapacity;
    signal.throwIfAborted();
    await navigator.locks.request("kinosail-offline-capacity", {signal}, async () => {
      signal.throwIfAborted();
      await ensureOfflineCapacity(requiredCapacity, job.id);
      signal.throwIfAborted();
      await beginOfflineTransfer(record, previousTransferID);
      admitted = true;
    });
    digest = streamingOfflineDigest();
    let networkBytes = 0, networkStart = 0;
    const prepare = async (offset) => {
      signal.throwIfAborted();
      const length = Math.min(chunkSize, job.size - offset);
      let data = await validStoredOfflineChunk(job, record, offset, length, writer);
      const stored = Boolean(data);
      if (!stored) {
        if (!networkStart) networkStart = performance.now();
        data = await fetchOfflineChunk(job, offset, length, signal);
        networkBytes += length;
      }
      return {data, stored, length, sha256: stored ? "" : await offlineDigest(data)};
    };
    const prefetch = (offset) => { const task = prepare(offset); task.catch(() => {}); return task; };
    pending = prefetch(0);
    for (let offset = 0; offset < job.size; offset += chunkSize) {
      const {data, stored, length, sha256} = await pending;
      signal.throwIfAborted();
      pending = offset + length < job.size ? prefetch(offset + length) : undefined;
      if (!stored) {
        const chunk = {id: `${job.id}:${offset}`, jobID: job.id, offset, length, sha256};
        if (record.storage === "opfs") {
          if (writer) await writer.write(offset, data);
          else await writeOfflineFile(job.id, offset, data);
          signal.throwIfAborted();
          await saveOfflineChunk(chunk);
        } else await saveOfflineChunk({...chunk, data});
        record.reservedBytes = Math.max(0, record.reservedBytes - length);
      }
      await digest.update(data);
      signal.throwIfAborted();
      record.bytes += length;
      await saveOfflineJob(record);
      const elapsed = performance.now() - networkStart;
      const speed = networkBytes * 1000 / elapsed;
      const minutes = Math.max(1, Math.ceil((job.size - record.bytes) / speed / 60));
      const estimate = networkBytes > 0 && elapsed >= 2000 && record.bytes < job.size
        ? `${(speed / 1024 ** 2).toFixed(1)} MB/s · ~${minutes} min` : "";
      notifyOffline(job.id, {state: "transferring", percent: Math.min(99, Math.floor(record.bytes / job.size * 100)), estimate});
    }
    if (record.storage === "opfs") {
      if (writer) { await writer.truncate(); await writer.close(); writer = undefined; }
      else await truncateOfflineFile(job.id, job.size);
    }
    signal.throwIfAborted();
    if (await digest.hex() !== job.sha256) {
      record.integrityVersion = 0;
      await removeOfflineJob(job.id).catch(() => {});
      throw new Error(offlineMessage("offlineVerificationError", "The download could not be verified"));
    }
    signal.throwIfAborted();
    delete record.reservedBytes;
    record.state = "ready";
    record.readyOffline = true;
    await saveOfflineJob(record);
    notifyOffline(job.id, {state: "ready"});
    return record;
  } catch (error) {
    if (record && admitted) {
      record.state = "needs_attention";
      record.readyOffline = false;
      record.error = signal.aborted ? offlineMessage("offlinePaused", "Download interrupted. Resume to continue where it stopped.") : error instanceof Error ? error.message : offlineMessage("offlineVerificationError", "The download could not be verified");
      await saveOfflineJob(record);
      notifyOffline(job.id, {state: record.state, error: record.error});
    }
    throw error;
  } finally {
    controller.abort();
    owner?.controller.signal.removeEventListener("abort", abort);
    await pending?.catch(() => {});
    digest?.close();
    await writer?.close();
  }
}
