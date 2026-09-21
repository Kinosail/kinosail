import { expect, test as base, type Video } from "@playwright/test";
import { createServer } from "node:http";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import type { AddressInfo } from "node:net";
import { downloadsSource, readStaticSource } from "./static-sources";

const test = base.extend({
  // A real disk cache is required; ephemeral WebKit contexts have none.
  context: async ({ playwright, browserName, contextOptions, launchOptions, channel, headless, video }, use, testInfo) => {
    const directory = await mkdtemp(join(tmpdir(), "kinosail-worker-upgrade-"));
    const mode = typeof video === "string" ? video : video.mode;
    const record = mode === "on" || mode === "retain-on-failure" || mode === "on-first-retry" && testInfo.retry === 1;
    const videos: Video[] = [];
    try {
      const context = await playwright[browserName].launchPersistentContext(directory, {
        ...launchOptions, ...contextOptions, channel, headless,
        recordVideo: record ? {dir: directory, ...(typeof video === "object" ? {size: video.size} : {})} : undefined,
      });
      context.on("page", page => { const capture = page.video(); if (capture) videos.push(capture); });
      try { await use(context); } finally {
        try {
          await Promise.all(context.pages().map(page => page.close()));
          // Save videos before closing the persistent context's browser connection.
          if (record && (mode !== "retain-on-failure" || testInfo.status !== testInfo.expectedStatus)) {
            for (const [index, capture] of videos.entries()) {
              const path = testInfo.outputPath(`video-${index}.webm`);
              await capture.saveAs(path);
              await testInfo.attach("video", {path, contentType: "video/webm"});
            }
          }
        } finally { await context.close(); }
      }
    } finally { await rm(directory, {recursive: true, force: true}); }
  },
});

type WorkerWindow = Window & { checkWorker: () => Promise<string>; identifiedProfile?: { type: string; profile: string; worker: string } };

const currentDownloads = downloadsSource;
const currentPWA = await readStaticSource(["../../../packages/webassets/static/offline-identity.js", "../../../packages/webassets/static/pwa.js"]);
const currentWorker = currentDownloads.match(/const offlineWorkerPath = "([^"]+)"/)![1];
const oldWorker = "/service-worker.js?v=38";
const oldDownloads = currentDownloads.replaceAll(currentWorker, oldWorker);
const oldPWA = currentPWA.replaceAll(currentWorker, oldWorker);
const exposeWorker = '\nwindow.checkWorker = async () => { try { await requireOfflineServiceWorker(); return "ready"; } catch (error) { return error.message; } };';

for (const app of [
  { name: "player", downloads: "18", navigation: "20" },
  { name: "subtitles", downloads: "5", navigation: "8" },
]) {
  test(`${app.name} refreshes both immutable scripts when upgrading its offline worker`, async ({ page }) => {
    const [offline, locale, workerSource] = await Promise.all([
      readStaticSource([`../../${app.name}/internal/server/static/offline.html`]),
      readStaticSource([`../../${app.name}/internal/server/locale.go`]),
      readStaticSource(["../../../packages/webassets/static/offline-runtime.js", "../../../packages/webassets/static/offline-profile.js", "../../../packages/webassets/static/offline-media.js", `../../${app.name}/internal/server/static/service-worker.js`]),
    ]);
    const downloadsPath = offline.match(/\/static\/downloads\.js\?v=[\w-]+/)![0];
    const navigationPath = [...locale.matchAll(/\/static\/main\.kinosail\.bundle\.js\?v=[\w-]+/g)].at(-1)![0];
    const oldDownloadsPath = `/static/downloads.js?v=${app.downloads}`;
    const oldNavigationPath = `/static/main.kinosail.bundle.js?v=${app.navigation}`;
    const errors: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    const hits = new Map<string, number>();
    const shell: Record<string, string> = JSON.parse(workerSource.match(/const shell = (\{.*\});/)![1]);
    let deployed = false;
    // Real worker lifecycle, profile storage, and shell validation run in the browser.
    // No request interception: the browser's actual immutable HTTP cache must run.
    const server = createServer((request, response) => {
      const path = request.url!;
      if (path.startsWith("/service-worker.js")) {
        response.writeHead(200, { "Content-Type": "text/javascript", "Cache-Control": "no-cache" });
        response.end(deployed ? workerSource : workerSource.replace(/kinosail-shell-v[0-9]+/g, "kinosail-shell-v38"));
      } else if (path.startsWith("/static/")) {
        const download = path.startsWith("/static/downloads.js");
        const old = path === (download ? oldDownloadsPath : oldNavigationPath);
        const source = download ? (old ? oldDownloads : currentDownloads) + exposeWorker : old ? oldPWA : currentPWA;
        if ([oldDownloadsPath, oldNavigationPath, downloadsPath, navigationPath].includes(path)) hits.set(path, (hits.get(path) || 0) + 1);
        response.writeHead(200, { "Content-Type": shell[path.split("?", 1)[0]] || "text/javascript", "Cache-Control": path.includes("?") ? "public, max-age=31536000, immutable" : "public, max-age=86400" });
        response.end(source);
      } else {
        const old = path === "/old";
        if (path === "/upgrade") deployed = true;
        response.writeHead(200, { "Content-Type": "text/html", "Cache-Control": "no-store" });
        response.end(`<!doctype html><script defer src="${old ? oldDownloadsPath : downloadsPath}"></script><script defer src="${old ? oldNavigationPath : navigationPath}"></script><body data-viewer-profile="viewer"></body>`);
      }
    });
    await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
    const origin = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
    try {
      await page.goto(`${origin}/old`);
      await expect.poll(() => page.evaluate(() => (window as WorkerWindow).checkWorker())).toBe("ready");
      await page.addInitScript(() => {
        navigator.serviceWorker.addEventListener("message", event => {
          if (event.data?.type === "offline-profile") (window as WorkerWindow).identifiedProfile = {
            type: event.data.type, profile: event.data.profile, worker: (event.source as ServiceWorker).scriptURL,
          };
        });
      });
      await page.goto(`${origin}/upgrade`);
      await page.waitForFunction((worker) => navigator.serviceWorker.controller?.scriptURL === new URL(worker, location.href).href, currentWorker);
      await expect.poll(() => page.evaluate(() => (window as WorkerWindow).checkWorker())).toBe("ready");
      await expect.poll(() => page.evaluate(() => (window as WorkerWindow).identifiedProfile)).toEqual({type:"offline-profile", profile:"viewer", worker:origin+currentWorker});
      await expect.poll(() => page.evaluate(() => caches.keys())).toEqual([workerSource.match(/const cacheName = "([^"]+)"/)![1]]);
      expect(downloadsPath).not.toBe(oldDownloadsPath);
      expect(navigationPath).not.toBe(oldNavigationPath);
      for (const path of [oldDownloadsPath, oldNavigationPath, downloadsPath, navigationPath]) expect(hits.get(path)).toBe(1);
      await page.reload();
      await expect.poll(() => page.evaluate(() => (window as WorkerWindow).checkWorker())).toBe("ready");
      for (const count of hits.values()) expect(count).toBe(1);
      expect(errors).toEqual([]);
    } finally {
      await page.close();
      server.closeAllConnections();
      await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
    }
  });
}
