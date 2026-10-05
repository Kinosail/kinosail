import { expect, type Browser, type Page, type TestInfo } from "@playwright/test";
import { createServer, type ServerResponse } from "node:http";
import { createHash } from "node:crypto";
import { downloadsSource, serviceWorkerSource, readStaticSource } from "./static-sources";

export const downloadChunk = 8 * 1024 * 1024;
export const downloadBytes = Buffer.concat([Buffer.alloc(downloadChunk, 1), Buffer.alloc(downloadChunk, 2), Buffer.alloc(31, 3)]);
export const downloadHash = createHash("sha256").update(downloadBytes).digest("hex");
export const firstBlockHash = createHash("sha256").update(downloadBytes.subarray(0, downloadChunk)).digest("hex");
export const downloadServer = process.env.KINOSAIL_DOWNLOAD_PAUSE_URL;
export const downloadIsolated = process.env.KINOSAIL_DOWNLOAD_PAUSE_ISOLATED === "1";
const stylesheet = await readStaticSource(["../../../packages/webassets/static/player-app.css", "../../../packages/webassets/static/last-light.css"]);
const id = "aaaaaaaaaaaaaaaa", item = "bbbbbbbbbbbbbbbb";
type PeerStats = { ranges: number[]; closed: number; removals: number };

export async function attachDownloadEnvironment(browser: Browser, info: TestInfo) {
  await info.attach("native-download-environment", {body: JSON.stringify({
    browser: browser.browserType().name(), version: browser.version(),
    requestedChannel: process.env.PLAYWRIGHT_CHANNEL ?? "configuration default",
    project: info.project.name, node: process.version, isolated: downloadIsolated,
  }), contentType: "application/json"});
}

export async function downloadPeer() {
  if (downloadServer) {
    expect((await fetch(`${downloadServer}/__download-pause?reset=1`, {method: "POST"})).ok).toBe(true);
    return {
      origin: downloadServer,
      async stats(): Promise<PeerStats> { return (await fetch(`${downloadServer}/__download-pause`)).json(); },
      async close() {},
    };
  }
  const pending = new Set<ServerResponse>();
  const ranges: number[] = [];
  let closed = 0, removals = 0, held = false;
  const shell = `<html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width"><link rel="stylesheet" href="/static/app.css"><script defer src="/static/downloads.js?v=baseline"></script></head><body class="detail-page" data-viewer-profile="profile"><main class="detail-shell downloads-shell" id="downloads"><h1>Offline downloads</h1><p>Prepared files are private to this Viewer Profile.</p><article class="download-job" data-download-job="${id}"><h2>Fictional Range Fixture</h2><p>original · Ready to download</p><div class="download-device-actions"><button class="mode" type="button" data-download-device data-download-focus="device" data-job-id="${id}" data-item-id="${item}" data-title="Fictional Range Fixture" data-quality="original">Download to this device</button><a class="mode" data-download-play hidden href="/offline?job=${id}">Play offline</a><span role="status" aria-live="polite" data-download-device-status>Not stored on this device</span></div><details><summary>Remove download</summary><form method="post" action="/offline-downloads/${id}/remove"><button class="danger">Remove download</button></form></details></article></main></body></html>`;
  const web = createServer((request, response) => {
    const path = new URL(request.url!, "http://localhost").pathname;
    if (request.method === "DELETE" || path.endsWith("/remove")) { removals++; response.writeHead(405); response.end(); return; }
    if (path === `/api/v1/downloads/${id}/file`) {
      const range = request.headers.range?.match(/^bytes=(\d+)-(\d+)$/);
      if (!range) { response.writeHead(416); response.end(); return; }
      const offset = Number(range[1]), end = Number(range[2]);
      ranges.push(offset);
      const bytes = downloadBytes.subarray(offset, end + 1);
      response.writeHead(206, {"Content-Range": `bytes ${offset}-${end}/${downloadBytes.length}`, "Content-Digest": `sha-256=:${createHash("sha256").update(bytes).digest("base64")}:`});
      if (offset === downloadChunk && !held) {
        held = true;
        pending.add(response);
        response.on("close", () => { pending.delete(response); closed++; });
        response.write(bytes.subarray(0, 65536));
      } else response.end(bytes);
      return;
    }
    if (path === `/api/v1/downloads/${id}`) {
      response.setHeader("Content-Type", "application/json");
      response.end(JSON.stringify({id, itemId: item, profileId: "profile", title: "Fictional Range Fixture", quality: "original", state: "ready", readyOffline: true, extension: ".mp4", size: downloadBytes.length, sha256: downloadHash}));
      return;
    }
    if (path === `/api/v1/items/${item}`) {
      response.setHeader("Content-Type", "application/json");
      response.end(JSON.stringify({profileId: "profile", item: {id: item, progress: {seconds: 0, watched: false, session: "", revision: 0}}}));
      return;
    }
    if (path === "/service-worker.js") { response.setHeader("Content-Type", "text/javascript"); response.end(serviceWorkerSource); return; }
    if (path === "/static/downloads.js") { response.setHeader("Content-Type", "text/javascript"); response.end(downloadsSource); return; }
    if (path === "/static/app.css") { response.setHeader("Content-Type", "text/css"); response.end(stylesheet); return; }
    // Ancillary shell cache assets have correct types, but are fixture placeholders.
    if (path.startsWith("/static/")) {
      response.setHeader("Content-Type", path.endsWith(".js") ? "text/javascript" : path.endsWith(".woff2") ? "font/woff2" : path.endsWith(".svg") ? "image/svg+xml" : path.endsWith(".jpg") ? "image/jpeg" : "image/png");
      response.end(); return;
    }
    const otherProfile = new URL(request.url!, "http://localhost").searchParams.get("profile") === "other";
    response.setHeader("Content-Type", "text/html"); response.end(otherProfile ? shell.replace('data-viewer-profile="profile"', 'data-viewer-profile="other"') : shell);
  });
  await new Promise<void>((resolve) => web.listen(0, "127.0.0.1", resolve));
  const address = web.address();
  if (!address || typeof address === "string") throw new Error("Missing download fixture port");
  return {
    origin: `http://127.0.0.1:${address.port}`,
    async stats(): Promise<PeerStats> { return {ranges: [...ranges], closed, removals}; },
    async close() {
      await new Promise<void>((resolve, reject) => {
        web.close((error) => error ? reject(error) : resolve());
        for (const response of pending) response.destroy();
        web.closeAllConnections();
      });
    },
  };
}

