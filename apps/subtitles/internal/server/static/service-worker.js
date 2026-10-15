const cacheName = "kinosail-shell-v57";
const shell = {"/offline": "text/html", "/static/app.css": "text/css", "/static/manrope.woff2": "font/woff2", "/static/main.kinosail.bundle.js": "text/javascript", "/static/theme.js": "text/javascript", "/static/pwa.js": "text/javascript", "/static/icon.svg": "image/svg+xml", "/static/icon-192.png": "image/png", "/static/icon-512.png": "image/png", "/static/icon-maskable-512.png": "image/png", "/static/apple-touch-icon.png": "image/png", "/static/cinema-backdrop.jpg": "image/jpeg"};
const fetchShell = async (path, expectedType) => {
  const response = await fetch(path, {credentials: path === "/offline" ? "same-origin" : "omit", cache: "reload"});
  const responseURL = new URL(response.url);
  const responseType = response.headers.get("Content-Type")?.split(";", 1)[0];
  if (response.status !== 200 || response.redirected || responseURL.origin !== self.location.origin || responseURL.pathname !== path || responseType !== expectedType) throw new Error(`shell asset unavailable: ${path}`);
  return response;
};
self.addEventListener("install", (event) => event.waitUntil(caches.open(cacheName).then((cache) => Promise.all(Object.entries(shell).map(async ([path, type]) => cache.put(path, await fetchShell(path, type))))).then(() => self.skipWaiting())));
self.addEventListener("activate", (event) => event.waitUntil(Promise.all([caches.keys().then((names) => Promise.all(names.filter((name) => name !== cacheName && name.startsWith("kinosail-")).map((name) => caches.delete(name)))), new Promise((resolve) => { const request = indexedDB.deleteDatabase("kinosail-offline-v1"); request.onsuccess = request.onerror = request.onblocked = resolve; })]).then(() => self.clients.claim())));
self.addEventListener("fetch", (event) => {
  if (event.request.method !== "GET") return;
  const url = new URL(event.request.url);
  if (url.origin !== self.location.origin) return;
  if (event.request.mode === "navigate") event.respondWith(fetch(event.request).catch(() => caches.open(cacheName).then((cache) => cache.match("/offline"))));
  else if (url.pathname in shell && event.request.destination !== "style") event.respondWith(fetch(event.request).catch(() => caches.open(cacheName).then((cache) => cache.match(url.pathname)).then((response) => response || new Response(null, {status: 503}))));
});
