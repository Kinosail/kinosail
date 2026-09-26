import { expect, test } from "@playwright/test";
import { createHash } from "node:crypto";
import { configureTestInstance, downloadsSource, firstPlayable, login, type OfflineClient } from "./test-instance-helpers";

configureTestInstance();

test.describe("large offline transfers", () => {

  test("offline download stores and verifies every transfer chunk", async ({ page }) => {
  let chunkScript = false;
  await page.route((url) => url.pathname === "/static/downloads.js", async (route) => {
    const instrumented = downloadsSource.replace("const chunkSize = 8 * 1024 * 1024;", "const chunkSize = 16;");
    if (instrumented === downloadsSource) throw new Error("offline chunk-size seam changed");
    chunkScript = true;
    await route.fulfill({ contentType: "text/javascript", body: instrumented });
  });
  await page.addInitScript(() => {
    const worker = { scriptURL: new URL("/service-worker.js?v=54", location.href).href, state: "activated" };
    Object.defineProperties(navigator.serviceWorker, {
      controller: { configurable: true, get: () => worker },
      getRegistration: { configurable: true, value: async () => ({ active: worker }) },
    });
    Object.defineProperty(Object.getPrototypeOf(navigator.storage), "persist", { configurable: true, value: async () => true });
    Object.defineProperty(navigator.storage, "getDirectory", { configurable: true, value: undefined });
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
  const chunk = 16;
  const size = chunk * 2 + 1;
  const synthetic = Buffer.concat([Buffer.alloc(chunk, 1), Buffer.alloc(chunk, 2), Buffer.alloc(1, 3)]);
  const sha256 = createHash("sha256").update(synthetic).digest("hex");
  const syntheticID = jobID!;
  const ranges: number[] = [];
  let manifests = 0;
  let manifestHash = "0".repeat(64);
  await page.route((url) => url.pathname === `/api/v1/downloads/${jobID}`, (route) => {
    manifests++;
    return route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({ id: syntheticID, itemId: itemID, profileId: profileID, title, quality, state: "ready", extension: ".mp4", sha256: manifestHash, size, readyOffline: true }),
    });
  });
  await page.route((url) => url.pathname === `/api/v1/downloads/${syntheticID}/file`, (route) => {
    const match = route.request().headers().range?.match(/^bytes=(\d+)-(\d+)$/);
    if (!match) return route.abort();
    const start = Number(match[1]);
    const end = Number(match[2]);
    const body = synthetic.subarray(start, end + 1);
    ranges.push(body.length);
    return route.fulfill({
      status: 206,
      body,
      headers: {
        "Content-Digest": `sha-256=:${createHash("sha256").update(body).digest("base64")}:`,
        "Content-Range": `bytes ${start}-${end}/${size}`,
      },
    });
  });

  expect(await page.evaluate(() => navigator.storage.persist())).toBe(true);
  expect(chunkScript).toBe(true);
  await expect(button).toHaveAttribute("data-bound", "true");
  const performanceChunk = 8 * 1024 * 1024;
  const performanceHash = createHash("sha256").update(Buffer.alloc(performanceChunk, 1)).update(Buffer.alloc(performanceChunk, 2)).digest("hex");
  const responsiveness = await page.evaluate(async ({ expected, size }) => {
    const first = new Uint8Array(size).fill(1);
    const second = new Uint8Array(size).fill(2);
    let previous = performance.now();
    let maxLag = 0;
    let ticks = 0;
    const timer = setInterval(() => {
      const now = performance.now();
      maxLag = Math.max(maxLag, now - previous - 10);
      previous = now;
      ticks++;
    }, 10);
    const digest = (window as Window & OfflineClient).streamingOfflineDigest();
    await digest.update(first.buffer);
    await digest.update(second.buffer);
    const actual = await digest.hex();
    digest.close();
    await new Promise((resolve) => setTimeout(resolve, 20));
    clearInterval(timer);
    return { actual, expected, maxLag, ticks };
  }, { expected: performanceHash, size: performanceChunk });
  expect(responsiveness.actual).toBe(responsiveness.expected);
  expect(responsiveness.ticks).toBeGreaterThan(2);
  expect(responsiveness.maxLag).toBeLessThan(100);
  await button.click();
  await expect.poll(() => manifests).toBe(1);
  await expect.poll(() => ranges).toEqual([chunk, chunk, 1]);
  await expect(page.getByText("The download could not be verified", { exact: true })).toBeVisible();
  await expect(button).toBeEnabled();
  manifestHash = sha256;
  await button.click();
  await expect.poll(() => manifests).toBe(2);
  await expect.poll(() => ranges).toEqual([chunk, chunk, 1, chunk, chunk, 1]);
  await expect.poll(() => page.evaluate((id) => (window as Window & OfflineClient).KinosailOfflineMedia.source(id), itemID!), { timeout: 20_000 }).toBe(`/offline-media/${profileID}/${syntheticID}`);
  });

  test("offline download rejects a mismatched manifest before storage changes", async ({ page }) => {
    await page.route((url) => url.pathname === "/static/downloads.js", (route) => route.fulfill({ contentType: "text/javascript", body: downloadsSource }));
    await page.addInitScript(() => {
      const worker = { scriptURL: new URL("/service-worker.js?v=54", location.href).href, state: "activated" };
      Object.defineProperties(navigator.serviceWorker, {
        controller: { configurable: true, get: () => worker },
        getRegistration: { configurable: true, value: async () => ({ active: worker }) },
      });
      Object.defineProperty(window, "__offlinePersistCalls", { configurable: true, writable: true, value: 0 });
      Object.defineProperty(Object.getPrototypeOf(navigator.storage), "persist", {
        configurable: true,
        value: async () => { (window as typeof window & { __offlinePersistCalls: number }).__offlinePersistCalls++; return true; },
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
    const profileID = await page.locator("#downloads").getAttribute("data-viewer-profile");
    expect(jobID && itemID && title && profileID).toBeTruthy();
    let fileRequests = 0;
    await page.route((url) => url.pathname === `/api/v1/downloads/${jobID}`, (route) => route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({ id: jobID, itemId: itemID, profileId: `${profileID}-other`, title, quality: "720p", state: "ready", extension: ".mp4", sha256: "0".repeat(64), size: 1, readyOffline: true }),
    }));
    page.on("request", (request) => { if (/\/api\/v1\/downloads\/[^/]+\/file$/.test(new URL(request.url()).pathname)) fileRequests++; });

    await button.click();
    await expect(page.getByText("The download could not be verified", { exact: true })).toBeVisible();
    expect(await page.evaluate(() => (window as typeof window & { __offlinePersistCalls: number }).__offlinePersistCalls)).toBe(0);
    expect(fileRequests).toBe(0);
    expect(await page.evaluate(() => new Promise((resolve, reject) => {
      const request = indexedDB.open("kinosail-offline-v1", 4);
      request.onsuccess = () => {
        const database = request.result;
        const transaction = database.transaction("jobs", "readonly");
        const jobs = transaction.objectStore("jobs").getAll();
        jobs.onsuccess = () => { database.close(); resolve(jobs.result); };
        jobs.onerror = () => reject(jobs.error);
      };
      request.onerror = () => reject(request.error);
    }))).toEqual([]);
  });

});
