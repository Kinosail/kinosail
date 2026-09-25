const shell = {"/offline": "text/html", "/static/app.css": "text/css", "/static/manrope.woff2": "font/woff2", "/static/main.kinosail.bundle.js": "text/javascript", "/static/theme.js": "text/javascript", "/static/pwa.js": "text/javascript", "/static/player.js": "text/javascript", "/static/downloads.js": "text/javascript", "/static/icon.svg": "image/svg+xml", "/static/icon-192.png": "image/png", "/static/icon-512.png": "image/png", "/static/icon-maskable-512.png": "image/png", "/static/apple-touch-icon.png": "image/png", "/static/cinema-backdrop.jpg": "image/jpeg"};
const fetchShell = async (path, expectedType) => {
  const response = await fetch(path, {credentials: path === "/offline" ? "same-origin" : "omit", cache: "reload"});
  const responseURL = new URL(response.url);
  const responseType = response.headers.get("Content-Type")?.split(";", 1)[0];
  if (response.status !== 200 || response.redirected || responseURL.origin !== self.location.origin || responseURL.pathname !== path || responseType !== expectedType) throw new Error(`shell asset unavailable: ${path}`);
  return response;
};
const populateShell = async (cache) => Promise.all(Object.entries(shell).map(async ([path, expectedType]) => cache.put(path, await fetchShell(path, expectedType))));

let selectedOfflineIdentity;
let offlineIdentityReady = true;
let offlineIdentityWrites = Promise.resolve();
const selectedProfile = async () => {
  while (!offlineIdentityReady) {
    const pending = offlineIdentityWrites;
    try { await pending; } catch { return ""; }
    if (pending === offlineIdentityWrites && !offlineIdentityReady) return "";
  }
  const stored = selectedOfflineIdentity ?? await offlineProfileState();
  return offlineIdentityReady ? (selectedOfflineIdentity ?? stored)?.profile || "" : "";
};
const offlineRequestURL = (event) => {
  if (event.request.method !== "GET") return;
  const url = new URL(event.request.url);
  return url.origin === self.location.origin ? url : undefined;
};

const registerOfflineLifecycle = (cacheName, retiredCache, populate, changed = () => {}) => {
  const identify = (profile, revision) => {
    const implicitLogout = profile === "" && revision === undefined;
    revision ??= Math.max(Date.now(), (selectedOfflineIdentity?.revision || 0) + 1);
    if (!Number.isSafeInteger(revision) || revision < 0 || revision > Date.now() + 60000 || revision < (selectedOfflineIdentity?.revision || 0)) return Promise.resolve();
    const previous = selectedOfflineIdentity;
    selectedOfflineIdentity = {profile, revision};
    offlineIdentityReady = profile === "";
    const broadcast = async (identity) => {
      const clients = await self.clients.matchAll({type: "window", includeUncontrolled: true});
      if (selectedOfflineIdentity.revision === identity.revision) for (const client of clients) client.postMessage({type: "offline-profile", ...identity});
    };
    const cleared = Promise.all([...(previous?.profile === profile ? [] : [changed("")]), ...(profile === "" ? [broadcast(selectedOfflineIdentity)] : [])]);
    offlineIdentityWrites = offlineIdentityWrites.catch(() => {}).then(async () => {
      const stored = await offlineProfileState();
      if (implicitLogout && Number.isSafeInteger(stored?.revision)) revision = Math.max(revision, stored.revision + 1);
      const next = stored && /^[A-Za-z0-9_-]{0,128}$/.test(stored.profile) && Number.isSafeInteger(stored.revision) && stored.revision > revision && stored.revision <= Date.now() + 60000 ? stored : {profile, revision};
      if (next !== stored) await offlineProfileState(next);
      if (selectedOfflineIdentity.revision <= next.revision) {
        selectedOfflineIdentity = next;
        offlineIdentityReady = true;
        await changed(next.profile);
        await broadcast(next);
      }
    });
    return Promise.all([cleared, offlineIdentityWrites]);
  };
  self.addEventListener("message", (event) => {
    if (event.origin !== self.location.origin || event.source?.type !== "window") return;
    try { if (new URL(event.source.url).origin !== self.location.origin) return; } catch { return; }
    const message = event.data;
    if (!message || typeof message !== "object" || Array.isArray(message) || Object.keys(message).some((key) => !["type", "profile", "revision"].includes(key))) return;
    if ("revision" in message && !Number.isSafeInteger(message.revision)) return;
    if (message.type === "profile" && typeof message.profile === "string" && /^[A-Za-z0-9_-]{1,128}$/.test(message.profile)) event.waitUntil(identify(message.profile, message.revision));
    if (message.type === "logout" && (message.profile === undefined || message.profile === "")) event.waitUntil(identify("", message.revision));
  });
  self.addEventListener("fetch", (event) => {
    const url = new URL(event.request.url);
    if (url.origin !== self.location.origin || !(event.request.method === "POST" && url.pathname === "/logout" || event.request.method === "DELETE" && url.pathname === "/api/v1/session")) return;
    event.waitUntil(identify("").catch(() => {}));
    event.respondWith(fetch(event.request));
  });
  self.addEventListener("install", (event) => event.waitUntil(caches.open(cacheName).then(populate).then(() => self.skipWaiting())));
  self.addEventListener("activate", (event) => event.waitUntil(caches.keys().then((names) => Promise.all(names.filter((name) => name === retiredCache || name.startsWith("kinosail-shell-") && name !== cacheName).map((name) => caches.delete(name)))).then(() => self.clients.claim())));
};

