import AxeBuilder from "@axe-core/playwright";
import { createHash } from "node:crypto";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { expect, test } from "@playwright/test";
import { createViewer, firstPlayable, login, loginViewer, newViewerPage, openLibrarySection, removeViewer, type OfflineClient } from "./test-instance-helpers";

export function registerTestInstancePlaybackTests() {
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
  await expect(page.getByText("720p · Ready offline", { exact: true })).toBeVisible({ timeout: 30_000 });
  expect(await page.evaluate(() => navigator.storage.estimate())).toEqual({ quota: 1, usage: 1 });
  let fileRequests = 0;
  page.on("request", (request) => { if (/\/api\/v1\/downloads\/[^/]+\/file$/.test(new URL(request.url()).pathname)) fileRequests++; });
  await page.getByRole("button", { name: "Download to this device", exact: true }).click();
  await expect(page.getByText("This device does not have enough storage for this download", { exact: true })).toBeVisible();
  expect(fileRequests).toBe(0);
});

test.describe("large offline transfers", () => {
  test.use({ serviceWorkers: "block" });
  test("offline download stores and verifies every transfer chunk", async ({ page }, testInfo) => {
  let chunkScript = false;
  await page.route((url) => url.pathname === "/static/downloads.js", async (route) => {
    const response = await route.fetch();
    const source = await response.text();
    const instrumented = source.replace("const chunkSize = 8 * 1024 * 1024;", "const chunkSize = 16;");
    if (instrumented === source) throw new Error("offline chunk-size seam changed");
    chunkScript = true;
    await route.fulfill({ response, body: instrumented });
  });
  await page.addInitScript(() => {
    Object.defineProperty(Object.getPrototypeOf(navigator.storage), "persist", { configurable: true, value: async () => true });
    Object.defineProperty(navigator.storage, "getDirectory", { configurable: true, value: undefined });
  });
  await login(page);
  await page.goto(await firstPlayable(page));
  await page.getByText("Playback & downloads", { exact: true }).click();
  await page.getByRole("button", { name: "Prepare 720p offline", exact: true }).click();
  await expect(page.getByText("720p · Ready offline", { exact: true })).toBeVisible({ timeout: 30_000 });

  const button = page.getByRole("button", { name: "Download to this device", exact: true });
  const jobID = await button.getAttribute("data-job-id");
  const itemID = await button.getAttribute("data-item-id");
  const profileID = await page.locator("#downloads").getAttribute("data-viewer-profile");
  expect(jobID && itemID && profileID).toBeTruthy();
  const chunk = 16;
  const size = chunk * 2 + 1;
  const sha256 = createHash("sha256").update(`${testInfo.project.name}-${Date.now()}`).digest("hex");
  const syntheticID = sha256.slice(0, 16);
  const ranges: number[] = [];
  let manifests = 0;
  await page.route((url) => url.pathname === `/api/v1/downloads/${jobID}`, (route) => {
    manifests++;
    return route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({ id: syntheticID, itemId: itemID, profileId: profileID, title: "Example Movie", quality: "720p", state: "ready", extension: "mp4", sha256, size, readyOffline: true }),
    });
  });
  await page.route((url) => url.pathname === `/api/v1/downloads/${syntheticID}/file`, (route) => {
    const match = route.request().headers().range?.match(/^bytes=(\d+)-(\d+)$/);
    if (!match) return route.abort();
    const start = Number(match[1]);
    const end = Number(match[2]);
    const body = Buffer.alloc(end - start + 1, ranges.length + 1);
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
  await button.click();
  await expect.poll(() => manifests).toBe(1);
  await expect.poll(() => ranges).toEqual([chunk, chunk, 1]);
  await expect.poll(() => page.evaluate((id) => (window as Window & OfflineClient).KinosailOfflineMedia.source(id), itemID!), { timeout: 20_000 }).toBe(`/offline-media/${syntheticID}`);
  });
});

test("adaptive playback starts on Auto and keeps semantic quality choices", async ({ page }) => {
  await login(page);
  await page.getByRole("link", { name: "Movies", exact: true }).click();
  const watch = await page.locator('a.card[href^="/watch/"]').first().getAttribute("href");
  expect(watch).toBeTruthy();
  await page.goto(`${watch}?compatible=1`);

  const quality = page.getByLabel("Stream quality");
  await expect(quality.locator("option")).toHaveText(["Auto", "360p", "Original"], { timeout: 30_000 });
  await page.getByRole("button", { name: "Settings", exact: true }).click();
  await expect(quality).toBeVisible({ timeout: 30_000 });
  await quality.selectOption("level:0");
  await expect.poll(() => page.evaluate(() => localStorage.getItem("kinosail.stream-quality"))).toBe("360p");
  await quality.selectOption("auto");
  await expect.poll(() => page.evaluate(() => localStorage.getItem("kinosail.stream-quality"))).toBe("auto");
  await expect(page.getByRole("status").filter({ hasText: /Auto/ })).toBeVisible();
});

test("Owner can inspect, update, and remove a skip marker", async ({ page }, testInfo) => {
  await login(page);
  await page.getByRole("link", { name: "Movies", exact: true }).click();
  const watch = await page.locator('a.card[href^="/watch/"]').first().getAttribute("href");
  expect(watch).toBeTruthy();
  const id = watch!.split("/").pop();
  expect(id).toBeTruthy();
  await page.goto(watch!);
  await page.getByText("Manage media", { exact: true }).click();
  await page.getByText("Edit skip markers", { exact: true }).click();

  await page.getByText("Add skip marker", { exact: true }).click();
  const saveForms = page.locator(`form[action="/markers/${id}"]`);
  const add = saveForms.last();
  await add.getByLabel("Type").selectOption("recap");
  await add.getByLabel("Start in seconds").fill("1");
  await add.getByLabel("End in seconds").fill("2");
  await add.getByRole("button", { name: "Save marker", exact: true }).click();

  for (const viewport of [{ width: 1440, height: 900 }, { width: 1024, height: 768 }, { width: 390, height: 844 }, { width: 320, height: 720 }]) {
    await page.setViewportSize(viewport);
    await page.goto(watch!);
    await page.getByText("Manage media", { exact: true }).click();
    await page.getByText("Edit skip markers", { exact: true }).click();
    await expect(page.getByText("Add skip marker", { exact: true })).toBeVisible();
    await expect(page.locator(`form[action="/markers/${id}"]`).last()).toBeHidden();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBeTruthy();
    await page.screenshot({ path: testInfo.outputPath(`marker-editor-${viewport.width}.png`), fullPage: true });
  }

  const existing = page.locator(`form[action="/markers/${id}"]:has(option[value="recap"]:checked)`).first();
  await expect(existing.getByLabel("Start in seconds")).toHaveValue("1");
  await expect(existing.getByLabel("End in seconds")).toHaveValue("2");
  await existing.getByLabel("Start in seconds").fill("1.5");
  await existing.getByLabel("End in seconds").fill("2.5");
  await existing.getByRole("button", { name: "Save marker", exact: true }).click();

  await page.getByText("Manage media", { exact: true }).click();
  await page.getByText("Edit skip markers", { exact: true }).click();
  await expect(page.locator(`form[action="/markers/${id}"]:has(option[value="recap"]:checked)`).first().getByLabel("Start in seconds")).toHaveValue("1.5");
  await page.getByRole("button", { name: "Remove Recap marker", exact: true }).click();
  await page.getByText("Manage media", { exact: true }).click();
  await page.getByText("Edit skip markers", { exact: true }).click();
  await expect(page.getByRole("button", { name: "Remove Recap marker", exact: true })).toHaveCount(0);
});
}
