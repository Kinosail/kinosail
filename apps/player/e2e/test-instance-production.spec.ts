import { expect } from "@playwright/test";
import { test } from "./test-instance-network";
import AxeBuilder from "@axe-core/playwright";
import { configureTestInstance, login } from "./test-instance-helpers";
import { offlineBrowserAPI, offlineFixture } from "./offline-browser-api.mjs";
import { offlineSeekWitness } from "./offline-seek-witness.mjs";

configureTestInstance();

test("fresh installation controls the populated app and serves its complete shell offline", async ({ page, connection }) => {
  await login(page);
  await page.goto("/");
  await expect.poll(() => page.evaluate(() => navigator.serviceWorker.controller?.state)).toBe("activated");
  expect(await page.evaluate(() => navigator.serviceWorker.controller?.scriptURL)).toContain("/service-worker.js?v=55");
  await connection.disconnect();
  try {
    await page.goto("/offline");
    await expect(page.locator("main")).toBeVisible();
    const font = await page.evaluate(async () => {
      const response = await fetch("/static/manrope.woff2?v=1");
      return { status: response.status, type: response.headers.get("Content-Type"), bytes: (await response.arrayBuffer()).byteLength };
    });
    expect(font.status).toBe(200);
    expect(font.type).toContain("font/woff2");
    expect(font.bytes).toBeGreaterThan(1000);
  } finally { await connection.disconnect(); }
});

test("populated library views stay accessible at desktop and phone sizes", async ({ page, browserName }, testInfo) => {
  // WebKit can report empty inherited variables for memory-cached CSS while
  // rendering the correct colors. Identical fresh CSS produces identical pixels
  // and correct computed styles. Warm-worker/offline flows are tested separately.
  if (browserName === "webkit") await page.route("**/static/app.css*", (route) => route.continue());
  await login(page);
  for (const width of [1440, 390]) {
    await page.setViewportSize({ width, height: 900 });
    for (const view of ["movies", "shows", "music", "audiobooks", "books", "photos"]) {
      await page.goto(`/?view=${view}`);
      await expect(page.locator("main")).toBeVisible();
      await expect(page.locator(".card").first()).toBeVisible();
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBeTruthy();
      expect((await new AxeBuilder({ page }).include("main").analyze()).violations).toEqual([]);
      await page.screenshot({ path: testInfo.outputPath(`${width}-${view}.png`), fullPage: true });
    }
  }
});

test("saved video Compatibility keeps real music and audiobook playback on their pages", async ({ page, baseURL }) => {
  await login(page);
  const library = await offlineBrowserAPI(page, "library", baseURL);
  for (const kind of ["audio", "audiobook"]) {
    const item = offlineFixture(library, kind);
    expect(item).toBeTruthy();
    await page.evaluate(() => localStorage.setItem("kinosail.playback-policy-v2", "compatible"));
    await page.goto(`/watch/${item.id}`);
    const media = page.locator("audio");
    await media.evaluate((audio: HTMLAudioElement) => audio.play());
    await expect.poll(() => media.evaluate((audio: HTMLAudioElement) => audio.readyState)).toBeGreaterThanOrEqual(2);
    await expect.poll(() => media.evaluate((audio: HTMLAudioElement) => audio.currentTime)).toBeGreaterThan(0.2);
    await expect(page).toHaveURL(new RegExp(`/watch/${item.id}$`));
    expect(await page.evaluate(() => localStorage.getItem("kinosail.playback-policy-v2"))).toBe("compatible");
  }
});

test("authenticated video reports its state to Home Assistant", async ({ page, baseURL }) => {
  await login(page);
  await page.goto("/settings");
  try {
    const settings = await offlineBrowserAPI(page, "home-assistant-on", baseURL);
    expect(settings.status).toBe(200);
    const library = await offlineBrowserAPI(page, "library", baseURL);
    const item = offlineFixture(library, "video");
    expect(item).toBeTruthy();
    const report = page.waitForResponse((response) => response.request().method() === "PUT" && response.url().includes("/api/v1/home-assistant/players/"));
    await page.goto(`/watch/${item.id}`);
    expect((await report).status()).toBe(200);
    await expect.poll(async () => (await offlineBrowserAPI(page, "players", baseURL)).some((player: { itemId: string }) => player.itemId === item.id)).toBeTruthy();
  } finally {
    await offlineBrowserAPI(page, "home-assistant-off", baseURL);
  }
});

