import { expect, test } from "@playwright/test";
import { login } from "./subtitle-dashboard-helpers";
import { registerSubtitleLanguageTests } from "./subtitle-dashboard-language-tests";
import { registerSubtitleLayoutTests } from "./subtitle-dashboard-layout-tests";
import { registerSubtitleProviderTests } from "./subtitle-dashboard-provider-tests";

test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated Kinosail Subtitles test instance");
test.describe.configure({ mode: "serial", timeout: 120_000 });
test.beforeEach(async ({ page }) => login(page));

registerSubtitleLanguageTests();
registerSubtitleProviderTests();
registerSubtitleLayoutTests();

test("phone search hint fits beside its submit action", async ({ page }) => {
  for (const width of [320, 390]) {
    await page.setViewportSize({ width, height: 800 });
    for (const view of ["/", "/?view=wanted"]) {
      await page.goto(view);
      const fits = await page.getByRole("searchbox", { name: "Search subtitle library" }).evaluate((input) => {
        const style = getComputedStyle(input);
        const canvas = document.createElement("canvas");
        const context = canvas.getContext("2d")!;
        context.font = style.font;
        const available = input.clientWidth - parseFloat(style.paddingLeft) - parseFloat(style.paddingRight);
        return context.measureText((input as HTMLInputElement).placeholder).width <= available;
      });
      expect(fits, `${width}px ${view} search placeholder`).toBe(true);
    }
  }
});
