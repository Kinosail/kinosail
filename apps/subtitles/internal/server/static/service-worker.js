const cacheName = "kinosail-shell-v52";
const chunkSize = 8 * 1024 * 1024;
const offlineDatabase = "kinosail-offline-v1";
const retiredOfflinePages = "kinosail-offline-pages-v1";
registerOfflineLifecycle(cacheName, retiredOfflinePages, populateShell);
self.addEventListener("fetch", (event) => {
  const url = offlineRequestURL(event);
  if (!url) return;
  if (url.pathname.startsWith("/offline-media/")) serveOfflineMedia(event, url);
  else if (event.request.mode === "navigate") event.respondWith(fetch(event.request).catch(() => caches.open(cacheName).then((cache) => cache.match("/offline"))));
  else if (url.pathname in shell && event.request.destination !== "style") event.respondWith(fetch(event.request).catch(() => caches.open(cacheName).then((cache) => cache.match(url.pathname)).then((response) => response || new Response(null, {status: 503}))));
});
