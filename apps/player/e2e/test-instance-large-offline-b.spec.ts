import { expect, test } from "@playwright/test";
import { createHash } from "node:crypto";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { configureTestInstance, downloadsSource, firstPlayable, login } from "./test-instance-helpers";

configureTestInstance();
test.use({ serviceWorkers: "block" });

// WebKit advertises OPFS in ephemeral contexts but cannot open its directory there.
const opfsTest = test.extend({
  context: async ({ playwright, browserName, launchOptions, channel, headless, contextOptions, baseURL, viewport, userAgent, deviceScaleFactor, isMobile, hasTouch, colorScheme, locale, ignoreHTTPSErrors, serviceWorkers, video }, use, testInfo) => {
    const profile = await mkdtemp(join(tmpdir(), "kinosail-opfs-quota-"));
    const videoDirectory = testInfo.outputPath("opfs-video");
    const videoMode = typeof video === "string" ? video : video.mode;
    let context: import("@playwright/test").BrowserContext | undefined;
    try {
      context = await playwright[browserName].launchPersistentContext(profile, {
        ...launchOptions, ...contextOptions, channel, headless, baseURL, viewport, userAgent,
        deviceScaleFactor, isMobile, hasTouch, colorScheme, locale, ignoreHTTPSErrors, serviceWorkers,
        ...(videoMode === "off" ? {} : { recordVideo: { dir: videoDirectory, size: typeof video === "string" ? undefined : video.size } }),
      });
      await use(context);
    } finally {
      const pages = context?.pages() ?? [];
      const recordings = pages.map((page) => page.video()).filter((recording) => recording !== null);
      try {
        // Finish recordings while the persistent browser connection is still open.
        await Promise.all(pages.map((page) => page.close()));
        if (videoMode === "on" || testInfo.status !== testInfo.expectedStatus) {
          for (const [index, recording] of recordings.entries()) {
            const path = testInfo.outputPath(`opfs-video-${index}.webm`);
            await recording.saveAs(path);
            await testInfo.attach("video", { path, contentType: "video/webm" });
          }
        }
      } finally {
        try { await context?.close(); }
        finally {
          await rm(profile, { recursive: true, force: true });
          await rm(videoDirectory, { recursive: true, force: true });
          await testInfo.attach("opfs-profile", { body: JSON.stringify({ mode: "fresh-persistent", removed: true }), contentType: "application/json" });
        }
      }
    }
  },
});

test.describe("large offline transfers", () => {

  opfsTest("offline resume does not count an orphaned OPFS write twice against quota", async ({ page }) => {
    await page.route((url) => url.pathname === "/static/downloads.js", (route) => route.fulfill({ contentType: "text/javascript", body: downloadsSource.replace("const chunkSize = 8 * 1024 * 1024;", "const chunkSize = 16;") }));
    await page.addInitScript(() => {
      const worker = { scriptURL: new URL("/service-worker.js?v=56", location.href).href, state: "activated" };
      Object.defineProperties(navigator.serviceWorker, {
        controller: { configurable: true, get: () => worker },
        getRegistration: { configurable: true, value: async () => ({ active: worker }) },
      });
      Object.defineProperties(navigator.storage, {
        estimate: { configurable: true, value: async () => ({ quota: 100, usage: 80 }) },
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
    const supportsOPFS = await page.evaluate(async (id) => {
      // Match the production writer's worker API; Window createWritable is not portable.
      const scope = async (jobID: string) => {
        if (typeof FileSystemFileHandle === "undefined" || !FileSystemFileHandle.prototype.createSyncAccessHandle) return self.postMessage({ supported: false });
        try {
          const file = await (await navigator.storage.getDirectory()).getFileHandle(jobID, { create: true });
          const writer = await file.createSyncAccessHandle();
          try { writer.write(new Uint8Array(16).fill(1)); writer.flush(); }
          finally { writer.close(); }
          self.postMessage({ supported: true });
        } catch (error) { self.postMessage({ error: String(error) }); }
      };
      const url = URL.createObjectURL(new Blob([`(${scope.toString()})(${JSON.stringify(id)})`], { type: "text/javascript" }));
      const worker = new Worker(url);
      try {
        return await new Promise<boolean>((resolve, reject) => {
          worker.onmessage = ({ data }) => data.error ? reject(new Error(data.error)) : resolve(data.supported);
          worker.onerror = reject;
        });
      } finally { worker.terminate(); URL.revokeObjectURL(url); }
    }, jobID!);
    test.skip(!supportsOPFS, "This engine lacks the OPFS sync writer; IndexedDB quota resume is covered separately.");
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

    await page.reload();
    await page.getByRole("button", { name: "Resume on this device", exact: true }).click();
    await expect(page.getByText("Saved and verified. Play to check compatibility.", { exact: true })).toBeVisible({ timeout: 20_000 });
    expect(ranges).toEqual([16, 16, 1]);
    expect(await page.evaluate(async (id) => (await (await (await navigator.storage.getDirectory()).getFileHandle(id)).getFile()).size, jobID!)).toBe(media.length);
  });

  test("offline resume accounts for replaced IndexedDB chunks near quota", async ({ page }) => {
    await page.route((url) => url.pathname === "/static/downloads.js", (route) => route.fulfill({ contentType: "text/javascript", body: downloadsSource.replace("const chunkSize = 8 * 1024 * 1024;", "const chunkSize = 16;") }));
    await page.addInitScript(() => {
      const worker = { scriptURL: new URL("/service-worker.js?v=56", location.href).href, state: "activated" };
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

    await page.reload();
    await page.getByRole("button", { name: "Resume on this device", exact: true }).click();
    await expect(page.getByText("Saved and verified. Play to check compatibility.", { exact: true })).toBeVisible({ timeout: 20_000 });
    expect(ranges).toEqual([16, 16, 1]);
  });

});
