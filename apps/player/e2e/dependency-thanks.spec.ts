import { expect, test } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { login } from "./layout-audit-helpers";

test("Owner can read dependency thanks and notices on phone and desktop", async ({ page }, testInfo) => {
	test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated test instance");
	await login(page);
	for (const width of [390, 1440]) {
		await page.setViewportSize({ width, height: 900 });
		await page.goto("/settings#thanks");
		await page.locator('[data-settings-nav] a[href="#thanks"]').click();
		const thanks = page.locator("#thanks");
		await expect(thanks).toBeVisible();
		await expect(thanks).toContainText("FFmpeg");
		await expect(thanks).toContainText("This product uses the TMDB API but is not endorsed or certified by TMDB.");
		await expect(thanks.getByRole("link", { name: "View third-party notices" })).toHaveAttribute("href", /THIRD_PARTY_NOTICES\.md$/);
		await expect(thanks.getByRole("img", { name: "TMDB" })).toHaveJSProperty("complete", true);
		expect((await new AxeBuilder({ page }).analyze()).violations.map(({ id }) => id)).toEqual([]);
		await thanks.screenshot({ path: testInfo.outputPath(`thanks-${width}.png`) });
		if (width === 1440) {
			await page.evaluate(() => { document.documentElement.dataset.theme = "light"; });
			expect((await new AxeBuilder({ page }).analyze()).violations.map(({ id }) => id)).toEqual([]);
			await thanks.screenshot({ path: testInfo.outputPath("thanks-light-1440.png") });
		}
	}
});
