const offlineDatabase = "kinosail-offline-v1";
const chunkSize = 8 * 1024 * 1024;
const offlineIntegrityVersion = 2;
const hasOfflineStorage = "indexedDB" in window && "crypto" in window && Boolean(window.crypto.subtle);
let offlineChannel;
const offlineWorkerPath = "/service-worker.js?v=45";
const offlineWorkerURL = new URL(offlineWorkerPath, location.href).href;
const isExactOfflineWorker = (worker) => worker?.scriptURL === offlineWorkerURL && worker.state === "activated";
const currentOfflineServiceWorker = async () => {
  try {
    const registration = await navigator.serviceWorker.getRegistration();
    return registration && isExactOfflineWorker(registration.active) && isExactOfflineWorker(navigator.serviceWorker.controller) ? registration : undefined;
  } catch (_) { return; }
};
const activateOfflineServiceWorker = async (registration) => {
  let worker = [registration.installing, registration.waiting, registration.active].find((candidate) => candidate?.scriptURL === offlineWorkerURL);
  if (!worker) {
    await new Promise((resolve) => {
      const timeout = setTimeout(resolve, 5000);
      registration.addEventListener("updatefound", () => { clearTimeout(timeout); resolve(); }, {once: true});
    });
    worker = [registration.installing, registration.waiting, registration.active].find((candidate) => candidate?.scriptURL === offlineWorkerURL);
  }
  if (!worker) return;
  if (worker.state !== "activated" && worker.state !== "redundant") await new Promise((resolve) => {
    const timeout = setTimeout(resolve, 5000);
    const settled = () => {
      if (worker.state !== "activated" && worker.state !== "redundant") return;
      clearTimeout(timeout);
      worker.removeEventListener("statechange", settled);
      resolve();
    };
    worker.addEventListener("statechange", settled);
    settled();
  });
  if (worker.state !== "activated" || navigator.serviceWorker.controller?.scriptURL !== offlineWorkerURL) await new Promise((resolve) => {
    const timeout = setTimeout(resolve, 5000);
    navigator.serviceWorker.addEventListener("controllerchange", () => { clearTimeout(timeout); resolve(); }, {once: true});
  });
  return currentOfflineServiceWorker();
};
const offlineServiceWorker = "serviceWorker" in navigator && window.isSecureContext
  ? currentOfflineServiceWorker().then((registration) => registration || navigator.serviceWorker.register(offlineWorkerPath).then(activateOfflineServiceWorker)).catch(currentOfflineServiceWorker)
  : Promise.resolve(undefined);
const offlineContext = () => document.querySelector("#downloads, [data-offline-library]")?.dataset || {};
const offlineMessage = (name, fallback) => offlineContext()[name] || fallback;
const offlineWorkerError = () => offlineMessage("offlineWorkerRequired", "Offline playback is not ready in this browser. Reconnect to the Server and reload Kinosail.");
const offlineRemovalError = () => offlineMessage("offlineRemovalError", "Could not remove this device’s offline data. Try again.");
const requireOfflineServiceWorker = async () => {
  const registration = await offlineServiceWorker;
  if (!registration || !isExactOfflineWorker(navigator.serviceWorker.controller)) throw new Error(offlineWorkerError());
  return registration;
};
const currentOfflineProfile = () => document.body.dataset.viewerProfile || document.querySelector("#downloads")?.dataset.viewerProfile || "";
const playerStorageValue = (key, value) => {
  try { if (value !== undefined) localStorage.setItem(key, value); return localStorage.getItem(key) || ""; } catch (_) { return ""; }
};
const activeOfflineProfile = () => currentOfflineProfile() || playerStorageValue("kinosail.offline-profile");
const offlineJobLockName = (id) => `kinosail-offline:${id}`;
const offlineTransferID = /^[0-9a-f]{32}$/;
const offlineTransferIDOf = (job) => offlineTransferID.test(job?.transferID || "") ? job.transferID : "";
const offlineSourceTransfers = new Map();
const offlineTransfers = new Map();
const offlineNow = () => performance.timeOrigin + performance.now();
const cancelOfflineTransfer = (request) => {
  if (!/^[0-9a-f]{16}$/.test(request?.jobID || "") || !Number.isFinite(request.requestedAt) || request.requestedAt < 0 || request.requestedAt > offlineNow() + 1000) return;
  const owner = offlineTransfers.get(request.jobID);
  if (owner && (request.transferID ? owner.transferID === request.transferID : owner.startedAt <= request.requestedAt)) owner.controller.abort();
};
const offlineDelay = (delay, signal) => new Promise((resolve, reject) => {
  signal?.throwIfAborted();
  const abort = () => { clearTimeout(timer); reject(signal.reason); };
  const timer = setTimeout(() => { signal?.removeEventListener("abort", abort); resolve(); }, delay);
  signal?.addEventListener("abort", abort, {once: true});
});
const withOfflineJobLock = async (id, action, transfer = false, signal) => {
  if (!navigator.locks?.request) throw new Error(transfer ? offlineMessage("offlineStorageUnsupported", "This browser cannot store offline media safely") : offlineRemovalError());
  return navigator.locks.request(offlineJobLockName(id), {mode: "exclusive", ...(signal ? {signal} : {})}, action);
};