export async function openDownloadPage(page: Page, origin: string, storage: "opfs" | "indexeddb") {
  if (storage === "indexeddb") await page.addInitScript(() => Object.defineProperty(navigator.storage, "getDirectory", {configurable: true, value: undefined}));
  await page.goto(`${origin}/offline-downloads`);
  await expect.poll(() => page.evaluate(() => navigator.serviceWorker.controller?.scriptURL)).toBe(`${origin}/service-worker.js?v=55`);
  await page.reload();
  const button = page.locator("[data-download-device]");
  await expect(button).toHaveAttribute("data-bound", "true");
  const jobID = await button.getAttribute("data-job-id");
  if (!jobID || !/^[a-f0-9]{16}$/.test(jobID)) throw new Error("No canonical fixture job");
  const bundle = await page.locator('script[src^="/static/downloads.js"]').getAttribute("src");
  const asset = await page.request.get(new URL(bundle!, origin).toString());
  expect(asset.ok()).toBe(true);
  const checksum = createHash("sha256").update(await asset.body()).digest("hex");
  expect(checksum).toBe(createHash("sha256").update(downloadsSource).digest("hex"));
  return {jobID, bundle, checksum};
}


export async function offlineWriterCapability(page: Page) {
  return page.evaluate(async () => {
    if (!navigator.storage?.getDirectory || !("Worker" in window)) return {supported: false, reason: "missing-storage-worker"};
    const source = `onmessage = async () => {
      const name = "kinosail-test-capability-" + crypto.randomUUID();
      let directory, handle, stage = "worker-api";
      try {
        if (typeof FileSystemFileHandle === "undefined" || !FileSystemFileHandle.prototype.createSyncAccessHandle) {
          postMessage({supported: false, reason: "missing-sync-access-handle"}); return;
        }
        stage = "directory"; directory = await navigator.storage.getDirectory();
        stage = "sync-handle"; handle = await (await directory.getFileHandle(name, {create: true})).createSyncAccessHandle();
        stage = "write-read"; const bytes = new Uint8Array([7, 19]);
        if (handle.write(bytes, {at: 0}) !== 2) throw new Error("short-write");
        handle.flush();
        const read = new Uint8Array(2);
        if (handle.read(read, {at: 0}) !== 2 || read[0] !== 7 || read[1] !== 19) throw new Error("read-mismatch");
        handle.close(); handle = undefined;
        await directory.removeEntry(name); directory = undefined;
        postMessage({supported: true, reason: "verified-worker-read-write"});
      } catch (error) {
        try { handle?.close(); if (directory) await directory.removeEntry(name); } catch {}
        if (["directory", "sync-handle"].includes(stage) && error instanceof DOMException) postMessage({supported: false, reason: stage + ":" + error.name});
        else postMessage({error: stage + ":" + (error instanceof DOMException ? error.name : "Error")});
      }
    };`;
    const url = URL.createObjectURL(new Blob([source], {type: "text/javascript"}));
    const worker = new Worker(url);
    try {
      return await new Promise<{supported: boolean; reason: string}>((resolve, reject) => {
        const timeout = setTimeout(() => reject(new Error("Offline capability probe timed out")), 5000);
        worker.onmessage = ({data}) => {
          clearTimeout(timeout);
          if (data.error) reject(new Error("Offline capability probe: " + data.error));
          else resolve(data);
        };
        worker.onerror = () => {clearTimeout(timeout); reject(new Error("Offline capability worker failed"));};
        worker.postMessage(null);
      });
    } finally {worker.terminate(); URL.revokeObjectURL(url);}
  });
}

