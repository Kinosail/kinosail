import { expect, test } from "@playwright/test";

export function registerPlayerDeviceTests() {
test("reserves movie stage geometry while loading on compact screens", async ({ page }, testInfo) => {
  await page.setViewportSize({width: 390, height: 844});
  const stage = page.locator(".media-stage");
  const loadingBox = await stage.boundingBox();
  await page.screenshot({path: testInfo.outputPath("390-loading-stage.png"), fullPage: true});
  await page.locator("video").evaluate((element) => {
    element.setAttribute("width", "1920");
    element.setAttribute("height", "1080");
    element.dispatchEvent(new Event("loadedmetadata"));
  });
  const loadedBox = await stage.boundingBox();
  expect(loadingBox).not.toBeNull();
  expect(loadedBox).not.toBeNull();
  expect(loadingBox!.height).toBeCloseTo(loadedBox!.height, 0);
  expect(loadedBox!.height / loadedBox!.width).toBeCloseTo(9 / 16, 2);
});

test("theater mode is accessible by control and keyboard", async ({ page }) => {
  const theater = page.getByRole("button", { name: "Theater" });
  await theater.click();
  await expect(page.locator("body")).toHaveClass(/player-theater/);
  await expect(theater).toHaveAttribute("aria-pressed", "true");
  await expect(theater).toHaveAttribute("aria-label", "Exit theater");

  await page.keyboard.press("Escape");
  await expect(page.locator("body")).not.toHaveClass(/player-theater/);
  await page.keyboard.press("t");
  await expect(page.locator("body")).toHaveClass(/player-theater/);
});

test("custom controls expose familiar transport, timeline, volume, and captions", async ({ page }) => {
  const video = page.locator("video");
  await expect(page.locator("[data-player-controls]")).toBeVisible();
  await expect(video).not.toHaveAttribute("controls", "");

  await page.getByRole("button", { name: "Play" }).first().click();
  await expect(page.getByRole("button", { name: "Pause" }).first()).toBeVisible();
  await page.getByRole("button", { name: "Go forward 10 seconds" }).first().click();
  await expect.poll(() => video.evaluate((element: HTMLVideoElement) => element.currentTime)).toBe(30);
  await page.getByRole("button", { name: "Go back 10 seconds" }).first().click();
  await expect.poll(() => video.evaluate((element: HTMLVideoElement) => element.currentTime)).toBe(20);
  await page.locator("[data-player-seek]").fill("55");
  await expect(page.locator("[data-player-time]")).toContainText("0:55 / 1:40");

  await page.getByRole("button", { name: "Mute" }).click();
  await expect(page.getByRole("button", { name: "Unmute" })).toHaveAttribute("aria-pressed", "true");
  await page.getByRole("button", { name: "Subtitles" }).click();
  await expect(page.getByRole("button", { name: "Subtitles" })).toHaveAttribute("aria-pressed", "true");
});

test("video playback speed is selectable and rejects unknown values", async ({ page }, testInfo) => {
  const rate = page.getByRole("combobox", { name: "Playback speed" });
  const video = page.locator("video");
  await page.setViewportSize({width: 390, height: 844});
  await page.getByRole("button", { name: "Settings" }).click();
  await page.screenshot({path: testInfo.outputPath("mobile-playback-speed.png")});
  await rate.selectOption("1.5");
  await page.setViewportSize({width: 1440, height: 900});
  await page.screenshot({path: testInfo.outputPath("desktop-playback-speed.png")});
  expect(await video.evaluate((element: HTMLVideoElement) => element.playbackRate)).toBe(1.5);
  await rate.evaluate((select: HTMLSelectElement) => {
    select.add(new Option("Unknown", "50"));
    select.value = "50";
    select.dispatchEvent(new Event("change"));
  });
  expect(await video.evaluate((element: HTMLVideoElement) => element.playbackRate)).toBe(1.5);
  await video.evaluate((element: HTMLVideoElement) => { element.playbackRate = 2; });
  await expect(rate).toHaveValue("2");
});

test("scrubbing previews the frame before seeking", async ({ page }) => {
  await page.route("**/trickplay/movie/*", route => route.fulfill({
    contentType: "image/svg+xml", body: '<svg xmlns="http://www.w3.org/2000/svg" width="320" height="180"/>',
  }));
  const seek = page.locator("[data-player-seek]");
  await seek.evaluate((input: HTMLInputElement) => {
    input.dataset.trickplay = "https://127.0.0.1:38128/trickplay/movie/{second}";
    input.value = "55";
    input.dispatchEvent(new Event("input"));
  });
  const preview = page.locator("[data-seek-preview]");
  await expect(preview).toBeVisible();
  await expect(preview.locator("[data-preview-time]")).toHaveText("0:55");
  await expect(preview.locator("img")).toBeVisible();
  expect(await page.locator("video").evaluate((media: HTMLVideoElement) => media.currentTime)).toBe(20);
  await seek.dispatchEvent("change");
  await expect(preview).toBeHidden();
  expect(await page.locator("video").evaluate((media: HTMLVideoElement) => media.currentTime)).toBe(55);
});

test("Picture-in-Picture keeps playback alive when the player page hides", async ({ page }, testInfo) => {
  const pip = page.locator("[data-player-pip]");
  await expect(pip).toBeVisible();

  await page.getByRole("button", { name: "Play" }).first().click();
  await pip.click();
  await expect(pip).toHaveAttribute("aria-label", "Exit Picture-in-Picture");
  await expect(pip).toHaveAttribute("aria-pressed", "true");

  await page.evaluate(() => window.dispatchEvent(new Event("pagehide")));
  expect(await page.locator("video").evaluate((video: HTMLVideoElement) => video.paused)).toBe(false);

  await pip.click();
  await expect(pip).toHaveAttribute("aria-label", "Picture-in-Picture");
  await expect(pip).toHaveAttribute("aria-pressed", "false");

  for (const width of [1440, 390, 320]) {
    await page.setViewportSize({width, height: 844});
    const box = await pip.boundingBox();
    expect(box).not.toBeNull();
    expect(box!.x).toBeGreaterThanOrEqual(0);
    expect(box!.x + box!.width).toBeLessThanOrEqual(width);
    await page.screenshot({path: testInfo.outputPath(`picture-in-picture-${width}.png`)});
  }
});

test("WebKit Picture-in-Picture uses native presentation mode", async ({ page }) => {
  const pip = page.locator("[data-player-pip]");
  await expect(pip).toBeVisible();
  await pip.click();
  await expect(pip).toHaveAttribute("aria-label", "Exit Picture-in-Picture");
  await expect(pip).toHaveAttribute("aria-pressed", "true");

  await page.evaluate(() => window.dispatchEvent(new Event("pagehide")));
  expect(await page.locator("video").evaluate((video: HTMLVideoElement) => video.paused)).toBe(true);

  await pip.click();
  await expect(pip).toHaveAttribute("aria-label", "Picture-in-Picture");
  await expect(pip).toHaveAttribute("aria-pressed", "false");
});

test("device playback stays available in the player toolbar on compact screens", async ({ page }, testInfo) => {
  const cast = page.locator("[data-cast]");
  if (testInfo.project.name === "webkit") await page.evaluate(() => {
    const event = new Event("webkitplaybacktargetavailabilitychanged");
    Object.defineProperty(event, "availability", {value: "available"});
    document.querySelector("video")?.dispatchEvent(event);
  });
  await expect(cast).toBeVisible();
  await cast.click();
  await expect(cast).toBeVisible();

  for (const width of [390, 320]) {
    await page.setViewportSize({width, height: 844});
    const box = await cast.boundingBox();
    expect(box).not.toBeNull();
    expect(box!.x).toBeGreaterThanOrEqual(0);
    expect(box!.x + box!.width).toBeLessThanOrEqual(width);
    await page.screenshot({path: testInfo.outputPath(`device-access-${width}.png`)});
  }
});

test("AirPlay playback follows native receiver availability", async ({ page }) => {
  const cast = page.locator("[data-cast]");
  await expect(cast).toBeHidden();
  await page.evaluate(() => {
    const event = new Event("webkitplaybacktargetavailabilitychanged");
    Object.defineProperty(event, "availability", {value: "available"});
    document.querySelector("video")?.dispatchEvent(event);
  });
  await expect(cast).toBeVisible();
  await cast.click();
  await expect.poll(() => page.evaluate(() => (window as Window & {airplayPickerCalls: number}).airplayPickerCalls)).toBe(1);
  await page.evaluate(() => {
    const event = new Event("webkitplaybacktargetavailabilitychanged");
    Object.defineProperty(event, "availability", {value: "not-available"});
    document.querySelector("video")?.dispatchEvent(event);
  });
  await expect(cast).toBeHidden();
});

test("remote playback follows device availability and connection state", async ({ page }) => {
  const cast = page.locator("[data-cast]");
  await expect(cast).toBeHidden();
  await page.evaluate(() => (window as Window & {setRemoteAvailability: (available: boolean) => void}).setRemoteAvailability(true));
  await expect(cast).toBeVisible();
  await cast.click();
  await expect(page.locator("[data-cast-state]")).toContainText("Playing on device");
  await page.evaluate(() => {
    const context = window as Window & {remote: EventTarget};
    context.remote.dispatchEvent(new Event("disconnect"));
  });
  await expect(page.locator("[data-cast-state]")).toContainText("Playing here");
});

for (const width of [390, 320]) test(`custom controls reflow without clipping at ${width}px`, async ({ page }) => {
  await page.setViewportSize({width, height: 844});
  await page.getByRole("button", {name: "Theater"}).click();
  await page.getByRole("button", {name: "Settings"}).click();
  const geometry = await page.locator(".media-stage").evaluate((stage) => {
    const panel = stage.querySelector(".player-settings")!.getBoundingClientRect();
    const row = stage.querySelector(".player-control-row")!;
    return {overflow: row.scrollWidth - row.clientWidth, panelLeft: panel.left, panelRight: panel.right, viewport: innerWidth};
  });
  expect(geometry.overflow).toBeLessThanOrEqual(0);
  expect(geometry.panelLeft).toBeGreaterThanOrEqual(0);
  expect(geometry.panelRight).toBeLessThanOrEqual(geometry.viewport);
});
}
