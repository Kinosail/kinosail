import { expect, test } from "@playwright/test";
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