test("a real offline download plays and seeks after the network disconnects", async ({ page, connection, baseURL }, testInfo) => {
  await login(page);
  const library = await offlineBrowserAPI(page, "library", baseURL);
  const item = offlineFixture(library, "video");
  expect(item).toBeTruthy();
  await page.goto(`/watch/${item.id}`);
  await page.getByText("Playback & downloads", { exact: true }).click();
  await page.getByRole("button", { name: "Prepare 720p offline", exact: true }).click();
  const job = page.locator("[data-download-job]").filter({ has: page.getByRole("heading", { name: "Example Movie", exact: true }) });
  await expect(job.getByText("720p · Ready to download", { exact: true })).toBeVisible({ timeout: 30_000 });
  await job.getByRole("button", { name: "Download to this device", exact: true }).click();
  await expect(job.locator("[data-download-device-status]")).toHaveText("Saved and verified. Play to check compatibility.", { timeout: 30_000 });
  const id = await job.getAttribute("data-download-job");
  const offlineProbe = await page.evaluate(async (id) => {
    const profile = document.querySelector<HTMLElement>("#downloads")?.dataset.viewerProfile;
    const response = await fetch(`/offline-media/${profile}/${id}`, { headers: { Range: "bytes=0-1" } });
    return { status: response.status, length: (await response.arrayBuffer()).byteLength };
  }, id);
  expect(offlineProbe).toEqual({ status: 206, length: 2 });
  const witness = offlineSeekWitness(page, baseURL, id);
  try {
    await connection.disconnect();
    witness.disconnected();
    await page.goto(`/offline?job=${id}`);
    const media = page.locator("video");
    await witness.arm();
    await expect.poll(() => media.evaluate((video: HTMLVideoElement) => video.readyState)).toBeGreaterThanOrEqual(2);
    await media.evaluate((video: HTMLVideoElement) => video.play());
    await expect.poll(() => media.evaluate((video: HTMLVideoElement) => video.currentTime)).toBeGreaterThan(0.2);
    await witness.seek(media);
    await expect.poll(() => media.evaluate((video: HTMLVideoElement) => !video.seeking && video.currentTime >= 4)).toBeTruthy();
    await page.screenshot({ path: testInfo.outputPath("offline-playback.png") });
  } catch (error) {
    await witness.attachFailure(testInfo);
    throw error;
  } finally { await witness.stop(); await connection.disconnect(); }
});

test("video and music open the receiver picker and restore focus", async ({ page, baseURL }, testInfo) => {
  await login(page);
  const library = await offlineBrowserAPI(page, "library", baseURL);
  const video = offlineFixture(library, "video");
  const audio = offlineFixture(library, "audio");
  for (const [kind, item] of [["video", video], ["audio", audio]] as const) {
    expect(item).toBeTruthy();
    for (const width of [1440, 390]) {
      await page.setViewportSize({ width, height: 900 });
      await page.goto(`/watch/${item.id}`);
      const button = page.getByRole("button", { name: "Play on another device", exact: true });
      await page.locator(kind).evaluate((media: HTMLMediaElement) => media.pause());
      await button.click();
      const dialog = page.getByRole("dialog", { name: "Play on another device", exact: true });
      await expect(dialog).toBeVisible();
      if (kind === "audio") {
        await expect(dialog.getByText("HomePod, AirPlay speakers and TVs, or devices offered by your browser.")).toBeVisible();
        await expect(dialog.getByText("Audio-only Cast speakers require a registered receiver app ID in Server configuration.")).toBeVisible();
        await expect(dialog.getByRole("heading", { name: "Screen mirroring / Miracast" })).toHaveCount(0);
      } else {
        await expect(dialog.getByText("Apple TV and AirPlay-enabled TVs, or devices offered by your browser.")).toBeVisible();
        await expect(dialog.getByRole("heading", { name: "Screen mirroring / Miracast" })).toBeVisible();
      }
      await expect(dialog.getByRole("button", { name: "Find DLNA receivers", exact: true })).toBeVisible();
      await expect(dialog.getByRole("button", { name: "Enable Google Cast", exact: true })).toBeVisible();
      await expect(dialog.getByRole("heading", { name: "Samsung and LG TVs", exact: true })).toBeVisible();
      expect((await new AxeBuilder({ page }).include(".tv-picker").analyze()).violations).toEqual([]);
      await page.screenshot({ path: testInfo.outputPath(`${width}-${kind}-receiver-picker.png`) });
      await dialog.getByRole("button", { name: "Close", exact: true }).click();
      await expect(dialog).toBeHidden();
      await expect(button).toBeFocused();
      await button.press("Enter");
      await expect(dialog).toBeVisible();
      await page.keyboard.press("Escape");
      await expect(dialog).toBeHidden();
      await expect(button).toBeFocused();
    }
  }
});