const openDatabase = openOfflineDatabaseConnection;
const readStore = async (store, action) => {
  const database = await openDatabase();
  return new Promise((resolve, reject) => {
    let transaction;
    let request;
    try {
      transaction = database.transaction(store, "readonly");
      request = action(transaction.objectStore(store));
    } catch (error) { database.close(); reject(error); return; }
    transaction.oncomplete = () => { database.close(); resolve(request.result); };
    transaction.onerror = () => { database.close(); reject(transaction.error || request.error); };
    transaction.onabort = () => { database.close(); reject(transaction.error || request.error); };
  });
};
const digest = async (data) => [...new Uint8Array(await crypto.subtle.digest("SHA-256", data))].map((byte) => byte.toString(16).padStart(2, "0")).join("");
const readOfflineChunk = async (jobID, offset) => {
  const database = await openDatabase();
  return new Promise((resolve, reject) => {
    let transaction;
    let count;
    let request;
    try {
      transaction = database.transaction("chunks", "readonly");
      const index = transaction.objectStore("chunks").index("jobRange");
      count = index.count([jobID, offset]);
      request = index.get([jobID, offset]);
    } catch (error) { database.close(); reject(error); return; }
    transaction.oncomplete = () => { database.close(); resolve(count.result === 1 ? request.result : undefined); };
    transaction.onerror = () => { database.close(); reject(transaction.error || request.error || count.error); };
    transaction.onabort = () => { database.close(); reject(transaction.error || request.error || count.error); };
  });
};
const verifiedOfflineChunk = async (job, file, offset) => {
  const chunk = await readOfflineChunk(job.id, offset);
  const length = Math.min(chunkSize, job.size - offset);
  if (!chunk || chunk.id !== `${job.id}:${offset}` || chunk.jobID !== job.id || chunk.offset !== offset || chunk.length !== length || !/^[0-9a-f]{64}$/.test(chunk.sha256)) throw new Error("offline chunk metadata is invalid");
  const data = file ? await file.slice(offset, offset + length).arrayBuffer() : chunk.data;
  if (!(data instanceof ArrayBuffer) || data.byteLength !== length || await digest(data) !== chunk.sha256) throw new Error("offline chunk integrity failed");
  return data;
};
const verifiedOfflineBody = async (job, start, end) => {
  const file = job.storage === "opfs" ? await (await (await navigator.storage.getDirectory()).getFileHandle(job.id)).getFile() : undefined;
  if (file && file.size !== job.size) return;
  let offset = Math.floor(start / chunkSize) * chunkSize;
  let next = await verifiedOfflineChunk(job, file, offset);
  let cancelled = false;
  return new ReadableStream({
    async pull(controller) {
      try {
		if (await selectedProfile() !== job.profileID) throw new Error("Offline profile changed");
        const chunkOffset = offset;
        const data = next || await verifiedOfflineChunk(job, file, chunkOffset);
        next = undefined;
        if (cancelled) return;
        if (await selectedProfile() !== job.profileID) throw new Error("Offline profile changed");
        const from = Math.max(0, start - chunkOffset);
        const to = Math.min(data.byteLength, end + 1 - chunkOffset);
        offset += data.byteLength;
        if (from < to) controller.enqueue(new Uint8Array(data, from, to - from));
        if (offset > end) controller.close();
      } catch (error) { if (!cancelled) controller.error(error); }
    },
    cancel() { cancelled = true; next = undefined; },
  }, {highWaterMark: 0});
};
const offlineResponse = async (request, profileID, jobID) => {
  if (!/^[A-Za-z0-9_-]{1,256}$/.test(profileID || "") || !/^[0-9a-f]{16}$/.test(jobID || "")) return;
  if (await selectedProfile() !== profileID) return;
  const job = await readStore("jobs", (store) => store.get(jobID));
  if (!job || job.id !== jobID || job.integrityVersion !== 2 || job.profileID !== profileID || job.state !== "ready" || !job.readyOffline || !["indexeddb", "opfs"].includes(job.storage) || !Number.isSafeInteger(job.size) || job.size <= 0 || job.size > chunkSize * 16384 || !/^[0-9a-f]{64}$/i.test(job.sha256)) return;
  const types = {".flac": "audio/flac", ".m4a": "audio/mp4", ".mp3": "audio/mpeg", ".ogg": "audio/ogg", ".opus": "audio/ogg", ".wav": "audio/wav", ".mkv": "video/x-matroska", ".webm": "video/webm"};
  const type = types[job.extension?.toLowerCase()] || (job.quality === "audio" ? "audio/mp4" : "video/mp4");
  const rangeHeader = request.headers.get("Range");
  const headers = {"Accept-Ranges": "bytes", "Cache-Control": "private, no-store", "Content-Type": type, "ETag": `"${job.sha256}"`};
  const range = rangeHeader?.match(/^bytes=(\d*)-(\d*)$/);
  const unsatisfied = () => new Response(null, {status: 416, headers: {...headers, "Content-Length": "0", "Content-Range": `bytes */${job.size}`}});
  if (rangeHeader && (!range || !range[1] && !range[2])) return unsatisfied();
  let start = range?.[1] ? Number(range[1]) : 0;
  let end = range?.[2] ? Number(range[2]) : job.size - 1;
  if (range && !range[1]) {
    const suffix = Number(range[2]);
    if (!Number.isSafeInteger(suffix) || suffix <= 0) return unsatisfied();
    start = Math.max(0, job.size - suffix);
    end = job.size - 1;
  }
  if (!Number.isSafeInteger(start) || !Number.isSafeInteger(end) || start >= job.size || end < start) return unsatisfied();
  const finalEnd = Math.min(end, job.size - 1);
  const body = await verifiedOfflineBody(job, start, finalEnd);
  if (!body) return;
  headers["Content-Length"] = String(finalEnd - start + 1);
  if (!rangeHeader) return new Response(body, {headers});
  headers["Content-Range"] = `bytes ${start}-${finalEnd}/${job.size}`;
  return new Response(body, {status: 206, headers});
};
const serveOfflineMedia = (event, url) => {
    const media = url.pathname.slice("/offline-media/".length).split("/");
    let decoded;
    try { decoded = media.length === 2 ? media.map(decodeURIComponent) : []; }
    catch { event.respondWith(Promise.resolve(new Response(null, {status: 404}))); return; }
    event.respondWith((decoded.length === 2 ? offlineResponse(event.request, decoded[0], decoded[1]) : Promise.resolve()).then((response) => response || fetch(event.request)).catch(() => new Response(null, {status: 404})));
};
