import { expect, test } from "@playwright/test";

export function registerPlayerControlTests() {
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
}
