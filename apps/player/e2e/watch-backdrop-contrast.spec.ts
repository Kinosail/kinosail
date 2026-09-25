import { expect, test } from "@playwright/test";
import { login } from "./layout-audit-helpers";
import { firstPlayable } from "./test-instance-helpers";

test("watch-page hints stay readable over extreme artwork", async ({ page }) => {
	test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
	await login(page);
	const watch = await firstPlayable(page);
	await page.goto(watch);
	const backdrop = page.locator(".player-page>.media-backdrop");
	await expect(backdrop).toBeVisible();
	const hint = page.locator(".playback-tools.player-disclosure>summary small");
	await page.evaluate(() => {
		const spacer = document.createElement("div");
		spacer.style.height = "200vh";
		spacer.setAttribute("aria-hidden", "true");
		document.body.append(spacer);
		document.documentElement.style.scrollBehavior = "auto";
	});
	for (const [theme, artwork, viewport] of [
		["dark", "white", { width: 1440, height: 900 }],
		["dark", "white", { width: 390, height: 844 }],
		["light", "black", { width: 1440, height: 900 }],
		["light", "black", { width: 390, height: 844 }],
	] as const) {
		await page.setViewportSize(viewport);
		await page.evaluate((value) => { document.documentElement.dataset.theme = value; }, theme);
		await backdrop.evaluate(async (image: HTMLImageElement, color) => {
			image.src = `data:image/svg+xml,${encodeURIComponent(`<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16"><rect width="16" height="16" fill="${color}"/></svg>`)}`;
			await image.decode();
		}, artwork);
		await hint.evaluate((element) => window.scrollTo(0, window.scrollY + element.getBoundingClientRect().top - 110));
		await page.evaluate(() => new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve()))));
		const foreground = await hint.evaluate((element) => getComputedStyle(element).color.match(/\d+(?:\.\d+)?/g)!.slice(0, 3).map(Number));
		const box = await hint.boundingBox();
		expect(box).toBeTruthy();
		expect(box!.y, "hint scrolled over the artwork").toBeGreaterThanOrEqual(80);
		expect(box!.y).toBeLessThanOrEqual(140);
		await hint.evaluate((element: HTMLElement) => { element.style.visibility = "hidden"; });
		const png = (await page.screenshot()).toString("base64");
		await hint.evaluate((element: HTMLElement) => { element.style.removeProperty("visibility"); });
		const background = await page.evaluate(async ({ png, x, y }) => {
			const image = new Image();
			image.src = `data:image/png;base64,${png}`;
			await image.decode();
			const canvas = document.createElement("canvas");
			canvas.width = image.width;
			canvas.height = image.height;
			const context = canvas.getContext("2d")!;
			context.drawImage(image, 0, 0);
			return [...context.getImageData(Math.round(x * devicePixelRatio), Math.round(y * devicePixelRatio), 1, 1).data].slice(0, 3);
		}, { png, x: box!.x + box!.width / 2, y: box!.y + box!.height / 2 });
		const luminance = (rgb: number[]) => rgb.map((channel) => {
			const value = channel / 255;
			return value <= .04045 ? value / 12.92 : ((value + .055) / 1.055) ** 2.4;
		}).reduce((sum, value, index) => sum + value * [.2126, .7152, .0722][index], 0);
		const [lighter, darker] = [luminance(foreground), luminance(background)].sort((a, b) => b - a);
		expect((lighter + .05) / (darker + .05), `${viewport.width}px ${theme} hint contrast over ${artwork} artwork`).toBeGreaterThanOrEqual(4.5);
	}
});
