import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { readFile, writeFile, unlink } from "node:fs/promises";
import { join } from "node:path";
import { expectNoHorizontalOverflow, expectSkipLinkOffscreen } from "./subtitle-dashboard-helpers";

export function registerSubtitleDefaultTests() {
  test.describe("subtitle review request states", () => {
    test.use({ serviceWorkers: "block" });
    for (const theme of ["dark", "light"]) test("review starts with sync and cleanup and keeps manual timing exclusive (" + theme + ")", { tag: "@smoke" }, async ({ page }, testInfo) => {
      await page.addInitScript(theme => {
        localStorage.setItem("kinosail-theme", theme);
        const receipts: {colors: string[]}[] = [];
        Object.defineProperty(window, "subtitleEnabledTransitions", {value: receipts});
        new MutationObserver(records => {
          for (const record of records) {
            const button = record.target;
            if (record.oldValue === null || !(button instanceof HTMLButtonElement) || button.disabled ||
                !button.matches('#subtitle-edit-form button[type="submit"]')) continue;
            // Hold real CSS transitions at an early mixed-color frame, when the
            // newly enabled label must already be readable. No styles are replaced.
            getComputedStyle(button).backgroundColor;
            const colors: string[] = [];
            for (const animation of button.getAnimations()) {
              if (!(animation instanceof CSSTransition) || !["color", "background-color"].includes(animation.transitionProperty)) continue;
              colors.push(animation.transitionProperty);
              animation.pause();
              animation.currentTime = Number(animation.effect!.getTiming().duration) * 0.1;
            }
            receipts.push({colors});
          }
        }).observe(document, {subtree: true, attributes: true, attributeFilter: ["disabled"], attributeOldValue: true});
      }, theme);
      const inventory = await page.evaluate(async () => (await fetch("/api/v1/subtitle-library?view=library")).json());
      const filename = process.env.KINOSAIL_E2E_MEDIA_DIR ? "Arrival.mkv" : "Example Movie.mp4";
      const item = inventory.items.find((item: { file: string }) => item.file.endsWith(filename));
      expect(item?.id).toBeTruthy();
      const base = `/api/v1/subtitle-library/${item.id}`;
      const media = process.env.KINOSAIL_E2E_MEDIA_DIR ?? join(process.env.KINOSAIL_TEST_ROOT!, "media", "Movies");
      const containerMedia = process.env.KINOSAIL_E2E_MEDIA_DIR;
      const path = join(media, containerMedia ? "Arrival.en.srt" : "Example Movie.en.srt");
      const original = await readFile(containerMedia ? path : join(media, "Example Movie.vtt"));
      if (!containerMedia) await writeFile(path, original);
      try {
        const capture = async (state: string) => {
          for (const viewport of [{ width: 390, height: 844 }, { width: 1440, height: 900 }]) {
            await page.setViewportSize(viewport);
            await expectNoHorizontalOverflow(page);
            const transitions = await page.evaluate(() => (window as typeof window & {subtitleEnabledTransitions: {colors: string[]}[]}).subtitleEnabledTransitions);
            if (state === "loaded") expect(transitions.length, "The native pending-to-enabled transition must be observed").toBeGreaterThan(0);
            await testInfo.attach("subtitle-enabled-button-" + viewport.width + "-" + state, {
              body: JSON.stringify({revision: process.env.KINOSAIL_TEST_REVISION, theme, state, transitions,
                boundary: "Actual CSS transitions paused at an early frame; real Server review state and unmodified colors"}),
              contentType: "application/json"});
            expect((await new AxeBuilder({ page }).include("#subtitle-edit-form").analyze()).violations).toEqual([]);
            await expectSkipLinkOffscreen(page);
            await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-subtitle-review-${state}.png`), fullPage: true });
          }
        };
        let release!: () => void;
        let heldRequests = 0;
        const held = new Promise<void>(resolve => { release = resolve; });
        await page.route(`**${base}/inspect?*`, async route => { heldRequests++; await held; await route.continue(); });
        await page.goto(`/subtitles/inspect/${item.id}?language=en`);
        await expect(page.locator("html")).toHaveAttribute("data-theme", theme);
        await expect.poll(() => heldRequests).toBe(1);
        const sync = page.getByRole("checkbox", { name: "Synchronize against the video's main dialogue" });
        await expect(sync).toBeChecked();
        await expect(page.locator("#inspector-status")).toHaveAttribute("aria-busy", "true");
        await expect(page.getByRole("button", { name: "Save reviewed subtitle" })).toBeDisabled();
        await capture("pending");
        release();
        await expect(page.locator("#inspector-status")).toContainText("Current subtitle loaded");
        await page.unroute(`**${base}/inspect?*`);
        await page.getByText("Timing anchors and cleanup", { exact: true }).click();
        const credits = page.getByRole("checkbox", { name: "Remove subtitle credits at the beginning and end" });
        const repeats = page.getByRole("checkbox", { name: "Merge adjacent repeated cues" });
        await expect(credits).toBeChecked();
        await expect(repeats).toBeChecked();
        await capture("loaded");
        const preview = page.getByRole("button", { name: "Preview changes" });
        const failed = page.waitForResponse(response => response.url().endsWith(`${base}/preview`));
        await preview.click();
        expect((await failed).ok()).toBe(false);
        await expect(page.getByRole("button", { name: "Save reviewed subtitle" })).toBeDisabled();
        expect(await readFile(path)).toEqual(original);
        await capture("failed");
        const shift = page.getByRole("spinbutton", { name: "Shift all cues (seconds)" });
        await shift.fill("0.5");
        await expect(sync).not.toBeChecked();
        await page.getByRole("button", { name: "Add timing anchor" }).click();
        await expect(shift).toHaveValue("0");
        await expect(sync).not.toBeChecked();
        await page.getByRole("spinbutton", { name: "Subtitle time (seconds)" }).fill("1");
        await page.getByRole("spinbutton", { name: "Correct video time (seconds)" }).fill("1.5");
        await sync.check();
        await expect(page.locator(".subtitle-anchor")).toHaveCount(0);
        await shift.fill("0.5");
        const sent = page.waitForRequest(request => request.url().endsWith(`${base}/preview`));
        await preview.click();
        expect((await sent).postDataJSON()).toMatchObject({ automaticSync: false, removeCredits: true, mergeRepeated: true, offsetMilliseconds: 500 });
        await expect(page.locator("#inspector-status")).toContainText("Preview ready");
        await expect(page.getByRole("button", { name: "Save reviewed subtitle" })).toBeEnabled();
        expect(await readFile(path)).toEqual(original);
        await credits.uncheck();
        await repeats.uncheck();
        await expect(page.getByRole("button", { name: "Save reviewed subtitle" })).toBeDisabled();
        await page.goto(`/subtitles/inspect/${item.id}?language=fr`);
        await expect(page.locator("#inspector-status")).toContainText("Choose a subtitle file to begin");
        await expect(sync).toBeChecked();
        await capture("empty");
      } finally {
        if (!containerMedia) await unlink(path);
      }
    });
  });
}
