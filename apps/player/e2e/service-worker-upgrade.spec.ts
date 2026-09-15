import { expect, test } from "@playwright/test";
import { createServer } from "node:http";
import type { AddressInfo } from "node:net";
import { readStaticSource } from "./static-sources";

const currentDownloads = await readStaticSource(["../../../packages/webassets/static/downloads.js"]);
const currentPWA = await readStaticSource(["../../../packages/webassets/static/pwa.js"]);
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
    const [offline, locale] = await Promise.all([
      readStaticSource([`../../${app.name}/internal/server/static/offline.html`]),
      readStaticSource([`../../${app.name}/internal/server/locale.go`]),
    ]);
    const downloadsPath = offline.match(/\/static\/downloads\.js\?v=[\w-]+/)![0];
    const navigationPath = [...locale.matchAll(/\/static\/main\.kinosail\.bundle\.js\?v=[\w-]+/g)].at(-1)![0];
    const oldDownloadsPath = `/static/downloads.js?v=${app.downloads}`;
    const oldNavigationPath = `/static/main.kinosail.bundle.js?v=${app.navigation}`;
    const hits = new Map<string, number>();
    // No request interception: the browser's actual immutable HTTP cache must run.
    const server = createServer((request, response) => {
      const path = request.url!;
      if (path.startsWith("/service-worker.js")) {
        response.writeHead(200, { "Content-Type": "text/javascript", "Cache-Control": "no-cache" });
        response.end("self.addEventListener('install', e => e.waitUntil(self.skipWaiting())); self.addEventListener('activate', e => e.waitUntil(self.clients.claim()));");
      } else if (path.startsWith("/static/")) {
        const download = path.startsWith("/static/downloads.js");
        const old = path === (download ? oldDownloadsPath : oldNavigationPath);
        const source = download ? (old ? oldDownloads : currentDownloads) + exposeWorker : old ? oldPWA : currentPWA;
        hits.set(path, (hits.get(path) || 0) + 1);
        response.writeHead(200, { "Content-Type": "text/javascript", "Cache-Control": "public, max-age=31536000, immutable" });
        response.end(source);
      } else {
        const old = path === "/old";
        response.writeHead(200, { "Content-Type": "text/html", "Cache-Control": "no-store" });
        response.end(`<!doctype html><script defer src="${old ? oldDownloadsPath : downloadsPath}"></script><script defer src="${old ? oldNavigationPath : navigationPath}"></script><body></body>`);
      }
    });
    await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
    const origin = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
    try {
      await page.goto(`${origin}/old`);
      await expect.poll(() => page.evaluate(() => (window as any).checkWorker())).toBe("ready");
      await page.goto(`${origin}/upgrade`);
      await page.waitForFunction((worker) => navigator.serviceWorker.controller?.scriptURL === new URL(worker, location.href).href, currentWorker);
      await expect.poll(() => page.evaluate(() => (window as any).checkWorker())).toBe("ready");
      expect(downloadsPath).not.toBe(oldDownloadsPath);
      expect(navigationPath).not.toBe(oldNavigationPath);
      for (const path of [oldDownloadsPath, oldNavigationPath, downloadsPath, navigationPath]) expect(hits.get(path)).toBe(1);
      await page.reload();
      await expect.poll(() => page.evaluate(() => (window as any).checkWorker())).toBe("ready");
      for (const count of hits.values()) expect(count).toBe(1);
    } finally {
      await page.goto("about:blank");
      server.closeAllConnections();
      await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
    }
  });
}
