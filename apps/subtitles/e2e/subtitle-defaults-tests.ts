import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { expectNoHorizontalOverflow } from "./subtitle-dashboard-helpers";

export function registerSubtitleDefaultTests() {
  test("review starts with sync and cleanup and keeps manual timing exclusive", { tag: "@smoke" }, async ({ page }, testInfo) => {
    const inventory = await page.evaluate(async () => (await fetch("/api/v1/subtitle-library?view=library")).json());
    const item = inventory.items.find((item: { title: string }) => item.title === "Example Movie" || item.title === "Arrival");
    expect(item?.id).toBeTruthy();
    const base = `/api/v1/subtitle-library/${item.id}`;
    const media = process.env.KINOSAIL_E2E_MEDIA_DIR ?? join(process.env.KINOSAIL_TEST_ROOT!, "media", "Movies");
    const path = join(media, process.env.KINOSAIL_E2E_MEDIA_DIR ? "Arrival.en.srt" : "Example Movie.vtt");
    const original = await readFile(path);
    const capture = async (state: string) => {
      for (const viewport of [{ width: 390, height: 844 }, { width: 1440, height: 900 }]) {
        await page.setViewportSize(viewport);
        await expectNoHorizontalOverflow(page);
        expect((await new AxeBuilder({ page }).include("#subtitle-edit-form").analyze()).violations).toEqual([]);
        await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-subtitle-review-${state}.png`), fullPage: true });
      }
    };
    let release!: () => void;
    const held = new Promise<void>(resolve => { release = resolve; });
    await page.route(`**${base}/inspect?*`, async route => { await held; await route.continue(); });
    await page.goto(`/subtitles/inspect/${item.id}?language=en`);
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
  });
}