export async function inspectDownload(page: Page, jobID: string) {
  return page.evaluate(async (id) => {
    type Job = {id: string; bytes: number; state: string; readyOffline: boolean; storage: string; sha256: string; transferID: string; profileID: string; itemID: string};
    type Chunk = {id: string; jobID: string; offset: number; length: number; sha256: string; data?: ArrayBuffer};
    let stage = "idb-open", database: IDBDatabase | undefined;
    try {
      database = await new Promise<IDBDatabase>((resolve, reject) => {
        const request = indexedDB.open("kinosail-offline-v1");
        request.onsuccess = () => resolve(request.result); request.onerror = () => reject(request.error);
      });
      stage = "idb-readonly-getAll";
      const records = await new Promise<{job?: Job; chunks: Chunk[]}>((resolve, reject) => {
        const transaction = database!.transaction(["jobs", "chunks"], "readonly");
        const job = transaction.objectStore("jobs").get(id), chunks = transaction.objectStore("chunks").index("jobID").getAll(id);
        transaction.oncomplete = () => resolve({job: job.result, chunks: chunks.result});
        transaction.onabort = () => reject(transaction.error);
      });
      database.close(); database = undefined;
      const hash = async (data: ArrayBuffer) => [...new Uint8Array(await crypto.subtle.digest("SHA-256", data))].map((byte) => byte.toString(16).padStart(2, "0")).join("");
      stage = "opfs-file";
      let file: File | undefined;
      if (records.job?.storage === "opfs") file = await (await (await navigator.storage.getDirectory()).getFileHandle(id)).getFile();
      const stored = records.chunks.sort((a, b) => a.offset - b.offset), chunks = [];
      for (const chunk of stored) {
        stage = "chunk-read";
        const bytes = file ? await file.slice(chunk.offset, chunk.offset + chunk.length).arrayBuffer() : chunk.data!;
        stage = "chunk-digest";
        chunks.push({offset: chunk.offset, length: chunk.length, sha256: chunk.sha256, actual: await hash(bytes)});
      }
      stage = file ? "opfs-whole-file-read" : "indexeddb-whole-file-copy";
      // IndexedDB already returned the real bytes. Blob reads can use WebKit
      // network machinery which Playwright offline emulation disables.
      const combined = new Uint8Array(file ? 0 : stored.reduce((size, chunk) => size + chunk.data!.byteLength, 0));
      let offset = 0;
      for (const chunk of stored) {if (file) break; combined.set(new Uint8Array(chunk.data!), offset); offset += chunk.data!.byteLength;}
      const bytes = file ? await file.arrayBuffer() : combined.buffer;
      stage = "whole-file-digest";
      const fileHash = await hash(bytes);
      stage = "locks-query";
      const locks = await navigator.locks.query();
      return {job: records.job, chunks, fileSize: bytes.byteLength, fileHash, locks: locks.held.map((lock) => lock.name)};
    } catch (error) {
      const name = error instanceof DOMException ? error.name : "Error";
      throw new Error("Offline storage inspection failed at " + stage + ": " + name);
    } finally {database?.close();}
  }, jobID);
}
