import { expect, test } from "@playwright/test";
import { configureTestInstance, login } from "./test-instance-helpers";

configureTestInstance();
test.use({ serviceWorkers: "block" });

test("Collections keep poster geometry while artwork loads", async ({ page }, testInfo) => {
	await login(page);
	let release!: () => void;
	const pending = new Promise<void>(resolve => { release = resolve; });
	await page.route("**/art/**", async route => {
		await pending;
		await route.continue();
	});
	await page.goto("/?view=collections", { waitUntil: "domcontentloaded" });
	const card = page.locator(".curation-card").filter({ has: page.locator("img") }).first();
	const poster = card.locator(".curation-poster");
	try {
		await expect(card).toBeVisible();
		await expect(card.locator("strong")).toBeVisible();
		const image = card.locator("img").first();
		await expect.poll(() => image.evaluate((image: HTMLImageElement) => image.complete)).toBe(false);
		const before = await poster.boundingBox();
		expect(before!.height / before!.width).toBeCloseTo(1.5, 1);
		await page.screenshot({ path: testInfo.outputPath("collections-pending.png") });
		release();
		await expect.poll(() => image.evaluate((image: HTMLImageElement) => image.complete && image.naturalWidth > 0)).toBe(true);
		expect(await poster.boundingBox()).toEqual(before);
		await page.screenshot({ path: testInfo.outputPath("collections-loaded.png") });
	} finally { release(); }
});
