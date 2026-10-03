import { readFile } from "node:fs/promises";
import { expect, test } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { fixtureDocument } from "../../../scripts/testing/fixture-document";

const fixtureDir = process.env.KINOSAIL_UI_FIXTURE_DIR;
const css = (await Promise.all([
	"../../../packages/webassets/static/player-app.css",
	"../../../packages/webassets/static/last-light.css",
	"../internal/server/static/home.css",
].map(path => readFile(new URL(path, import.meta.url), "utf8")))).join("\n");

for (const width of [320, 390, 720, 1024, 1440, 1920]) {
	test(`show actions stay compact with long episode titles at ${width}px`, { tag: "@smoke" }, async ({ page }, testInfo) => {
		test.skip(!fixtureDir, "requires TestWriteShowActionFixtures exports");
		await page.setViewportSize({ width, height: 900 });
		for (const state of ["long", "unbroken", "watched", "empty"]) {
			const source = await readFile(`${fixtureDir}/show-action-${state}.html`, "utf8");
			await page.setContent(await page.evaluate(fixtureDocument, { source, css, dark: true }));
			if (state === "empty") {
				await expect(page.locator(".hero-actions")).toHaveCount(0);
				continue;
			}
			const play = page.locator(".hero-actions .hero-action");
			await expect(play).toHaveText(state === "watched" ? "Play again" : "Play next");
			await expect(play).toHaveAttribute("href", "/watch/episode-1");
			await expect(play).toHaveAttribute("aria-label", new RegExp(`^${state === "watched" ? "Play again" : "Play next"} · `));
			const geometry = await play.evaluate(element => {
				const box = element.getBoundingClientRect();
				const sibling = element.nextElementSibling!.getBoundingClientRect();
				const hero = element.closest<HTMLElement>(".media-hero")!;
				return { width: box.width, height: box.height, siblingHeight: sibling.height,
					overflow: hero.scrollWidth - hero.clientWidth };
			});
			expect(geometry.overflow, state).toBe(0);
			expect(geometry.height, state).toBeGreaterThanOrEqual(44);
			expect(geometry.height, state).toBeLessThanOrEqual(60);
			expect(geometry.width, state).toBeLessThan(200);
			expect(geometry.siblingHeight, state).toBeCloseTo(geometry.height, 0);
			if (state === "long") {
				await play.focus();
				await expect(play).toBeFocused();
				expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
				await page.screenshot({ path: testInfo.outputPath(`show-actions-${width}.png`), fullPage: true });
				await page.getByRole("link", { name: "Jump to seasons" }).click();
				expect(await page.evaluate(() => location.hash)).toBe("#seasons");
			}
		}
	});
}
