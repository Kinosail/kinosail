const CACHE = "kinosail-dashboard-shell-v11";
const SHELL = ["/", "/manifest.webmanifest", "/static/base.css", "/static/manrope.woff2?v=1", "/static/last-light.css?v=electric-1", "/static/appearance.js?v=electric-1", "/static/dashboard.css", "/static/app.js", "/static/api.js", "/static/dom.js", "/static/drag.js", "/static/editor.js", "/static/icons.svg", "/static/icon.svg?v=5", "/static/icon-192.png?v=5", "/static/icon-512.png?v=5", "/static/icon-maskable-512.png?v=5", "/static/apple-touch-icon.png?v=5"];
self.addEventListener("install", event => event.waitUntil(caches.open(CACHE).then(cache => cache.addAll(SHELL))));
self.addEventListener("activate", event => event.waitUntil(caches.keys().then(keys => Promise.all(keys.filter(key => key !== CACHE).map(key => caches.delete(key))))));
self.addEventListener("fetch", event => {
  const url = new URL(event.request.url);
  if (event.request.method !== "GET" || url.origin !== location.origin) return;
  if (event.request.mode === "navigate") {
    event.respondWith(fetch(event.request).then(response => { const copy = response.clone(); caches.open(CACHE).then(cache => cache.put("/", copy)); return response; }).catch(() => caches.match("/")));
    return;
  }
  if (!url.pathname.startsWith("/static/") && url.pathname !== "/manifest.webmanifest") return;
  event.respondWith(fetch(event.request).then(response => { const copy = response.clone(); caches.open(CACHE).then(cache => cache.put(event.request, copy)); return response; }).catch(() => caches.match(event.request)));
});
