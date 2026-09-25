import { expect, test } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { configureTestInstance, login } from "./test-instance-helpers";

configureTestInstance();

async function prepareContinuedMovie(page: import("@playwright/test").Page) {
	await login(page);
	await page.goto("/");
	await expect(page.locator('meta[name="kinosail-csrf"]')).toHaveAttribute("content", /.+/);
	await page.evaluate(async () => {
		const csrf = document.querySelector<HTMLMetaElement>('meta[name="kinosail-csrf"]')!.content;
		const response = await fetch("/api/v1/library?view=all");
		if (!response.ok) throw new Error(`library failed: ${response.status}`);
		const catalog = await response.json() as { items: Array<{ id: string; title: string }> };
		const item = catalog.items.find((candidate) => candidate.title === "Example Movie");
		if (!item) throw new Error("Example Movie fixture is missing");
		const saved = await fetch(`/api/v1/items/${item.id}/progress`, {
			method: "PUT", headers: { "Content-Type": "application/json", "X-Kinosail-CSRF": csrf },
			body: JSON.stringify({ seconds: 1, watched: false }),
		});
		if (!saved.ok) throw new Error(`progress failed: ${saved.status}`);
	});
}

test("Home keeps featured resume and navigation reachable", async ({ page }, testInfo) => {
	await prepareContinuedMovie(page);
	for (const viewport of [
		{ width: 1920, height: 1080 }, { width: 1440, height: 900 },
		{ width: 1024, height: 768 }, { width: 901, height: 768 },
		{ width: 900, height: 768 }, { width: 390, height: 844 }, { width: 320, height: 800 },
	]) {
		await page.setViewportSize(viewport);
		await page.goto("/");
		const feature = page.getByRole("region", { name: "Featured title" });
		await expect(feature.getByRole("heading", { name: "Example Movie" })).toBeVisible();
		await expect(feature.getByRole("progressbar", { name: "Watch progress" })).toBeVisible();
		const resume = feature.getByRole("link", { name: "Resume", exact: true });
		await expect(resume).toHaveAttribute("href", /^\/watch\//);
		await expect(feature.getByRole("link", { name: "View details" })).toHaveAttribute("href", /^\/item\//);
		const removal = feature.getByRole("button", { name: /Remove Example Movie/ });
		expect((await removal.boundingBox())!.height).toBeGreaterThanOrEqual(44);
		const geometry = await page.evaluate(() => {
			const nav = document.querySelector(".app-header nav")!.getBoundingClientRect();
			const search = document.querySelector(".app-header .search")!.getBoundingClientRect();
			const featured = document.querySelector(".home-feature")!.getBoundingClientRect();
			const overlaps = nav.left < search.right && nav.right > search.left && nav.top < search.bottom && nav.bottom > search.top;
			const clipped = [...document.querySelectorAll<HTMLElement>(".app-header nav > a, .app-header nav > details")]
				.filter((element) => getComputedStyle(element).display !== "none")
				.some((element) => { const box = element.getBoundingClientRect(); return box.left < -1 || box.right > innerWidth + 1; });
			return { overlaps, clipped, featureTop: featured.top, documentWidth: document.documentElement.scrollWidth };
		});
		expect(geometry.overlaps, `navigation overlaps Search at ${viewport.width}px`).toBe(false);
		expect(geometry.clipped, `navigation is clipped at ${viewport.width}px`).toBe(false);
		expect(geometry.featureTop, `feature is buried at ${viewport.width}px`).toBeLessThan(viewport.height * 0.8);
		expect(geometry.documentWidth, `horizontal overflow at ${viewport.width}px`).toBeLessThanOrEqual(viewport.width);
		await resume.scrollIntoViewIfNeeded();
		await expect(resume).toBeInViewport();
		await resume.focus();
		await expect(resume).toHaveCSS("outline-style", "solid");
		await feature.getByRole("heading", { name: "Example Movie" }).evaluate((heading) => {
			heading.textContent = "A very long continued episode title with an extended translated series name and additional episode information";
		});
		expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(viewport.width);
		await page.screenshot({ path: testInfo.outputPath(`home-${viewport.width}.png`) });
	}
});

test("Home resume remains accessible across appearance and input preferences", async ({ page }, testInfo) => {
	await prepareContinuedMovie(page);
	for (const theme of ["dark", "light"]) {
		for (const width of [1440, 390, 320]) {
			await page.setViewportSize({ width, height: 900 });
			await page.goto("/");
			await page.evaluate((appearance) => { document.documentElement.dataset.theme = appearance; }, theme);
			await page.evaluate(async () => {
				getComputedStyle(document.body).color;
				await Promise.all(document.getAnimations().filter((animation) => animation instanceof CSSTransition).map((animation) => animation.finished.catch(() => {})));
			});
			expect((await new AxeBuilder({ page }).analyze()).violations, `${theme} home at ${width}px`).toEqual([]);
			await page.screenshot({ path: testInfo.outputPath(`home-${theme}-${width}.png`) });
		}
	}
	await page.setViewportSize({ width: 320, height: 800 });
	for (const language of ["en", "de"]) {
		await page.goto(`/?lang=${language}`);
		await page.evaluate(() => { document.documentElement.style.fontSize = "200%"; });
		const resume = page.locator('.home-feature a[href^="/watch/"]');
		await expect(resume).toBeVisible();
		expect(await page.evaluate(() => document.documentElement.scrollWidth), `${language} 200% text reflow`).toBeLessThanOrEqual(320);
		await page.screenshot({ path: testInfo.outputPath(`home-large-text-${language}-320.png`) });
	}
	await page.evaluate(() => { document.documentElement.style.fontSize = ""; });
	await page.emulateMedia({ forcedColors: "active", reducedMotion: "reduce" });
	const resume = page.locator('.home-feature a[href^="/watch/"]');
	await resume.focus();
	await expect(resume).toBeFocused();
	await expect(resume).toHaveCSS("outline-style", "solid");
	expect((await new AxeBuilder({ page }).analyze()).violations, "forced colors home").toEqual([]);
	await page.screenshot({ path: testInfo.outputPath("home-forced-colors-320.png") });
});

test("Show seasons stay below the desktop header while scrolling", async ({ page }) => {
	await login(page);
	for (const width of [1440, 901]) {
		await page.setViewportSize({ width, height: 768 });
		await page.goto("/?view=shows");
		await page.locator('a.show-details[href^="/show/"]').first().click();
		await page.evaluate(() => scrollTo(0, document.body.scrollHeight));
		const positions = await page.evaluate(() => ({
			header: document.querySelector(".app-header")!.getBoundingClientRect().bottom,
			season: document.querySelector(".season-heading")!.getBoundingClientRect().top,
		}));
		expect(positions.season, `season heading clears the header at ${width}px`).toBeGreaterThanOrEqual(positions.header - 1);
	}
});
