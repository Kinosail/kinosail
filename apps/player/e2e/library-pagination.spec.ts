import { expect, test } from "@playwright/test";
import {registerLibraryNavigationControls} from "./library-navigation-controls";
import {gotoPaginationCatalog} from "./library-navigation";

const origin = process.env.KINOSAIL_LIBRARY_BROWSER_URL;
test.skip(!origin, "requires TestLibraryPaginationBrowserJourney disposable Go Server");
test.use({ serviceWorkers: "block" });
registerLibraryNavigationControls(origin);

const showTitles = Array.from({ length: 30 }, (_, position) => `Pagination Show ${String(position + 1).padStart(2, "0")}`);
const movieTitles = Array.from({ length: 6 }, (_, position) => `Pagination Movie ${String(position + 1).padStart(2, "0")}`);

async function loadAll(page: import("@playwright/test").Page) {
	await expect.poll(async () => {
		if (await page.locator("[data-library-next]").count()) await page.locator("[data-library-status]").scrollIntoViewIfNeeded();
		return page.locator("[data-library-status]").textContent();
	}, { timeout: 20_000 }).toBe("All titles are loaded.");
}

for (const width of [390, 1440]) {
	test(`all Shows remain reachable across real Server pages at ${width}px`, async ({ page }, info) => {
		await page.setViewportSize({ width, height: 844 });
		const response = await page.request.get(`${origin}/api/v1/library?view=shows&limit=4`);
		expect(response.status()).toBe(200);
		const api = await response.json();
		expect(api.total).toBe(30);
		expect(api.items).toHaveLength(4);
		await gotoPaginationCatalog(page, `${origin}/?view=shows&limit=4`, info);
		const bundle = await page.locator('script[src^="/static/main.kinosail.bundle.js"]').getAttribute("src");
		expect(new URL(bundle!, origin).searchParams.get("v")).not.toBe("34-htmx4");
		await loadAll(page);
		expect((await page.locator("#library .show-details h2").allTextContents()).sort()).toEqual(showTitles);
		const links = await page.locator("#library .show-details").evaluateAll(elements => elements.map(element => element.getAttribute("href")));
		expect(new Set(links).size).toBe(30);
		for (const link of links) expect(link).toMatch(/^\/show\/[a-f0-9]{16}$/);
		expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width);
		await page.screenshot({ path: info.outputPath(`shows-loaded-${width}.png`), fullPage: true });
	});
}

test("real Server mixed pages keep both Show and Movie cards", async ({ page }, info) => {
	await gotoPaginationCatalog(page, `${origin}/?q=Pagination&limit=4`, info);
	await loadAll(page);
	expect((await page.locator('[data-library-group="shows"] .card h2').allTextContents()).sort()).toEqual(showTitles);
	expect((await page.locator('[data-library-group="movies"] .card h2').allTextContents()).sort()).toEqual(movieTitles);
	await expect(page.locator("#library .card")).toHaveCount(36);
	await page.screenshot({ path: info.outputPath("mixed-loaded.png"), fullPage: true });
});

test("real catalog records remain reachable while an unrelated load resource is pending", async ({page}, info) => {
	let release!: () => void;
	const held = new Promise<void>(resolve => {release = resolve;});
	let requested = false;
	await page.route("**/qa-delayed-resource.png", async route => {
		requested = true;
		await held;
		await route.fulfill({status: 204});
	});
	await page.addInitScript(() => document.addEventListener("DOMContentLoaded", () => {
		const image = new Image();
		image.src = "/qa-delayed-resource.png";
		document.body.append(image);
	}, {once: true}));
	try {
		await page.goto(`${origin}/?q=Pagination&limit=4`, {timeout: 3000, waitUntil: "commit"});
		await loadAll(page);
		await expect.poll(() => requested).toBe(true);
		expect(await page.evaluate(() => document.readyState)).not.toBe("complete");
		expect((await page.locator('[data-library-group="shows"] .card h2').allTextContents()).sort()).toEqual(showTitles);
		expect((await page.locator('[data-library-group="movies"] .card h2').allTextContents()).sort()).toEqual(movieTitles);
		await expect(page.locator("#library .card")).toHaveCount(36);
		await info.attach("pending-load-catalog", {body: JSON.stringify({revision: process.env.KINOSAIL_TEST_REVISION,
			browser: info.project.name, shows: 30, movies: 6, unrelatedResourceHeld: requested, result: "passed"}), contentType: "application/json"});
	} finally {
		release();
		await page.unrouteAll({behavior: "wait"});
	}
});
