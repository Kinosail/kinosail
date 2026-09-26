import { expect, test } from "@playwright/test";
import { configureTestInstance, createViewer, downloadsSource, firstPlayable, login, loginViewer, newViewerPage, removeViewer } from "./test-instance-helpers";

configureTestInstance();

test("two browser clients synchronize and reconnect in one Watch Together room", async ({ browser, page }) => {
  await login(page);
  const viewerName = `Room Viewer ${Date.now()}`;
  const viewerPassword = "room-viewer-password";
  const viewerID = await createViewer(page, viewerName, viewerPassword);
  const viewer = await newViewerPage(browser, new URL(page.url()).origin);
  const leaderFrames: string[] = [];
  page.on("websocket", (socket) => socket.on("framesent", ({ payload }) => leaderFrames.push(payload.toString())));
  try {
    await page.goto(await firstPlayable(page));
    await page.getByText("Watch together", { exact: true }).click();
    await page.getByRole("button", { name: "Start Watch Together", exact: true }).click();
    await page.getByText("Watch together", { exact: true }).click();
    await expect(page.getByRole("status").filter({ hasText: "Room connected · you lead" })).toBeVisible();
    const invite = await page.getByRole("link", { name: "Invite link", exact: true }).getAttribute("href");
    expect(invite).toMatch(/^\/room\//);

    await loginViewer(viewer, viewerName, viewerPassword);
    await viewer.goto(invite!);
    await viewer.getByText("Watch together", { exact: true }).click();
    await expect(viewer.getByRole("status").filter({ hasText: "Room connected · following leader" })).toBeVisible();
    await page.locator("video").evaluate((media: HTMLVideoElement) => { media.currentTime = 4; media.dispatchEvent(new Event("seeked")); });
    await expect.poll(() => leaderFrames.some((frame) => JSON.parse(frame).action === "seek")).toBeTruthy();
    await expect.poll(() => viewer.locator("video").evaluate((media: HTMLVideoElement) => media.currentTime)).toBeGreaterThan(3.5);

    await viewer.reload();
    await viewer.getByText("Watch together", { exact: true }).click();
    await expect(viewer.getByRole("status").filter({ hasText: "Room connected · following leader" })).toBeVisible();
    const metrics = await page.context().request.get("/api/v1/metrics");
    const body = await metrics.text();
    expect(body).toMatch(/kinosail_watch_room_reconnects_total [1-9]\d*/);
    expect(body).toMatch(/kinosail_watch_room_drift_observations_total [1-9]\d*/);
  } finally {
    await viewer.context().close();
    await removeViewer(page, viewerID);
  }
});

test("offline download stops before transfer when device storage is full", async ({ page }) => {
  await page.addInitScript(() => Object.defineProperties(Object.getPrototypeOf(navigator.storage), {
    estimate: { configurable: true, value: async () => ({ quota: 1, usage: 1 }) },
    persist: { configurable: true, value: async () => true },
  }));
  await login(page);
  await page.goto(await firstPlayable(page));
  await page.getByText("Playback & downloads", { exact: true }).click();
  await page.getByRole("button", { name: "Prepare 720p offline", exact: true }).click();
  await expect(page.getByText("720p · Ready to download", { exact: true })).toBeVisible({ timeout: 30_000 });
  expect(await page.evaluate(() => navigator.storage.estimate())).toEqual({ quota: 1, usage: 1 });
  let fileRequests = 0;
  page.on("request", (request) => { if (/\/api\/v1\/downloads\/[^/]+\/file$/.test(new URL(request.url()).pathname)) fileRequests++; });
  await page.getByRole("button", { name: "Download to this device", exact: true }).click();
  await expect(page.getByText("This device does not have enough storage for this download", { exact: true })).toBeVisible();
  expect(fileRequests).toBe(0);
});

test("offline removal binds immediately, blocks repeat submits, and resumes live updates after BFCache", async ({ page }) => {
  await page.addInitScript(() => {
    Object.defineProperty(navigator, "serviceWorker", {
      configurable: true,
      value: Object.assign(new EventTarget(), { controller: null, getRegistration: () => new Promise(() => {}) }),
    });
    Object.assign(window, { __offlineChannels: 0, __offlineChannelCloses: 0, __offlineEvents: 0, __offlineEventCloses: 0 });
    class OfflineChannel extends EventTarget {
      constructor() { super(); (window as typeof window & { __offlineChannels: number }).__offlineChannels++; }
      close() { (window as typeof window & { __offlineChannelCloses: number }).__offlineChannelCloses++; }
      postMessage() {}
    }
    class OfflineEvents extends EventTarget {
      onerror: ((event: Event) => void) | null = null;
      onopen: ((event: Event) => void) | null = null;
      constructor() { super(); (window as typeof window & { __offlineEvents: number }).__offlineEvents++; queueMicrotask(() => this.onopen?.(new Event("open"))); }
      close() { (window as typeof window & { __offlineEventCloses: number }).__offlineEventCloses++; }
    }
    Object.defineProperties(window, { BroadcastChannel: { configurable: true, value: OfflineChannel }, EventSource: { configurable: true, value: OfflineEvents } });
  });
  await page.route((url) => url.pathname === "/__offline-lifecycle-test", (route) => route.fulfill({
    contentType: "text/html",
    body: '<body><main id="downloads" data-downloads-pending="true"><form method="post" action="/offline-downloads/aaaaaaaaaaaaaaaa/remove"><span hidden data-download-remove-status></span><button type="submit">Confirm remove</button></form></main><script src="/static/downloads.js?v=5"></script></body>',
  }));
  await page.route((url) => url.pathname === "/static/downloads.js", (route) => route.fulfill({ contentType: "text/javascript", body: downloadsSource }));
  let removals = 0;
  await page.route((url) => url.pathname === "/offline-downloads/aaaaaaaaaaaaaaaa/remove", (route) => {
    if (route.request().method() === "POST") removals++;
    return route.fulfill({ contentType: "text/html", body: "<!doctype html><title>Removed</title>" });
  });
  await page.goto("/__offline-lifecycle-test");
  const form = page.locator('form[action="/offline-downloads/aaaaaaaaaaaaaaaa/remove"]');
  await expect(form).toHaveAttribute("data-bound", "true");
  expect(await page.evaluate(() => ({
    channels: (window as typeof window & { __offlineChannels: number }).__offlineChannels,
    events: (window as typeof window & { __offlineEvents: number }).__offlineEvents,
  }))).toEqual({ channels: 1, events: 1 });
  expect(await page.evaluate(() => {
    const hidden = new Event("pagehide");
    const shown = new Event("pageshow");
    Object.defineProperty(hidden, "persisted", { value: true });
    Object.defineProperty(shown, "persisted", { value: true });
    dispatchEvent(hidden);
    dispatchEvent(shown);
    return {
      channelCloses: (window as typeof window & { __offlineChannelCloses: number }).__offlineChannelCloses,
      channels: (window as typeof window & { __offlineChannels: number }).__offlineChannels,
      eventCloses: (window as typeof window & { __offlineEventCloses: number }).__offlineEventCloses,
      events: (window as typeof window & { __offlineEvents: number }).__offlineEvents,
    };
  })).toEqual({ channelCloses: 1, channels: 2, eventCloses: 1, events: 2 });
  expect(await form.evaluate((node) => {
    node.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
    node.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
    return (node.querySelector("button") as HTMLButtonElement).disabled;
  })).toBe(true);
  await expect.poll(() => removals).toBe(1);
});

test("offline surfaces replace pending text when IndexedDB initialization fails", async ({ page }) => {
  await page.addInitScript(() => {
    const worker = { scriptURL: new URL("/service-worker.js?v=54", location.href).href, state: "activated" };
    Object.defineProperties(navigator.serviceWorker, {
      controller: { configurable: true, get: () => worker },
      getRegistration: { configurable: true, value: async () => ({ active: worker }) },
    });
    Object.defineProperty(window, "indexedDB", {
      configurable: true,
      value: {
        open: () => {
          const request = {
            error: new DOMException("Offline storage blocked", "UnknownError"),
            onerror: null as ((event: Event) => void) | null,
            onsuccess: null as ((event: Event) => void) | null,
            onupgradeneeded: null as ((event: Event) => void) | null,
          };
          queueMicrotask(() => request.onerror?.(new Event("error")));
          return request;
        },
      },
    });
  });
  await page.route((url) => url.pathname === "/__offline-storage-unavailable", (route) => route.fulfill({
    contentType: "text/html",
    body: '<body data-viewer-profile="profile"><main id="downloads" data-offline-storage-unsupported="Offline storage is unavailable."><article><button data-download-device data-job-id="aaaaaaaaaaaaaaaa" data-item-id="bbbbbbbbbbbbbbbb" data-title="Movie" data-quality="720p">Download</button><span data-download-device-status>Checking this device…</span></article><section data-offline-library>Checking this device…</section></main><script src="/static/downloads.js?v=5"></script></body>',
  }));
  await page.route((url) => url.pathname === "/static/downloads.js", (route) => route.fulfill({ contentType: "text/javascript", body: downloadsSource }));

  await page.goto("/__offline-storage-unavailable");
  await expect(page.locator("[data-download-device-status]")).toHaveText("Offline storage is unavailable.");
  await expect(page.locator("[data-offline-library]")).toHaveText("Offline storage is unavailable.");
});

test("offline writes fail closed before local changes without Web Locks", async ({ page }) => {
  await page.addInitScript(() => {
    const worker = { scriptURL: new URL("/service-worker.js?v=54", location.href).href, state: "activated" };
    Object.defineProperties(navigator.serviceWorker, {
      controller: { configurable: true, get: () => worker },
      getRegistration: { configurable: true, value: async () => ({ active: worker }) },
    });
    Object.defineProperty(navigator, "locks", { configurable: true, value: undefined });
    Object.assign(window, { __offlineDatabaseOpens: 0, __offlinePersistCalls: 0 });
    const open = indexedDB.open.bind(indexedDB);
    Object.defineProperty(indexedDB, "open", {
      configurable: true,
      value: (...args: Parameters<IDBFactory["open"]>) => {
        (window as typeof window & { __offlineDatabaseOpens: number }).__offlineDatabaseOpens++;
        return open(...args);
      },
    });
    Object.defineProperty(Object.getPrototypeOf(navigator.storage), "persist", {
      configurable: true,
      value: async () => { (window as typeof window & { __offlinePersistCalls: number }).__offlinePersistCalls++; return true; },
    });
  });
  const jobID = "aaaaaaaaaaaaaaaa";
  const itemID = "bbbbbbbbbbbbbbbb";
  const profileID = "profile";
  await page.route((url) => url.pathname === "/__offline-no-locks", (route) => route.fulfill({
    contentType: "text/html",
    body: `<body data-viewer-profile="${profileID}"><main id="downloads" data-viewer-profile="${profileID}" data-offline-storage-unsupported="Offline storage is unavailable."><article data-download-job="${jobID}"><button data-download-device data-job-id="${jobID}" data-item-id="${itemID}" data-title="Movie" data-quality="720p">Download</button><span data-download-device-status></span></article></main><script src="/static/downloads.js?v=5"></script></body>`,
  }));
  await page.route((url) => url.pathname === "/static/downloads.js", (route) => route.fulfill({ contentType: "text/javascript", body: downloadsSource }));
  await page.route((url) => url.pathname === `/api/v1/downloads/${jobID}`, (route) => route.fulfill({
    contentType: "application/json",
    body: JSON.stringify({ id: jobID, itemId: itemID, profileId: profileID, title: "Movie", quality: "720p", state: "ready", readyOffline: true, extension: ".mp4", sha256: "a".repeat(64), size: 1 }),
  }));
  let fileRequests = 0;
  await page.route((url) => url.pathname === `/api/v1/downloads/${jobID}/file`, (route) => { fileRequests++; return route.abort(); });

  await page.goto("/__offline-no-locks");
  const button = page.getByRole("button", { name: "Download", exact: true });
  await expect(page.locator("[data-download-device-status]")).toHaveText("Offline storage is unavailable.");
  await button.click();
  await expect(page.locator("[data-download-device-status]")).toHaveText("Offline storage is unavailable.");
  expect(await page.evaluate(() => ({
    databaseOpens: (window as typeof window & { __offlineDatabaseOpens: number }).__offlineDatabaseOpens,
    persistCalls: (window as typeof window & { __offlinePersistCalls: number }).__offlinePersistCalls,
  }))).toEqual({ databaseOpens: 0, persistCalls: 0 });
  expect(fileRequests).toBe(0);
});
