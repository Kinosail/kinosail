import { expect, test } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { configureLayoutAudit, login } from "./layout-audit-helpers";

configureLayoutAudit();

for (const viewport of [{ width: 320, height: 568 }, { width: 390, height: 844 }]) {
	test(`Quick Connect is visible when More opens at ${viewport.width}px`, async ({ page }) => {
		test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
		await login(page);
		await page.setViewportSize(viewport);
		await page.goto("/", { waitUntil: "domcontentloaded" });
		await expect(page.locator("[data-mobile-tabs]")).toHaveClass(/has-personal-tabs/);
		await page.locator(".nav-more > summary").click();
		const menu = page.locator(".nav-more-menu");
		const connect = menu.getByRole("link", { name: "Quick Connect", exact: true });
		await expect(connect).toBeInViewport({ ratio: 1 });
		expect(await menu.evaluate(element => element.scrollTop)).toBe(0);
		await expect(menu.getByRole("link", { name: "Music", exact: true })).toHaveAttribute("href", "/?view=music");
		await expect(menu.getByRole("button", { name: "Customize tabs", exact: true })).toBeAttached();
		await connect.click();
		await expect(page).toHaveURL(/\/quick-connect$/);
		await expect(page.getByRole("heading", { name: "Connect a device", exact: true })).toBeVisible();
	});
}


test("Quick Connect actions remain readable and reachable in both themes @smoke", async ({ page }, testInfo) => {
	test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
	await login(page);
	for (const theme of ["light", "dark"]) {
		await page.goto("/settings");
		await page.getByRole("link", { name: "Appearance & language", exact: true }).click();
		await page.getByRole("radio", { name: theme === "light" ? "Light" : "Dark", exact: true }).check();
		for (const viewport of [{ width: 720, height: 450 }, { width: 390, height: 844 }, { width: 1440, height: 900 }, { width: 1920, height: 1080 }]) {
			await page.setViewportSize(viewport);
			await page.goto("/quick-connect", { waitUntil: "domcontentloaded" });
			await expect(page.locator("html")).toHaveAttribute("data-theme", theme);
			await expect(page.getByRole("button", { name: "Scan QR code", exact: true })).toBeVisible();
			expect((await new AxeBuilder({ page }).include("main").analyze()).violations, `${theme} at ${viewport.width}px`).toEqual([]);
			for (const control of [page.locator("[data-quick-connect-digit]").first(), page.getByRole("button", { name: "Scan QR code", exact: true }), page.getByRole("button", { name: "Authorize device", exact: true })]) {
				await control.click({ trial: true });
			}
			await page.screenshot({ path: testInfo.outputPath(`${theme}-${viewport.width}-quick-connect.png`) });
		}
	}
});
