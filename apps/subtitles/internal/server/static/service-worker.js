const cacheName = "kinosail-shell-v45";
const chunkSize = 8 * 1024 * 1024;
const offlineDatabase = "kinosail-offline-v1";
const retiredOfflinePages = "kinosail-offline-pages-v1";
const shell = ["/offline", "/static/app.css", "/static/manrope.woff2?v=1", "/static/theme.js", "/static/pwa.js", "/static/player.js", "/static/downloads.js", "/static/icon.svg", "/static/icon-192.png", "/static/icon-512.png", "/static/icon-maskable-512.png", "/static/apple-touch-icon.png", "/static/cinema-backdrop.jpg"];
const populateShell = async (cache) => {
  await Promise.all(shell.map(async (path) => {
    try {
      const response = await fetch(path, {credentials: "same-origin"});
      if (response.ok) await cache.put(path, response);
    } catch {}
  }));
};
registerOfflineLifecycle(cacheName, retiredOfflinePages, populateShell);
self.addEventListener("fetch", (event) => {
  const url = offlineRequestURL(event);
  if (!url) return;
  if (url.pathname.startsWith("/offline-media/")) serveOfflineMedia(event, url);
  else if (event.request.mode === "navigate") event.respondWith(fetch(event.request).catch(() => caches.match("/offline")));
  else if (shell.includes(url.pathname) && event.request.destination !== "style") event.respondWith(fetch(event.request).catch(() => caches.match(url.pathname).then((response) => response || new Response(null, {status: 503}))));
});
