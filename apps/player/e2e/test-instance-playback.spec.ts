import {attachNativePlaybackState} from "./native-playback-state.mjs";
import { expect, test } from "@playwright/test";
import { configureTestInstance, firstPlayable, login } from "./test-instance-helpers";

configureTestInstance();

test("@smoke native playback seeks and retains controls on a populated title", async ({ page }, testInfo) => {
  await login(page);
  await page.goto(await firstPlayable(page));
  const video = page.locator("video");
  await expect(video).toHaveJSProperty("controls", true);
  await expect(video).toHaveAttribute("data-native-controls", "");
  await expect.poll(() => video.evaluate((media: HTMLVideoElement) => Number.isFinite(media.duration) && media.duration > 0)).toBeTruthy();
  const seekTo = await video.evaluate((media: HTMLVideoElement) => media.duration / 3);
  await video.evaluate((media: HTMLVideoElement, position) => { media.pause(); media.currentTime = position; }, seekTo);
  await expect.poll(() => video.evaluate((media: HTMLVideoElement) => media.currentTime)).toBeGreaterThanOrEqual(seekTo - 0.1);
  await expect.poll(() => video.evaluate((media: HTMLVideoElement) => media.readyState)).toBeGreaterThanOrEqual(2);
  try {
    await expect.poll(() => video.evaluate((media: HTMLVideoElement, position) =>
      !media.seeking && !media.ended && Number.isFinite(media.currentTime) &&
      Math.abs(media.currentTime - position) <= 0.1 && media.currentTime < media.duration, seekTo)).toBe(true);
    const settled = await video.evaluate((media: HTMLVideoElement) => media.currentTime);
    await attachNativePlaybackState(video, testInfo, "before-play");
    await video.evaluate((media: HTMLVideoElement) => media.play());
    await expect.poll(() => video.evaluate((media: HTMLVideoElement) =>
      !media.paused && !media.ended ? media.currentTime : 0)).toBeGreaterThan(settled);
    await video.dispatchEvent("click");
    await expect(video).toHaveJSProperty("paused", false);
    await attachNativePlaybackState(video, testInfo, "after-click");
  } catch (error) {
    await attachNativePlaybackState(video, testInfo, "failure");
    throw error;
  }
  for (const viewport of [{ width: 390, height: 844 }, { width: 844, height: 390 }, { width: 1440, height: 900 }]) {
    await page.setViewportSize(viewport);
    await expect(page.locator("[data-player-controls]")).toBeHidden();
    await page.screenshot({ path: testInfo.outputPath(`native-player-${viewport.width}.png`), fullPage: true });
  }
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

test("native compatibility playback retains the full movie seek range", async ({page}) => {
  await login(page);
  const watch = await firstPlayable(page);
  await page.goto(`${watch}?compatible=1`);
  const video = page.locator("video");
  await expect(video).toHaveJSProperty("controls", true);
  await expect.poll(() => video.evaluate(media => media.readyState), {timeout: 30_000}).toBeGreaterThanOrEqual(2);
  await page.getByRole("button", {name: "Settings", exact: true}).click();
  const position = page.getByRole("slider", {name: "Movie position", exact: true});
  await expect(position).toBeVisible();
  const duration = Number(await video.getAttribute("data-duration"));
  await expect(position).toHaveAttribute("max", String(duration));
  await position.fill(String(duration * 0.75));
  await expect.poll(() => video.evaluate(media => media.currentTime), {timeout: 30_000}).toBeGreaterThanOrEqual(duration * 0.75 - 1);
});