const withOfflineTransferSlot = async (action, signal) => {
  const names = ["kinosail-offline-slot:0", "kinosail-offline-slot:1"];
  for (const name of names) {
    signal?.throwIfAborted();
    const result = await navigator.locks.request(name, {ifAvailable: true}, async (lock) => lock ? {value: await action()} : undefined);
    if (result) return result.value;
  }
  let winner;
  const controllers = names.map(() => new AbortController());
  const attempts = names.map((name, index) => navigator.locks.request(name, {signal: signal ? AbortSignal.any([signal, controllers[index].signal]) : controllers[index].signal}, async () => {
    signal?.throwIfAborted();
    if (winner !== undefined) return;
    winner = index;
    controllers.forEach((controller, other) => { if (other !== index) controller.abort(); });
    return {value: await action()};
  }).catch((error) => { if (winner === undefined || winner === index) throw error; }));
  const results = await Promise.all(attempts);
  return results.find((result) => result)?.value;
};

let offlineDatabaseConnection;
const closeOfflineDatabase = () => {
  const connection = offlineDatabaseConnection;
  offlineDatabaseConnection = undefined;
  connection?.then((database) => database.close()).catch(() => {});
};
async function openOfflineDatabase() {
  await requireOfflineServiceWorker();
  if (!offlineDatabaseConnection) {
    const connection = openOfflineDatabaseConnection().then((database) => {
      const reset = () => { if (offlineDatabaseConnection === connection) offlineDatabaseConnection = undefined; database.close(); };
      database.onversionchange = reset;
      database.onclose = reset;
      return database;
    }).catch((error) => { if (offlineDatabaseConnection === connection) offlineDatabaseConnection = undefined; throw error; });
    offlineDatabaseConnection = connection;
  }
  return offlineDatabaseConnection;
}

async function offlineRequest(store, mode, action) {
  const database = await openOfflineDatabase();
  return new Promise((resolve, reject) => {
    const transaction = database.transaction(store, mode, mode === "readwrite" ? {durability: "strict"} : undefined);
    let result;
    let request;
    transaction.oncomplete = () => resolve(result);
    transaction.onabort = () => reject(transaction.error || request?.error || new Error("Offline storage interrupted"));
    try { request = action(transaction.objectStore(store)); }
    catch (error) { transaction.abort(); reject(error); return; }
    request.onsuccess = () => { result = request.result; };
  });
}

const openExistingOfflineDatabase = () => new Promise((resolve, reject) => {
  const request = indexedDB.open(offlineDatabase);
  let missing = false;
  request.onupgradeneeded = () => { missing = true; request.transaction.abort(); };
  request.onsuccess = () => resolve(request.result);
  request.onerror = () => missing && request.error?.name === "AbortError" ? resolve(undefined) : reject(request.error);
});
const readExistingOfflineJob = (database, id) => new Promise((resolve, reject) => {
  const request = database.transaction("jobs", "readonly").objectStore("jobs").get(id);
  request.onsuccess = () => resolve(request.result);
  request.onerror = () => reject(request.error);
});

const getOfflineJob = (id) => offlineRequest("jobs", "readonly", (store) => store.get(id));
const getOfflineJobs = () => offlineRequest("jobs", "readonly", (store) => store.getAll());
const getOfflineChunk = (id) => offlineRequest("chunks", "readonly", (store) => store.get(id));
const saveOfflineJob = (job) => offlineRequest("jobs", "readwrite", (store) => store.put(job));
const saveOfflineChunk = (chunk) => offlineRequest("chunks", "readwrite", (store) => store.put(chunk));

async function getExistingOfflineJob(id) {
  const database = await openExistingOfflineDatabase();
  if (!database) return;
  try {
    return await readExistingOfflineJob(database, id);
  } finally { database.close(); }
}

