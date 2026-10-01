import { expect, test } from "@playwright/test";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { fixtureDocument } from "../../../scripts/testing/fixture-document";

for (const theme of ["light", "dark"]) {
	for (const width of [1440, 1024, 720, 390, 320]) {
		test(`badge descriptions stay inside their cells when artwork fails at ${width}px in ${theme}`, { tag: width === 390 && theme === "dark" ? "@smoke" : [] }, async ({ page, request }, testInfo) => {
			const directory = process.env.KINOSAIL_UI_FIXTURE_DIR;
			test.skip(!directory, "requires the production Supporter template fixture");
			await page.setViewportSize({ width, height: 900 });
			const stylesheet = await request.get("/static/app.css");
			expect(stylesheet.ok()).toBeTruthy();
			const source = await readFile(join(directory!, "supporter-populated-active.html"), "utf8");
			const html = await page.evaluate(fixtureDocument, { source, css: await stylesheet.text(), dark: theme === "dark" });
			await page.route("**/static/supporter/badges/**", route => route.abort());
			await page.route("**/qa-supporter", route => route.fulfill({ contentType: "text/html", body: html }));
			await page.goto("/qa-supporter");
			const badges = page.locator(".badge-level");
			await expect(badges).toHaveCount(20);
			await expect.poll(() => badges.locator("img").evaluateAll(images => images.every(image => image instanceof HTMLImageElement && image.complete && image.naturalWidth === 0))).toBe(true);
			const overflow = await badges.evaluateAll(cells => cells.flatMap(cell => {
				const bounds = cell.getBoundingClientRect();
				return [...cell.children].filter(child => {
					const box = child.getBoundingClientRect();
					return box.left < bounds.left - 1 || box.right > bounds.right + 1;
				}).map(child => child.textContent?.trim());
			}));
			await page.screenshot({ path: testInfo.outputPath(`badges-${width}-${theme}-failed-artwork.png`), fullPage: true });
			expect(overflow).toEqual([]);
			expect(await page.evaluate(() => document.documentElement.scrollWidth - innerWidth)).toBeLessThanOrEqual(1);
		});
	}
}
