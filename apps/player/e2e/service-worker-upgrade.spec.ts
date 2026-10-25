import { expect, test as base, type Video } from "@playwright/test";
import { createHash } from "node:crypto";
import { createServer } from "node:http";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import type { AddressInfo } from "node:net";
import { readStaticSource } from "./static-sources";

const test = base.extend({
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

type WorkerWindow = Window & { identifiedProfile?: { type: string; profile: string; worker: string } };

const currentPWA = await readStaticSource(["../../../packages/webassets/static/offline-identity.js", "../../../packages/webassets/static/pwa-library.js", "../../../packages/webassets/static/pwa.js"]);
const currentWorker = currentPWA.match(/register\("([^"]+)"\)/)![1];
const oldWorker = "/service-worker.js?v=38";
const oldPWA = currentPWA.replaceAll(currentWorker, oldWorker);
const contentVersion = (source: string) => createHash("sha256").update(source).digest("hex");
const controller = (page: import("@playwright/test").Page) => page.evaluate(() => navigator.serviceWorker.controller?.scriptURL || "");

test("player refreshes its immutable script and removes browser downloads when upgrading its offline worker", async ({ page }) => {
  const workerSource = await readStaticSource(["../../../packages/webassets/static/offline-runtime.js", "../../../packages/webassets/static/offline-profile.js", "../../../packages/webassets/static/offline-shell.js", "../internal/server/static/service-worker.js"]);
  const navigationPath = `/static/main.kinosail.bundle.js?v=${contentVersion(currentPWA)}`;
  const oldNavigationPath = "/static/main.kinosail.bundle.js?v=20";
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  const hits = new Map<string, number>();
  const shell: Record<string, string> = JSON.parse(workerSource.match(/const shell = (\{.*\});/)![1]);
  let deployed = false;
  const server = createServer((request, response) => {
    const path = request.url!;
    if (path.startsWith("/service-worker.js")) {
      response.writeHead(200, { "Content-Type": "text/javascript", "Cache-Control": "no-cache" });
      response.end(deployed ? workerSource : workerSource.replace(/kinosail-shell-v[0-9]+/g, "kinosail-shell-v38"));
    } else if (path.startsWith("/static/")) {
      const old = path === oldNavigationPath;
      if ([oldNavigationPath, navigationPath].includes(path)) hits.set(path, (hits.get(path) || 0) + 1);
      response.writeHead(200, { "Content-Type": shell[path.split("?", 1)[0]] || "text/javascript", "Cache-Control": path.includes("?") ? "public, max-age=31536000, immutable" : "public, max-age=86400" });
      response.end(old ? oldPWA : currentPWA);
    } else {
      const old = path === "/old";
      if (path === "/upgrade") deployed = true;
      response.writeHead(200, { "Content-Type": "text/html", "Cache-Control": "no-store" });
      response.end(`<!doctype html><script defer src="${old ? oldNavigationPath : navigationPath}"></script><body data-viewer-profile="viewer"></body>`);
    }
  });
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  const origin = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
  try {
    await page.goto(`${origin}/old`);
    await expect.poll(() => controller(page)).toBe(origin + oldWorker);
    const seeded = await page.evaluate(() => new Promise<number>((resolve, reject) => {
      const request = indexedDB.open("kinosail-offline-v1", 4);
      request.onerror = () => reject(request.error);
      request.onsuccess = () => {
        const database = request.result;
        const transaction = database.transaction(["jobs", "chunks"], "readwrite");
        transaction.objectStore("jobs").put({ id: "aaaaaaaaaaaaaaaa", profileID: "viewer", state: "ready" });
        transaction.objectStore("chunks").put({ id: "aaaaaaaaaaaaaaaa:0", jobID: "aaaaaaaaaaaaaaaa", offset: 0, data: new ArrayBuffer(4) });
        const count = transaction.objectStore("chunks").count();
        transaction.oncomplete = () => { database.close(); resolve(count.result); };
        transaction.onerror = () => reject(transaction.error);
      };
    }));
    expect(seeded).toBe(1);
    await page.addInitScript(() => {
      navigator.serviceWorker.addEventListener("message", event => {
        if (event.data?.type === "offline-profile") (window as WorkerWindow).identifiedProfile = {
          type: event.data.type, profile: event.data.profile, worker: (event.source as ServiceWorker).scriptURL,
        };
      });
    });
    await page.goto(`${origin}/upgrade`);
    await expect.poll(() => controller(page)).toBe(origin + currentWorker);
    await expect.poll(() => page.evaluate(() => (window as WorkerWindow).identifiedProfile)).toEqual({type:"offline-profile", profile:"viewer", worker:origin+currentWorker});
    await expect.poll(() => page.evaluate(() => caches.keys())).toEqual([workerSource.match(/const cacheName = "([^"]+)"/)![1]]);
    await expect.poll(() => page.evaluate(() => new Promise<number[]>((resolve, reject) => {
      const request = indexedDB.open("kinosail-offline-v1");
      request.onerror = () => reject(request.error);
      request.onsuccess = () => {
        const database = request.result;
        const transaction = database.transaction(["jobs", "chunks"], "readonly");
        const jobs = transaction.objectStore("jobs").count();
        const chunks = transaction.objectStore("chunks").count();
        transaction.oncomplete = () => { database.close(); resolve([jobs.result, chunks.result]); };
      };
    }))).toEqual([0, 0]);
    expect(navigationPath).not.toBe(oldNavigationPath);
    for (const path of [oldNavigationPath, navigationPath]) expect(hits.get(path)).toBe(1);
    await page.reload();
    await expect.poll(() => controller(page)).toBe(origin + currentWorker);
    for (const count of hits.values()) expect(count).toBe(1);
    expect(errors).toEqual([]);
  } finally {
    await page.close();
    server.closeAllConnections();
    await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
  }
});
