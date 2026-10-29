const populateShell = async (cache) => Promise.all(Object.entries(shell).map(async ([path, expectedType]) => cache.put(path, await fetchShell(path, expectedType))));

let selectedOfflineIdentity;
let offlineIdentityWrites = Promise.resolve();
const offlineRequestURL = (event) => {
  if (event.request.method !== "GET") return;
  const url = new URL(event.request.url);
  return url.origin === self.location.origin ? url : undefined;
};

const registerOfflineLifecycle = (cacheName, retiredCache, populate, changed = () => {}, retire = async () => {}) => {
  const identify = (profile, revision) => {
    const implicitLogout = profile === "" && revision === undefined;
    revision ??= Math.max(Date.now(), (selectedOfflineIdentity?.revision || 0) + 1);
    if (!Number.isSafeInteger(revision) || revision < 0 || revision > Date.now() + 60000 || revision < (selectedOfflineIdentity?.revision || 0)) return Promise.resolve();
    const previous = selectedOfflineIdentity;
    selectedOfflineIdentity = {profile, revision};
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
  self.addEventListener("activate", (event) => event.waitUntil(Promise.all([retire(), caches.keys().then((names) => Promise.all(names.filter((name) => name === retiredCache || name.startsWith("kinosail-shell-") && name !== cacheName).map((name) => caches.delete(name))))]).then(() => self.clients.claim())));
};

