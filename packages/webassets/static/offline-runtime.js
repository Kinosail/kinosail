const openOfflineDatabaseConnection = () => new Promise((resolve, reject) => {
  const request = indexedDB.open(offlineDatabase, 3);
  request.onupgradeneeded = () => {
    const database = request.result;
    if (!database.objectStoreNames.contains("jobs")) database.createObjectStore("jobs", {keyPath: "id"});
    const chunks = database.objectStoreNames.contains("chunks") ? request.transaction.objectStore("chunks") : database.createObjectStore("chunks", {keyPath: "id"});
    if (!chunks.indexNames.contains("jobID")) chunks.createIndex("jobID", "jobID");
    if (!chunks.indexNames.contains("jobRange")) chunks.createIndex("jobRange", ["jobID", "offset"]);
  };
  request.onsuccess = () => resolve(request.result);
  request.onerror = () => reject(request.error);
});

const offlineRequestURL = (event) => {
  if (event.request.method !== "GET") return;
  const url = new URL(event.request.url);
  return url.origin === self.location.origin ? url : undefined;
};

const registerOfflineLifecycle = (cacheName, retiredCache, populate) => {
  self.addEventListener("install", (event) => event.waitUntil(caches.open(cacheName).then(populate).then(() => self.skipWaiting())));
  self.addEventListener("activate", (event) => event.waitUntil(caches.keys().then((names) => Promise.all(names.filter((name) => name === retiredCache || name.startsWith("kinosail-shell-") && name !== cacheName).map((name) => caches.delete(name)))).then(() => self.clients.claim())));
};
