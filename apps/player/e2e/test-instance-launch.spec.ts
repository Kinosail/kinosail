import { chromium, expect } from "@playwright/test";
import { createHash } from "node:crypto";
import { createReadStream } from "node:fs";
import { readFile, readdir, stat, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { login } from "./test-instance-helpers";
import { test } from "./test-instance-network";

// This opt-in journey needs a dedicated generated >2 GiB MP4 and disk space.
// It uses the production transfer, hash worker, OPFS, and service worker unchanged.
// Tracing buffers network bodies; retain small receipts and explicit screenshots.
test.use({ trace: "off", screenshot: "off", video: "off" });
test("real large transfer survives browser restart and plays with the Server disconnected", async ({ connection }, info) => {
  test.skip(process.env.KINOSAIL_LAUNCH_READINESS !== "1", "requires the isolated large-file launch fixture");
  test.skip(info.project.name !== "chromium", "persistent Chromium OPFS proof");
  test.setTimeout(900_000);
  const profile = process.env.KINOSAIL_LAUNCH_PROFILE_DIR!;
  const expected = JSON.parse(await readFile(process.env.KINOSAIL_LAUNCH_SOURCE_RECEIPT!, "utf8"));
  expect(profile).toContain("/qa/");
  expect(expected.bytes).toBeGreaterThan(2 ** 31);
  const ranges: string[] = [];
  let context: Awaited<ReturnType<typeof chromium.launchPersistentContext>> | undefined;
  const reopen = async () => {
    context = await chromium.launchPersistentContext(profile, { baseURL: connection.url, ignoreHTTPSErrors: true,
      args: ["--allow-insecure-localhost", "--ignore-certificate-errors"], viewport: { width: 1280, height: 800 } });
    await context.addInitScript(() => Object.defineProperty(PublicKeyCredential, "isConditionalMediationAvailable", { value: async () => false }));
    const page = await context.newPage();
    page.on("response", response => {
      if (/\/api\/v1\/downloads\/[^/]+\/file$/.test(new URL(response.url()).pathname)) {
        const range = response.headers()["content-range"];
        if (range) ranges.push(range);
      }
    });
    return page;
  };
  type OfflineRecord = { bytes: number; size: number; sha256: string; storage: string; state: string; readyOffline: boolean };
  const record = (page: import("@playwright/test").Page, id: string) => page.evaluate(async id => {
    const db = await new Promise<IDBDatabase>((resolve, reject) => {
      const request = indexedDB.open("kinosail-offline-v1");
      request.onsuccess = () => resolve(request.result); request.onerror = () => reject(request.error);
    });
    try { return await new Promise<OfflineRecord>((resolve, reject) => {
      const request = db.transaction("jobs").objectStore("jobs").get(id);
      request.onsuccess = () => resolve(request.result); request.onerror = () => reject(request.error);
    }); } finally { db.close(); }
  }, id);
  const receipt: {
    revision?: string; command: string; environment: string; testData: string;
    expected: { bytes: number; sha256: string }; result: string; interruptedBytes?: number;
    saved?: Pick<OfflineRecord, "bytes" | "size" | "sha256" | "storage">;
    independentStoredFileHash?: string; rangeRequests?: number; ranges?: string[];
    playback?: { time: number; width: number; height: number; error: string | null };
  } = { revision: process.env.KINOSAIL_TEST_REVISION,
    command: "KINOSAIL_LAUNCH_READINESS=1 pnpm exec playwright test test-instance-launch.spec.ts --project=chromium --workers=1",
    environment: "Nox ARM64 isolated production container, Node 26, Chromium 1.63, persistent browser profile",
    testData: "Generated 12-second MP4 with an ISO free box exceeding 2 GiB; this proves transfer size, not film duration", expected, result: "failed" };
  try {
    let page = await reopen();
    await login(page);
    await page.goto("/offline-downloads");
    const job = await page.evaluate(async () => {
      const csrf = document.querySelector<HTMLMetaElement>('meta[name="kinosail-csrf"]')!.content;
      const library = await fetch("/api/v1/library?view=movies").then(response => response.json());
      const item = library.items.find((item: { title: string }) => item.title === "Launch Large Transfer");
      if (!item) throw new Error("dedicated large fixture is missing");
      const response = await fetch(`/api/v1/items/${item.id}/downloads`, { method: "POST",
        headers: { "Content-Type": "application/json", "X-Kinosail-CSRF": csrf }, body: JSON.stringify({ quality: "original" }) });
      if (response.status !== 202) throw new Error(`prepare original: ${response.status}`);
      return response.json();
    });
    await expect.poll(async () => (await page.request.get(`/api/v1/downloads/${job.id}`)).json(), { timeout: 120_000 })
      .toMatchObject({ state: "ready", size: expected.bytes, sha256: expected.sha256, readyOffline: true });
    await page.reload();
    let article = page.locator(`[data-download-job="${job.id}"]`);
    await article.getByRole("button", { name: "Download to this device", exact: true }).click();
    await expect.poll(async () => (await record(page, job.id))?.bytes ?? 0, { timeout: 120_000 }).toBeGreaterThanOrEqual(64 * 1024 ** 2);
    const partial = await record(page, job.id);
    expect(partial.bytes).toBeLessThan(expected.bytes);
    receipt.interruptedBytes = partial.bytes;
    await context!.close(); context = undefined;

    page = await reopen();
    await login(page);
    await page.goto("/offline-downloads");
    article = page.locator(`[data-download-job="${job.id}"]`);
    await expect(article.getByRole("button", { name: "Resume on this device", exact: true })).toBeVisible();
    expect((await record(page, job.id)).bytes).toBeGreaterThanOrEqual(partial.bytes);
    await article.getByRole("button", { name: "Resume on this device", exact: true }).click();
    await expect(article.locator("[data-download-device-status]")).toContainText("Saved and verified", { timeout: 600_000 });
    const saved = await record(page, job.id);
    expect(saved).toMatchObject({ bytes: expected.bytes, size: expected.bytes, sha256: expected.sha256, storage: "opfs", state: "ready", readyOffline: true });
    receipt.saved = { bytes: saved.bytes, size: saved.size, sha256: saved.sha256, storage: saved.storage };
    expect(ranges.some(range => Number(range.match(/^bytes (\d+)-/)?.[1]) >= 2 ** 31)).toBe(true);
    await page.screenshot({ path: info.outputPath("large-transfer-verified.png") });
    await context!.close(); context = undefined;

    const findStored = async (directory: string): Promise<string | undefined> => {
      for (const entry of await readdir(directory, { withFileTypes: true })) {
        const path = join(directory, entry.name);
        if (entry.isDirectory()) { const found = await findStored(path); if (found) return found; }
        else if (entry.isFile() && (await stat(path)).size === expected.bytes) return path;
      }
    };
    const stored = await findStored(profile);
    expect(stored, "the actual OPFS backing file must exist after browser shutdown").toBeTruthy();
    const digest = createHash("sha256");
    for await (const chunk of createReadStream(stored!)) digest.update(chunk);
    const independentHash = digest.digest("hex");
    expect(independentHash).toBe(expected.sha256);
    receipt.independentStoredFileHash = independentHash;

    page = await reopen();
    await login(page);
    await page.goto("/offline");
    await expect(page.getByRole("link").filter({ hasText: "Launch Large Transfer" })).toBeVisible();
    await connection.disconnect();
    await page.goto(`/offline?job=${job.id}`);
    const media = page.locator("video");
    await expect(media).toBeVisible();
    await expect.poll(() => media.evaluate((video: HTMLVideoElement) => video.readyState), { timeout: 30_000 }).toBeGreaterThanOrEqual(2);
    await media.evaluate(async (video: HTMLVideoElement) => { video.muted = true; await video.play(); });
    await expect.poll(() => media.evaluate((video: HTMLVideoElement) => video.currentTime)).toBeGreaterThan(1);
    await media.evaluate((video: HTMLVideoElement) => { video.currentTime = 7; });
    await expect.poll(() => media.evaluate((video: HTMLVideoElement) => video.currentTime)).toBeGreaterThanOrEqual(7);
    receipt.playback = await media.evaluate((video: HTMLVideoElement) => ({ time: video.currentTime, width: video.videoWidth, height: video.videoHeight, error: video.error?.message ?? null }));
    expect(receipt.playback).toMatchObject({ error: null });
    await page.screenshot({ path: info.outputPath("large-offline-disconnected-playback.png") });
    receipt.rangeRequests = ranges.length; receipt.ranges = ranges; receipt.result = "passed";
  } finally {
    await context?.close();
    const path = info.outputPath("large-transfer-receipt.json");
    await writeFile(path, JSON.stringify(receipt, null, 2));
    await info.attach("large-transfer-receipt", { path, contentType: "application/json" });
  }
});
