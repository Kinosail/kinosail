import { expect, test } from "@playwright/test";
import { login } from "./test-instance-helpers";

test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");

test("phone navigation keeps the TV Shows context on show details", async ({ page }, testInfo) => {
	await page.setViewportSize({ width: 390, height: 844 });
	await login(page);
	await page.goto("/?view=shows");
	await page.getByRole("link", { name: "Example Show", exact: true }).first().click();
	const showsTab = page.locator('.mobile-navigation a.nav-personal-tab[href="/?view=shows"]');
	await expect(showsTab).toBeVisible();
	await expect(showsTab).toHaveAttribute("aria-current", "page");
	await expect(page.locator('.mobile-navigation a[aria-current="page"]:visible')).toHaveCount(1);
	await expect(page.locator(".mobile-navigation .nav-more > summary")).not.toHaveClass(/active/);
	await page.screenshot({ path: testInfo.outputPath("390-show-detail-navigation.png") });
	await page.goto("/settings");
	await expect(page.locator(".mobile-navigation .nav-more > summary")).toHaveClass(/active/);
});

test("phone brand link has a full touch target", async ({ page }) => {
	await page.setViewportSize({ width: 390, height: 844 });
	await login(page);
	const brandHeight = await page.locator(".app-header .brand-lockup").evaluate((link) => link.getBoundingClientRect().height);
	expect(brandHeight).toBeGreaterThanOrEqual(44);
});

test("watch page keeps device warnings out of initial playback", async ({ page }, testInfo) => {
	await page.setViewportSize({ width: 390, height: 844 });
	await login(page);
	await page.goto("/?view=movies");
	await page.getByRole("link", { name: "Example Movie", exact: true }).first().click();
	await expect(page.locator("[data-cast-state]")).toBeHidden();
	await expect.poll(() => page.locator("video").evaluate((video) => (video as HTMLVideoElement).readyState)).toBeGreaterThanOrEqual(2);
	await expect(page.locator("[data-player-status]")).toBeHidden();
	await page.screenshot({ path: testInfo.outputPath("390-watch-without-cast-warning.png") });
	await page.setViewportSize({ width: 1440, height: 900 });
	await expect(page.locator("[data-cast-state]")).toBeHidden();
	await page.screenshot({ path: testInfo.outputPath("1440-watch-without-cast-warning.png") });
	await page.setViewportSize({ width: 390, height: 844 });
	await page.getByRole("button", { name: "Play on another device" }).click();
	await expect(page.getByRole("dialog", { name: "Play on another device" })).toBeVisible();
	await page.screenshot({ path: testInfo.outputPath("390-cast-picker.png") });
});
