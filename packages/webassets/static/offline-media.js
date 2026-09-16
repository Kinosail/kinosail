const offlineRequestURL = (event) => {
  if (event.request.method !== "GET") return;
  const url = new URL(event.request.url);
  return url.origin === self.location.origin ? url : undefined;
};

const registerOfflineLifecycle = (cacheName, retiredCache, populate) => {
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
        const chunkOffset = offset;
        const data = next || await verifiedOfflineChunk(job, file, chunkOffset);
        next = undefined;
        if (cancelled) return;
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