async function removeOfflineJob(id) {
  const database = await openExistingOfflineDatabase();
  const removeFile = async () => {
    if (!navigator.storage?.getDirectory) return;
    try { await (await navigator.storage.getDirectory()).removeEntry(id); }
    catch (error) { if (error?.name !== "NotFoundError") throw error; }
  };
  if (!database) return removeFile();
  try {
    const job = await readExistingOfflineJob(database, id);
    if (!job || job.storage === "opfs") await removeFile();
    await new Promise((resolve, reject) => {
      const transaction = database.transaction(["jobs", "chunks"], "readwrite");
      const jobs = transaction.objectStore("jobs");
      const chunks = transaction.objectStore("chunks");
      jobs.delete(id);
      if (chunks.indexNames.contains("jobID")) chunks.index("jobID").getAllKeys(id).onsuccess = ({target}) => {
        for (const chunkID of target.result) chunks.delete(chunkID);
      };
      else chunks.openCursor().onsuccess = ({target}) => {
        const cursor = target.result;
        if (!cursor) return;
        if (cursor.value?.jobID === id) cursor.delete();
        cursor.continue();
      };
      transaction.oncomplete = resolve;
      transaction.onerror = () => reject(transaction.error);
      transaction.onabort = () => reject(transaction.error);
    });
  } finally { database.close(); }
}

async function removeOfflineJobSafely(id, explicit = false, afterRemove) {
  if (!/^[0-9a-f]{16}$/.test(id)) throw new Error(offlineRemovalError());
  const sourceTransferID = explicit ? undefined : offlineSourceTransfers.get(id);
  const observed = sourceTransferID === undefined ? await getExistingOfflineJob(id) : {transferID: sourceTransferID};
  const request = {type: "cancel", jobID: id, requestedAt: offlineNow(), transferID: sourceTransferID};
  cancelOfflineTransfer(request);
  offlineChannel?.postMessage(request);
  try {
    return await withOfflineJobLock(id, async () => {
      const current = await getExistingOfflineJob(id);
      const transferID = offlineTransferIDOf(current);
      // A removal from an old playback source must never delete its replacement.
      if (current && (sourceTransferID !== undefined ? transferID !== sourceTransferID :
        Number.isFinite(current.transferStartedAt) ? current.transferStartedAt > request.requestedAt : transferID !== offlineTransferIDOf(observed))) return false;
      await removeOfflineJob(id);
      await afterRemove?.();
      return true;
    });
  } finally { offlineSourceTransfers.delete(id); }
}

async function offlineDigest(data) {
  const digest = new Uint8Array(await crypto.subtle.digest("SHA-256", data));
  return [...digest].map((byte) => byte.toString(16).padStart(2, "0")).join("");
}

function contentDigest(value) {
  const match = value?.match(/^sha-256=:([A-Za-z0-9+/]{43}=):$/);
  if (!match) return "";
  return [...Uint8Array.from(atob(match[1]), (character) => character.charCodeAt(0))].map((byte) => byte.toString(16).padStart(2, "0")).join("");
}

