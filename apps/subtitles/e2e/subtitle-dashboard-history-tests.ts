import { expect, test } from "@playwright/test";
import { expectNoHorizontalOverflow } from "./subtitle-dashboard-helpers";

export function registerSubtitleHistoryTests() {

test("Subtitle history has clear empty and responsive states", async ({ page }, testInfo) => {
  for (const width of [1440, 390, 320]) {
    await page.setViewportSize({ width, height: width === 1440 ? 900 : 844 });
    await page.goto("/?view=history");
    await expect(page.getByRole("heading", { name: "Subtitle history" })).toBeVisible();
    await expect(page.getByRole("heading", { name: "No subtitle changes yet." })).toBeVisible();
    await expect(page.getByRole("link", { name: "History", exact: true })).toHaveAttribute("aria-current", "page");
    await expect(page.getByRole("link", { name: "Find subtitles" })).toBeVisible();
    await expectNoHorizontalOverflow(page);
    await page.screenshot({ path: testInfo.outputPath(`subtitle-history-${width}.png`) });
  }
});

}
