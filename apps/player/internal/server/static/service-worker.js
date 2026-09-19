const cacheName = "kinosail-shell-v46";
const offlineDatabase = "kinosail-offline-v1";
const chunkSize = 8 * 1024 * 1024;
const retiredOfflinePages = "kinosail-offline-pages-v1";
const imageCachePrefix = "kinosail-private-images-v1-";
const profilePattern = /^[A-Za-z0-9_-]{1,128}$/;
const imageCacheLimit = 512;
let viewerProfile = "";
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
const privateImagePath = (path) => ["/art/", "/backdrop/", "/person/"].some((prefix) => path.startsWith(prefix));
const privateImageCache = () => viewerProfile ? `${imageCachePrefix}${viewerProfile}` : "";
const trimPrivateImages = async (cache) => {
  const keys = await cache.keys();
  await Promise.all(keys.slice(0, Math.max(0, keys.length - imageCacheLimit)).map((key) => cache.delete(key)));
};
const storePrivateImage = async (cache, request, response) => {
  const url = new URL(response.url || request.url);
  const type = response.headers.get("Content-Type")?.split(";", 1)[0];
  if (response.status !== 200 || response.redirected || url.origin !== self.location.origin || !type?.startsWith("image/")) return response;
  await cache.put(request, response.clone());
  await trimPrivateImages(cache);
  return response;
};
const refreshPrivateImage = async (request, cacheNameForProfile, profile) => {
  try {
    const response = await fetch(request, {credentials: "include", cache: "no-cache"});
    if (profile === viewerProfile) await storePrivateImage(await caches.open(cacheNameForProfile), request, response);
  } catch { return; }
};
const servePrivateImage = (event, url) => {
  const profile = viewerProfile;
  const cacheNameForProfile = privateImageCache();
  if (!profile || !privateImagePath(url.pathname)) return;
  event.respondWith(caches.open(cacheNameForProfile).then(async (cache) => {
    const cached = await cache.match(event.request);
    if (cached) {
      event.waitUntil(refreshPrivateImage(event.request, cacheNameForProfile, profile));
      return cached;
    }
    return storePrivateImage(cache, event.request, await fetch(event.request, {credentials: "include", cache: "no-cache"}));
  }));
};
const clearPrivateImageCaches = async (keep) => {
  const names = await caches.keys();
  await Promise.all(names.filter((name) => name.startsWith(imageCachePrefix) && name !== keep).map((name) => caches.delete(name)));
};
registerOfflineLifecycle(cacheName, retiredOfflinePages, populateShell);
self.addEventListener("message", (event) => {
  if (event.data?.type !== "profile") return;
  const next = typeof event.data.profile === "string" && profilePattern.test(event.data.profile) ? event.data.profile : "";
  if (next === viewerProfile) return;
  viewerProfile = next;
  event.waitUntil(clearPrivateImageCaches(privateImageCache()));
});
self.addEventListener("fetch", (event) => {
  const url = offlineRequestURL(event);
  if (!url) return;
  if (url.pathname.startsWith("/offline-media/")) serveOfflineMedia(event, url);
  else if (privateImagePath(url.pathname)) servePrivateImage(event, url);
  else if (event.request.mode === "navigate") {
    const navigation = fetch(event.request);
    event.respondWith(navigation.catch(() => caches.open(cacheName).then((cache) => cache.match("/offline"))));
    event.waitUntil(navigation.then((response) => response.ok ? refreshOffline() : undefined).catch(() => {}));
  }
  else if (url.pathname in shell) event.respondWith(fetch(event.request).catch(() => caches.open(cacheName).then((cache) => cache.match(url.pathname)).then((response) => response || new Response(null, {status: 503}))));
});
