import { expect, test } from "@playwright/test";
import { createHash } from "node:crypto";
import { configureTestInstance, downloadsSource, type OfflineClient } from "./test-instance-helpers";

configureTestInstance();

test.describe("large offline transfers", () => {

  test("offline playback cleanup removes an unchanged copy with a missing later chunk", async ({ page }) => {
    await page.addInitScript(() => {
      const worker = { scriptURL: new URL("/service-worker.js?v=17", location.href).href, state: "activated" };
      Object.defineProperties(navigator.serviceWorker, {
        controller: { configurable: true, get: () => worker },
        getRegistration: { configurable: true, value: async () => ({ active: worker }) },
      });
    });
    await page.route((url) => url.pathname === "/__offline-corrupt-cleanup", (route) => route.fulfill({
      contentType: "text/html",
      body: '<body data-viewer-profile="profile"><main id="downloads"><button hidden data-download-device data-job-id="aaaaaaaaaaaaaaaa" data-item-id="bbbbbbbbbbbbbbbb" data-title="Movie" data-quality="720p">Download</button><span data-download-device-status></span></main><script src="/static/downloads.js?v=5"></script></body>',
    }));
    await page.route((url) => url.pathname === "/static/downloads.js", (route) => route.fulfill({ contentType: "text/javascript", body: downloadsSource }));
    await page.goto("/__offline-corrupt-cleanup");
    await expect(page.locator("[data-download-device]")).toHaveAttribute("data-bound", "true");
    await page.waitForTimeout(200);
    const firstChunk = Buffer.alloc(16, 1);
    await page.evaluate(({ chunk, chunkHash }) => new Promise<void>((resolve, reject) => {
      const request = indexedDB.open("kinosail-offline-v1", 3);
      request.onsuccess = () => {
        const database = request.result;
        const transaction = database.transaction(["jobs", "chunks"], "readwrite");
        transaction.objectStore("jobs").put({ id: "aaaaaaaaaaaaaaaa", integrityVersion: 2, profileID: "profile", itemID: "bbbbbbbbbbbbbbbb", title: "Movie", quality: "720p", extension: ".mp4", sha256: "a".repeat(64), size: 32, storage: "indexeddb", state: "ready", readyOffline: true, bytes: 32, transferID: "1".repeat(32) });
        transaction.objectStore("chunks").put({ id: "aaaaaaaaaaaaaaaa:0", jobID: "aaaaaaaaaaaaaaaa", offset: 0, length: 16, sha256: chunkHash, data: new Uint8Array(chunk).buffer });
        transaction.oncomplete = () => { database.close(); resolve(); };
        transaction.onerror = () => reject(transaction.error);
      };
      request.onerror = () => reject(request.error);
    }), { chunk: [...firstChunk], chunkHash: createHash("sha256").update(firstChunk).digest("hex") });

    await expect.poll(() => page.evaluate(() => (window as Window & OfflineClient).KinosailOfflineMedia.source("bbbbbbbbbbbbbbbb"))).toBe("/offline-media/profile/aaaaaaaaaaaaaaaa");
    expect(await page.evaluate(() => (window as Window & OfflineClient).KinosailOfflineMedia.remove("aaaaaaaaaaaaaaaa"))).toBe(true);
    expect(await page.evaluate(() => new Promise((resolve, reject) => {
      const request = indexedDB.open("kinosail-offline-v1");
      request.onsuccess = () => {
        const database = request.result;
        const transaction = database.transaction(["jobs", "chunks"], "readonly");
        const jobs = transaction.objectStore("jobs").getAllKeys();
        const chunks = transaction.objectStore("chunks").getAllKeys();
        transaction.oncomplete = () => { database.close(); resolve({ jobs: jobs.result, chunks: chunks.result }); };
        transaction.onerror = () => reject(transaction.error);
      };
      request.onerror = () => reject(request.error);
    }))).toEqual({ jobs: [], chunks: [] });
  });

  test("offline removal waits for a transfer lock across tabs", async ({ context, page }) => {
    await context.addInitScript(() => {
      const worker = { scriptURL: new URL("/service-worker.js?v=17", location.href).href, state: "activated" };
      Object.defineProperties(navigator.serviceWorker, {
        controller: { configurable: true, get: () => worker },
        getRegistration: { configurable: true, value: async () => ({ active: worker }) },
      });
      Object.defineProperty(navigator.storage, "getDirectory", { configurable: true, value: undefined });
      Object.defineProperty(Object.getPrototypeOf(navigator.storage), "persist", { configurable: true, value: async () => true });
      Object.defineProperty(window, "EventSource", { configurable: true, value: class extends EventTarget { close() {} } });
    });
    const jobID = "aaaaaaaaaaaaaaaa";
    const itemID = "bbbbbbbbbbbbbbbb";
    const profileID = "profile";
    const media = Buffer.concat([Buffer.alloc(16, 1), Buffer.alloc(16, 2), Buffer.alloc(1, 3)]);
    const sha256 = createHash("sha256").update(media).digest("hex");
    const oldMedia = Buffer.concat([Buffer.alloc(16, 4), Buffer.alloc(16, 5), Buffer.alloc(1, 6)]);
    const oldSha256 = createHash("sha256").update(oldMedia).digest("hex");
    await context.route((url) => url.pathname === "/__offline-transfer-lock-test", (route) => route.fulfill({
      contentType: "text/html",
      body: `<body data-viewer-profile="${profileID}"><main id="downloads" data-viewer-profile="${profileID}"><article data-download-job="${jobID}"><button data-download-device data-job-id="${jobID}" data-item-id="${itemID}" data-title="Movie" data-quality="720p">Download</button><span data-download-device-status></span></article></main><script src="/static/downloads.js?v=5"></script></body>`,
    }));
    await context.route((url) => url.pathname === "/__offline-remove-lock-test", (route) => route.fulfill({
      contentType: "text/html",
      body: `<body><main id="downloads"><form method="post" action="/offline-downloads/${jobID}/remove"><span hidden data-download-remove-status></span><button type="submit">Confirm remove</button></form></main><script src="/static/downloads.js?v=5"></script></body>`,
    }));
    await context.route((url) => url.pathname === "/static/downloads.js", (route) => route.fulfill({ contentType: "text/javascript", body: downloadsSource.replace("const chunkSize = 8 * 1024 * 1024;", "const chunkSize = 16;") }));
    await context.route((url) => url.pathname === `/api/v1/downloads/${jobID}`, (route) => route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({ id: jobID, itemId: itemID, profileId: profileID, title: "Movie", quality: "720p", state: "ready", extension: ".mp4", sha256, size: media.length, readyOffline: true }),
    }));
    let releaseFirstRange = () => {};
    const firstRangeReleased = new Promise<void>((resolve) => { releaseFirstRange = resolve; });
    let observeFirstRange = () => {};
    const firstRangeObserved = new Promise<void>((resolve) => { observeFirstRange = resolve; });
    let ranges = 0;
    await context.route((url) => url.pathname === `/api/v1/downloads/${jobID}/file`, async (route) => {
      const match = route.request().headers().range?.match(/^bytes=(\d+)-(\d+)$/);
      if (!match) return route.abort();
      const start = Number(match[1]);
      const end = Number(match[2]);
      const body = media.subarray(start, end + 1);
      ranges++;
      if (ranges === 1) { observeFirstRange(); await firstRangeReleased; }
      return route.fulfill({
        status: 206,
        body,
        headers: { "Content-Digest": `sha-256=:${createHash("sha256").update(body).digest("base64")}:`, "Content-Range": `bytes ${start}-${end}/${media.length}` },
      });
    });
    let removals = 0;
    let releaseRemovalResponse = () => {};
    const removalResponseReleased = new Promise<void>((resolve) => { releaseRemovalResponse = resolve; });
    await context.route((url) => url.pathname === `/offline-downloads/${jobID}/remove`, async (route) => {
      if (route.request().method() === "POST") removals++;
      await removalResponseReleased;
      return route.fulfill({ contentType: "text/html", body: "<!doctype html><title>Removed</title>" });
    });

    await page.goto("/__offline-transfer-lock-test");
    await page.evaluate(({ chunks, id, itemID, profileID, sha256 }) => new Promise<void>((resolve, reject) => {
      const request = indexedDB.open("kinosail-offline-v1", 3);
      request.onsuccess = () => {
        const database = request.result;
        const transaction = database.transaction(["jobs", "chunks"], "readwrite");
        transaction.objectStore("jobs").put({ id, integrityVersion: 2, profileID, itemID, title: "Movie", quality: "720p", extension: ".mp4", sha256, size: 33, storage: "indexeddb", state: "ready", readyOffline: true, bytes: 33 });
        for (const chunk of chunks) transaction.objectStore("chunks").put({ id: `${id}:${chunk.offset}`, jobID: id, offset: chunk.offset, length: chunk.data.length, sha256: chunk.sha256, data: new Uint8Array(chunk.data).buffer });
        transaction.oncomplete = () => { database.close(); resolve(); };
        transaction.onerror = () => reject(transaction.error);
      };
      request.onerror = () => reject(request.error);
    }), {
      chunks: [0, 16, 32].map((offset) => {
        const data = [...oldMedia.subarray(offset, Math.min(offset + 16, oldMedia.length))];
        return { data, offset, sha256: createHash("sha256").update(Buffer.from(data)).digest("hex") };
      }),
      id: jobID,
      itemID,
      profileID,
      sha256: oldSha256,
    });
    await expect.poll(() => page.evaluate((id) => (window as Window & OfflineClient).KinosailOfflineMedia.source(id), itemID)).toBe(`/offline-media/${profileID}/${jobID}`);
    await page.getByRole("button", { name: "Download", exact: true }).click();
    await firstRangeObserved;
    const removalPage = await context.newPage();
    await removalPage.goto("/__offline-remove-lock-test");
    const form = removalPage.locator(`form[action="/offline-downloads/${jobID}/remove"]`);
    await expect(form).toHaveAttribute("data-bound", "true");
    await removalPage.evaluate((id) => {
      Object.assign(window, { __playerRemovalDone: false, __playerRemovalResult: undefined });
      (window as Window & OfflineClient).KinosailOfflineMedia.remove(id).then((result) => {
        Object.assign(window, { __playerRemovalDone: true, __playerRemovalResult: result });
      });
    }, jobID);
    expect(await form.evaluate((node) => {
      node.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
      return (node.querySelector("button") as HTMLButtonElement).disabled;
    })).toBe(true);
    await new Promise((resolve) => setTimeout(resolve, 100));
    expect(removals).toBe(0);
    expect(await removalPage.evaluate(() => (window as typeof window & { __playerRemovalDone: boolean }).__playerRemovalDone)).toBe(false);
    releaseFirstRange();
    await expect(page.getByText("Ready offline on this device", { exact: true })).toBeVisible({ timeout: 20_000 });
    await expect.poll(() => removalPage.evaluate(() => (window as typeof window & { __playerRemovalDone: boolean }).__playerRemovalDone)).toBe(true);
    expect(await removalPage.evaluate(() => (window as typeof window & { __playerRemovalResult: boolean }).__playerRemovalResult)).toBe(false);
    await expect.poll(() => removals).toBe(1);
    expect(ranges).toBe(3);
    expect(await page.evaluate(() => new Promise((resolve, reject) => {
      const request = indexedDB.open("kinosail-offline-v1");
      request.onsuccess = () => {
        const database = request.result;
        const transaction = database.transaction(["jobs", "chunks"], "readonly");
        const jobs = transaction.objectStore("jobs").getAllKeys();
        const chunks = transaction.objectStore("chunks").getAllKeys();
        transaction.oncomplete = () => { database.close(); resolve({ jobs: jobs.result, chunks: chunks.result }); };
        transaction.onerror = () => reject(transaction.error);
      };
      request.onerror = () => reject(request.error);
    }))).toEqual({ jobs: [], chunks: [] });
    releaseRemovalResponse();
    await removalPage.close();
  });
});
