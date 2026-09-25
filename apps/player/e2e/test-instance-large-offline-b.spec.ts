import { expect, test } from "@playwright/test";
import { createHash } from "node:crypto";
import { configureTestInstance, downloadsSource, firstPlayable, login } from "./test-instance-helpers";

configureTestInstance();

test.describe("large offline transfers", () => {

  test("offline resume does not count an orphaned OPFS write twice against quota", async ({ page }) => {
    await page.route((url) => url.pathname === "/static/downloads.js", (route) => route.fulfill({ contentType: "text/javascript", body: downloadsSource.replace("const chunkSize = 8 * 1024 * 1024;", "const chunkSize = 16;") }));
    await page.addInitScript(() => {
      const worker = { scriptURL: new URL("/service-worker.js?v=53", location.href).href, state: "activated" };
      Object.defineProperties(navigator.serviceWorker, {
        controller: { configurable: true, get: () => worker },
        getRegistration: { configurable: true, value: async () => ({ active: worker }) },
      });
      Object.assign(window, { __offlineFile: new Uint8Array(16).fill(1) });
      const root = {
        getFileHandle: async () => ({
          createWritable: async () => ({
            close: async () => {},
            truncate: async (size: number) => {
              const current = (window as typeof window & { __offlineFile: Uint8Array }).__offlineFile;
              const replacement = new Uint8Array(size);
              replacement.set(current.subarray(0, size));
              (window as typeof window & { __offlineFile: Uint8Array }).__offlineFile = replacement;
            },
            write: async ({ data, position }: { data: ArrayBuffer; position: number }) => {
              const current = (window as typeof window & { __offlineFile: Uint8Array }).__offlineFile;
              const incoming = new Uint8Array(data);
              const replacement = new Uint8Array(Math.max(current.length, position + incoming.length));
              replacement.set(current);
              replacement.set(incoming, position);
              (window as typeof window & { __offlineFile: Uint8Array }).__offlineFile = replacement;
            },
          }),
          getFile: async () => new File([(window as typeof window & { __offlineFile: Uint8Array }).__offlineFile], "offline.mp4"),
        }),
        removeEntry: async () => { (window as typeof window & { __offlineFile: Uint8Array }).__offlineFile = new Uint8Array(); },
      };
      Object.defineProperties(navigator.storage, {
        estimate: { configurable: true, value: async () => ({ quota: 100, usage: 80 }) },
        getDirectory: { configurable: true, value: async () => root },
        persist: { configurable: true, value: async () => true },
      });
    });
    await login(page);
    await page.goto(await firstPlayable(page));
    await page.getByText("Playback & downloads", { exact: true }).click();
    await page.getByRole("button", { name: "Prepare 720p offline", exact: true }).click();
    await expect(page.getByText("720p · Ready to download", { exact: true })).toBeVisible({ timeout: 30_000 });

    const button = page.getByRole("button", { name: "Download to this device", exact: true });
    const jobID = await button.getAttribute("data-job-id");
    const itemID = await button.getAttribute("data-item-id");
    const title = await button.getAttribute("data-title");
    const quality = await button.getAttribute("data-quality");
    const profileID = await page.locator("#downloads").getAttribute("data-viewer-profile");
    expect(jobID && itemID && title && quality && profileID).toBeTruthy();
    const media = Buffer.concat([Buffer.alloc(16, 1), Buffer.alloc(16, 2), Buffer.alloc(1, 3)]);
    const sha256 = createHash("sha256").update(media).digest("hex");
    const ranges: number[] = [];
    await page.route((url) => url.pathname === `/api/v1/downloads/${jobID}`, (route) => route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({ id: jobID, itemId: itemID, profileId: profileID, title, quality, state: "ready", extension: ".mp4", sha256, size: media.length, readyOffline: true }),
    }));
    await page.route((url) => url.pathname === `/api/v1/downloads/${jobID}/file`, (route) => {
      const match = route.request().headers().range?.match(/^bytes=(\d+)-(\d+)$/);
      if (!match) return route.abort();
      const start = Number(match[1]);
      const end = Number(match[2]);
      const body = media.subarray(start, end + 1);
      ranges.push(body.length);
      return route.fulfill({
        status: 206,
        body,
        headers: { "Content-Digest": `sha-256=:${createHash("sha256").update(body).digest("base64")}:`, "Content-Range": `bytes ${start}-${end}/${media.length}` },
      });
    });
    await page.evaluate(({ id, itemID, profileID, quality, sha256, size, title }) => new Promise<void>((resolve, reject) => {
      const request = indexedDB.open("kinosail-offline-v1", 4);
      request.onsuccess = () => {
        const database = request.result;
        const transaction = database.transaction("jobs", "readwrite");
        transaction.objectStore("jobs").put({ id, integrityVersion: 2, profileID, itemID, title, quality, extension: ".mp4", sha256, size, storage: "opfs", state: "needs_attention", bytes: 0 });
        transaction.oncomplete = () => { database.close(); resolve(); };
        transaction.onerror = () => reject(transaction.error);
      };
      request.onerror = () => reject(request.error);
    }), { id: jobID!, itemID: itemID!, profileID: profileID!, quality: quality!, sha256, size: media.length, title: title! });

    await button.click();
    await expect(page.getByText("Ready offline on this device", { exact: true })).toBeVisible({ timeout: 20_000 });
    expect(ranges).toEqual([16, 16, 1]);
    expect(await page.evaluate(() => (window as typeof window & { __offlineFile: Uint8Array }).__offlineFile.length)).toBe(media.length);
  });

  test("offline resume accounts for replaced IndexedDB chunks near quota", async ({ page }) => {
    await page.route((url) => url.pathname === "/static/downloads.js", (route) => route.fulfill({ contentType: "text/javascript", body: downloadsSource.replace("const chunkSize = 8 * 1024 * 1024;", "const chunkSize = 16;") }));
    await page.addInitScript(() => {
      const worker = { scriptURL: new URL("/service-worker.js?v=53", location.href).href, state: "activated" };
      Object.defineProperties(navigator.serviceWorker, {
        controller: { configurable: true, get: () => worker },
        getRegistration: { configurable: true, value: async () => ({ active: worker }) },
      });
      Object.defineProperties(navigator.storage, {
        estimate: { configurable: true, value: async () => ({ quota: 100, usage: 80 }) },
        getDirectory: { configurable: true, value: undefined },
        persist: { configurable: true, value: async () => true },
      });
    });
    await login(page);
    await page.goto(await firstPlayable(page));
    await page.getByText("Playback & downloads", { exact: true }).click();
    await page.getByRole("button", { name: "Prepare 720p offline", exact: true }).click();
    await expect(page.getByText("720p · Ready to download", { exact: true })).toBeVisible({ timeout: 30_000 });

    const button = page.getByRole("button", { name: "Download to this device", exact: true });
    const jobID = await button.getAttribute("data-job-id");
    const itemID = await button.getAttribute("data-item-id");
    const title = await button.getAttribute("data-title");
    const quality = await button.getAttribute("data-quality");
    const profileID = await page.locator("#downloads").getAttribute("data-viewer-profile");
    expect(jobID && itemID && title && quality && profileID).toBeTruthy();
    const media = Buffer.concat([Buffer.alloc(16, 1), Buffer.alloc(16, 2), Buffer.alloc(1, 3)]);
    const sha256 = createHash("sha256").update(media).digest("hex");
    const ranges: number[] = [];
    await page.route((url) => url.pathname === `/api/v1/downloads/${jobID}`, (route) => route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({ id: jobID, itemId: itemID, profileId: profileID, title, quality, state: "ready", extension: ".mp4", sha256, size: media.length, readyOffline: true }),
    }));
    await page.route((url) => url.pathname === `/api/v1/downloads/${jobID}/file`, (route) => {
      const match = route.request().headers().range?.match(/^bytes=(\d+)-(\d+)$/);
      if (!match) return route.abort();
      const start = Number(match[1]);
      const end = Number(match[2]);
      const body = media.subarray(start, end + 1);
      ranges.push(body.length);
      return route.fulfill({
        status: 206,
        body,
        headers: { "Content-Digest": `sha-256=:${createHash("sha256").update(body).digest("base64")}:`, "Content-Range": `bytes ${start}-${end}/${media.length}` },
      });
    });
    await page.evaluate(({ id, itemID, profileID, quality, sha256, size, title }) => new Promise<void>((resolve, reject) => {
      const request = indexedDB.open("kinosail-offline-v1", 4);
      request.onsuccess = () => {
        const database = request.result;
        const transaction = database.transaction(["jobs", "chunks"], "readwrite");
        transaction.objectStore("jobs").put({ id, integrityVersion: 2, profileID, itemID, title, quality, extension: ".mp4", sha256, size, storage: "indexeddb", state: "needs_attention", bytes: 0 });
        transaction.objectStore("chunks").put({ id: `${id}:0`, jobID: id, offset: 0, length: 16, sha256: "0".repeat(64), data: new Uint8Array(16).buffer });
        transaction.oncomplete = () => { database.close(); resolve(); };
        transaction.onerror = () => reject(transaction.error);
      };
      request.onerror = () => reject(request.error);
    }), { id: jobID!, itemID: itemID!, profileID: profileID!, quality: quality!, sha256, size: media.length, title: title! });

    await button.click();
    await expect(page.getByText("Ready offline on this device", { exact: true })).toBeVisible({ timeout: 20_000 });
    expect(ranges).toEqual([16, 16, 1]);
  });

});
