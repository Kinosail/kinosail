const offlineDatabase = "kinosail-offline-v1";
const chunkSize = 8 * 1024 * 1024;
const offlineIntegrityVersion = 2;
const hasOfflineStorage = "indexedDB" in window && "crypto" in window && Boolean(window.crypto.subtle);
let offlineChannel;
const offlineWorkerPath = "/service-worker.js?v=51";
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
const currentOfflineProfile = () => document.body.dataset.viewerProfile || document.querySelector("#downloads")?.dataset.viewerProfile || document.querySelector("[data-nav-profile]")?.dataset.navProfile || "";
const playerStorageValue = (key, value) => {
  try { if (value !== undefined) localStorage.setItem(key, value); return localStorage.getItem(key) || ""; } catch (_) { return ""; }
};
const activeOfflineProfile = () => window.kinosailOfflineIdentity?.current() ?? playerStorageValue("kinosail.offline-profile");
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

window.addEventListener("kinosail:offline-profile", () => {
  for (const transfer of offlineTransfers.values()) transfer.controller.abort();
  for (const media of document.querySelectorAll('[data-offline-library] video, [data-offline-library] audio')) {
    media.pause(); media.removeAttribute("src"); media.load();
  }
  renderOfflineLibrary().catch(() => {});
});