function sha256WorkerScope() {
const sha256Constants = new Uint32Array([
  0x428a2f98, 0x71374491, 0xb5c0fbcf, 0xe9b5dba5, 0x3956c25b, 0x59f111f1, 0x923f82a4, 0xab1c5ed5,
  0xd807aa98, 0x12835b01, 0x243185be, 0x550c7dc3, 0x72be5d74, 0x80deb1fe, 0x9bdc06a7, 0xc19bf174,
  0xe49b69c1, 0xefbe4786, 0x0fc19dc6, 0x240ca1cc, 0x2de92c6f, 0x4a7484aa, 0x5cb0a9dc, 0x76f988da,
  0x983e5152, 0xa831c66d, 0xb00327c8, 0xbf597fc7, 0xc6e00bf3, 0xd5a79147, 0x06ca6351, 0x14292967,
  0x27b70a85, 0x2e1b2138, 0x4d2c6dfc, 0x53380d13, 0x650a7354, 0x766a0abb, 0x81c2c92e, 0x92722c85,
  0xa2bfe8a1, 0xa81a664b, 0xc24b8b70, 0xc76c51a3, 0xd192e819, 0xd6990624, 0xf40e3585, 0x106aa070,
  0x19a4c116, 0x1e376c08, 0x2748774c, 0x34b0bcb5, 0x391c0cb3, 0x4ed8aa4a, 0x5b9cca4f, 0x682e6ff3,
  0x748f82ee, 0x78a5636f, 0x84c87814, 0x8cc70208, 0x90befffa, 0xa4506ceb, 0xbef9a3f7, 0xc67178f2,
]);
const rotateRight = (value, count) => value >>> count | value << 32 - count;

class StreamingSHA256 {
  constructor() {
    this.state = new Uint32Array([0x6a09e667, 0xbb67ae85, 0x3c6ef372, 0xa54ff53a, 0x510e527f, 0x9b05688c, 0x1f83d9ab, 0x5be0cd19]);
    this.buffer = new Uint8Array(64);
    this.bufferLength = 0;
    this.bytes = 0;
    this.words = new Uint32Array(64);
  }

  update(value) {
    const data = value instanceof Uint8Array ? value : new Uint8Array(value);
    this.bytes += data.byteLength;
    let offset = 0;
    while (offset < data.byteLength) {
      const length = Math.min(64 - this.bufferLength, data.byteLength - offset);
      this.buffer.set(data.subarray(offset, offset + length), this.bufferLength);
      this.bufferLength += length;
      offset += length;
      if (this.bufferLength === 64) { this.compress(this.buffer); this.bufferLength = 0; }
    }
    return this;
  }

  compress(block) {
    const words = this.words;
    const view = new DataView(block.buffer, block.byteOffset, block.byteLength);
    for (let index = 0; index < 16; index++) words[index] = view.getUint32(index * 4);
    for (let index = 16; index < 64; index++) {
      const before = words[index - 15];
      const recent = words[index - 2];
      const small0 = rotateRight(before, 7) ^ rotateRight(before, 18) ^ before >>> 3;
      const small1 = rotateRight(recent, 17) ^ rotateRight(recent, 19) ^ recent >>> 10;
      words[index] = words[index - 16] + small0 + words[index - 7] + small1;
    }
    let [a, b, c, d, e, f, g, h] = this.state;
    for (let index = 0; index < 64; index++) {
      const large1 = rotateRight(e, 6) ^ rotateRight(e, 11) ^ rotateRight(e, 25);
      const choice = e & f ^ ~e & g;
      const first = h + large1 + choice + sha256Constants[index] + words[index];
      const large0 = rotateRight(a, 2) ^ rotateRight(a, 13) ^ rotateRight(a, 22);
      const majority = a & b ^ a & c ^ b & c;
      const second = large0 + majority;
      h = g; g = f; f = e; e = d + first; d = c; c = b; b = a; a = first + second;
    }
    for (const [index, value] of [a, b, c, d, e, f, g, h].entries()) this.state[index] += value;
  }

  hex() {
    const final = new Uint8Array(this.bufferLength < 56 ? 64 : 128);
    final.set(this.buffer.subarray(0, this.bufferLength));
    final[this.bufferLength] = 0x80;
    const view = new DataView(final.buffer);
    view.setUint32(final.byteLength - 8, Math.floor(this.bytes / 0x20000000));
    view.setUint32(final.byteLength - 4, this.bytes << 3);
    for (let offset = 0; offset < final.byteLength; offset += 64) this.compress(final.subarray(offset, offset + 64));
    return [...this.state].map((word) => word.toString(16).padStart(8, "0")).join("");
  }
}

let digest = new StreamingSHA256();
self.onmessage = ({data}) => {
  try {
    if (data.type === "update") digest.update(data.value);
    const value = data.type === "hex" ? digest.hex() : undefined;
    self.postMessage({id: data.id, value});
  } catch (error) { self.postMessage({id: data.id, error: error instanceof Error ? error.message : "SHA-256 failed"}); }
};
}

function streamingOfflineDigest() {
  const source = `(${sha256WorkerScope.toString()})()`;
  const url = URL.createObjectURL(new Blob([source], {type: "text/javascript"}));
  const worker = new Worker(url);
  let revoked = false;
  const revoke = () => { if (!revoked) URL.revokeObjectURL(url); revoked = true; };
  let nextID = 0;
  const pending = new Map();
  const fail = (error) => {
    for (const {reject} of pending.values()) reject(error);
    pending.clear();
  };
  worker.onerror = (event) => { revoke(); fail(new Error(event.message || "SHA-256 failed")); };
  worker.onmessage = ({data}) => {
    revoke();
    const request = pending.get(data.id);
    if (!request) return;
    pending.delete(data.id);
    if (data.error) request.reject(new Error(data.error));
    else request.resolve(data.value);
  };
  const request = (type, value) => new Promise((resolve, reject) => {
    const id = nextID++;
    pending.set(id, {resolve, reject});
    worker.postMessage({id, type, value}, value ? [value] : []);
  });
  return {update: (data) => request("update", data), hex: () => request("hex"), close: () => { revoke(); fail(new Error("SHA-256 closed")); worker.terminate(); }};
}
