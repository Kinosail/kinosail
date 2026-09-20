import { expect, test } from "@playwright/test";
import { installPlayerExperienceFixture } from "./player-experience-fixture";

installPlayerExperienceFixture();

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
  await page.evaluate(() => {
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
  await expect(cast).toBeVisible();
  await expect(cast).toBeDisabled();
  await page.evaluate(() => {
    const event = new Event("webkitplaybacktargetavailabilitychanged");
    Object.defineProperty(event, "availability", {value: "available"});
    document.querySelector("video")?.dispatchEvent(event);
  });
  await expect(cast).toBeVisible();
  await expect(cast).toBeEnabled();
  await cast.click();
  await expect.poll(() => page.evaluate(() => (window as Window & {airplayPickerCalls: number}).airplayPickerCalls)).toBe(1);
  await page.evaluate(() => {
    const event = new Event("webkitplaybacktargetavailabilitychanged");
    Object.defineProperty(event, "availability", {value: "not-available"});
    document.querySelector("video")?.dispatchEvent(event);
  });
  await expect(cast).toBeVisible();
  await expect(cast).toBeDisabled();
});

test("AirPlay stays disabled without the availability constructor until a receiver appears", async ({ page }) => {
  const cast = page.locator("[data-cast]");
  expect(await page.locator("video").evaluate((video) => ({
    remote: (video as HTMLVideoElement & {remote?: EventTarget}).remote ?? null,
    availabilityConstructor: typeof (window as Window & {WebKitPlaybackTargetAvailabilityEvent?: typeof Event}).WebKitPlaybackTargetAvailabilityEvent,
  }))).toEqual({remote: null, availabilityConstructor: "undefined"});
  await expect(cast).toBeVisible();
  await expect(cast).toBeDisabled();
  await page.evaluate(() => {
    const event = new Event("webkitplaybacktargetavailabilitychanged");
    Object.defineProperty(event, "availability", {value: "available"});
    document.querySelector("video")?.dispatchEvent(event);
  });
  await expect(cast).toBeVisible();
});

test("remote playback follows device availability and connection state", async ({ page }) => {
  const cast = page.locator("[data-cast]");
  await expect(cast).toBeVisible();
  await expect(cast).toBeDisabled();
  await page.evaluate(() => (window as Window & {setRemoteAvailability: (available: boolean) => void}).setRemoteAvailability(true));
  await expect(cast).toBeVisible();
  await expect(cast).toBeEnabled();
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

test("compact theater settings mask the primary player actions", async ({ page }, testInfo) => {
  await page.setViewportSize({width: 390, height: 844});
  await page.evaluate(() => document.documentElement.dataset.theme = "dark");
  await page.locator(".media-stage").evaluate((stage) => stage.insertAdjacentHTML("afterend", `
    <div class="primary-player-actions">
      <div class="primary-action-row">
        <form><button>Remove from My List</button></form>
        <form><button class="quiet">Mark watched</button></form>
      </div>
    </div>
  `));
  await page.getByRole("button", {name: "Theater"}).click();
  await page.getByRole("button", {name: "Settings"}).click();
  const overlap = await page.locator(".player-settings").evaluate((panel) => {
    const settings = panel.getBoundingClientRect();
    const actions = document.querySelector(".primary-player-actions")!.getBoundingClientRect();
    const background = getComputedStyle(panel).backgroundColor;
    return {settingsBottom: settings.bottom, actionsTop: actions.top, opaque: !background.startsWith("rgba")};
  });
  expect(overlap.settingsBottom).toBeGreaterThan(overlap.actionsTop);
  expect(overlap.opaque).toBeTruthy();
  await page.screenshot({path: testInfo.outputPath("390-theater-settings.png")});
});

test("chapters expose chronological timestamps and follow playback position", async ({ page }) => {
  await page.locator(".chapters > summary").click();
  const first = page.getByRole("button", { name: "First contact 0:00" });
  const second = page.getByRole("button", { name: "The answer 1:00" });
  await expect(first).toHaveAttribute("aria-current", "true");
  await second.click();
  await expect(second).toHaveAttribute("aria-current", "true");
  await expect.poll(() => page.locator("video").evaluate((video: HTMLVideoElement) => video.currentTime)).toBe(60);
});

test("chapter count does not overlap the disclosure affordance", async ({ page }) => {
  const layout = await page.locator(".chapters > summary").evaluate((summary) => ({
    columns: getComputedStyle(summary).gridTemplateColumns,
    countColumn: getComputedStyle(summary.querySelector("small")!).gridColumn,
    affordanceColumn: getComputedStyle(summary, "::after").gridColumn,
  }));
  expect(layout.columns.split(" ")).toHaveLength(3);
  expect(layout.countColumn).not.toBe(layout.affordanceColumn);
});

test("playback settings open, retain focus, and close with Escape", async ({ page }) => {
  const settings = page.getByRole("button", { name: "Settings" });
  await settings.click();
  await expect(settings).toHaveAttribute("aria-expanded", "true");
  await expect(page.locator(".player-settings")).toBeVisible();
  await expect(page.getByRole("button", { name: "Close" })).toBeFocused();
  await page.locator("[data-subtitles]").selectOption("0");
  await expect.poll(() => page.evaluate(() => (window as Window & { textTrack: { mode: string } }).textTrack.mode)).toBe("showing");
  await page.keyboard.press("Escape");
  await expect(page.locator(".player-settings")).toBeHidden();
  await expect(settings).toBeFocused();
});

test("theater control gets out of the way during playback", async ({ page }) => {
  const stage = page.locator(".media-stage");
  const theater = page.getByRole("button", { name: "Theater" });
  await expect(theater).toBeVisible();

  await page.evaluate(() => Object.defineProperty(document.querySelector("video"), "paused", { value: false, configurable: true }));
  await page.locator("video").dispatchEvent("playing");
  await expect(page.locator("[data-player-controls]")).toHaveCSS("opacity", "0", { timeout: 3000 });
  await stage.dispatchEvent("pointermove");
  await expect(theater).toBeVisible();
  await expect(page.locator("[data-player-controls]")).toHaveCSS("opacity", "0", { timeout: 3000 });
  await stage.dispatchEvent("pointerdown");
  await theater.focus();
  await page.waitForTimeout(2000);
  await expect(theater).toBeVisible();
  await theater.blur();
  await expect(page.locator("[data-player-controls]")).toHaveCSS("opacity", "0", { timeout: 3000 });

  await page.evaluate(() => Object.defineProperty(document.querySelector("video"), "paused", { value: true, configurable: true }));
  await page.locator("video").dispatchEvent("pause");
  await expect(theater).toBeVisible();
});
