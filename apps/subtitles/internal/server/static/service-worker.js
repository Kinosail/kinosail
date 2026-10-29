const cacheName = "kinosail-shell-v57";
self.addEventListener("install", (event) => event.waitUntil(caches.open(cacheName).then((cache) => Promise.all(Object.entries(shell).map(async ([path, type]) => cache.put(path, await fetchShell(path, type))))).then(() => self.skipWaiting())));
self.addEventListener("activate", (event) => event.waitUntil(Promise.all([caches.keys().then((names) => Promise.all(names.filter((name) => name !== cacheName && name.startsWith("kinosail-")).map((name) => caches.delete(name)))), new Promise((resolve) => { const request = indexedDB.deleteDatabase("kinosail-offline-v1"); request.onsuccess = request.onerror = request.onblocked = resolve; })]).then(() => self.clients.claim())));
self.addEventListener("fetch", (event) => {
  if (event.request.method !== "GET") return;
  const url = new URL(event.request.url);
  if (url.origin !== self.location.origin) return;
  if (event.request.mode === "navigate") event.respondWith(fetch(event.request).catch(() => caches.open(cacheName).then((cache) => cache.match("/offline"))));
  else if (url.pathname in shell && event.request.destination !== "style") event.respondWith(fetch(event.request).catch(() => caches.open(cacheName).then((cache) => cache.match(url.pathname)).then((response) => response || new Response(null, {status: 503}))));
});
