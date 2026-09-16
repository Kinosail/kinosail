const cacheName = "kinosail-shell-v45";
const offlineDatabase = "kinosail-offline-v1";
const chunkSize = 8 * 1024 * 1024;
const retiredOfflinePages = "kinosail-offline-pages-v1";
const shell = {"/offline": "text/html", "/static/app.css": "text/css", "/static/manrope.woff2": "font/woff2", "/static/main.kinosail.bundle.js": "text/javascript", "/static/theme.js": "text/javascript", "/static/pwa.js": "text/javascript", "/static/player.js": "text/javascript", "/static/downloads.js": "text/javascript", "/static/icon.svg": "image/svg+xml", "/static/icon-192.png": "image/png", "/static/icon-512.png": "image/png", "/static/icon-maskable-512.png": "image/png", "/static/apple-touch-icon.png": "image/png", "/static/cinema-backdrop.jpg": "image/jpeg"};
const fetchShell = async (path, expectedType) => {
  const response = await fetch(path, {credentials: path === "/offline" ? "same-origin" : "omit", cache: "reload"});
  const responseURL = new URL(response.url);
  const responseType = response.headers.get("Content-Type")?.split(";", 1)[0];
  if (response.status !== 200 || response.redirected || responseURL.origin !== self.location.origin || responseURL.pathname !== path || responseType !== expectedType) throw new Error(`shell asset unavailable: ${path}`);
  return response;
};
const populateShell = async (cache) => Promise.all(Object.entries(shell).map(async ([path, expectedType]) => cache.put(path, await fetchShell(path, expectedType))));
const refreshOffline = () => caches.open(cacheName).then(async (cache) => cache.put("/offline", await fetchShell("/offline", shell["/offline"])));
registerOfflineLifecycle(cacheName, retiredOfflinePages, populateShell);
self.addEventListener("fetch", (event) => {
  const url = offlineRequestURL(event);
  if (!url) return;
  if (url.pathname.startsWith("/offline-media/")) serveOfflineMedia(event, url);
  else if (event.request.mode === "navigate") {
    const navigation = fetch(event.request);
    event.respondWith(navigation.catch(() => caches.open(cacheName).then((cache) => cache.match("/offline"))));
    event.waitUntil(navigation.then((response) => response.ok ? refreshOffline() : undefined).catch(() => {}));
  }
  else if (url.pathname in shell) event.respondWith(fetch(event.request).catch(() => caches.open(cacheName).then((cache) => cache.match(url.pathname)).then((response) => response || new Response(null, {status: 503}))));
});
