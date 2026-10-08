import { expect, test, type Page } from "@playwright/test";

const origin = process.env.KINOSAIL_LIBRARY_BROWSER_URL;
test.skip(!origin, "requires TestLibraryPaginationBrowserJourney disposable Go Server");
test.use({ serviceWorkers: "block" });
const pageURL = `${origin}/?view=shows&limit=4`;
const continuation = (request: import("@playwright/test").Request) => request.headers()["x-kinosail-library-page"] === "1";

async function loadAll(page: Page) {
	await expect.poll(async () => {
		if (await page.locator("[data-library-next]").count()) await page.locator("[data-library-status]").scrollIntoViewIfNeeded();
		return page.locator("[data-library-status]").textContent();
	}, { timeout: 20_000 }).toBe("All titles are loaded.");
}

// Isolated fault checks: the normal Server does not return overlapping or
// malformed pages. Each modified response starts with its actual Go fragment.
test("overlapping and repeated Show cards deduplicate in existing and new mixed groups", async ({ page }) => {
	const first = await (await page.request.get(pageURL)).text();
	const overlap = first.match(/<article class="card show-card">[\s\S]*?<\/article>/)![0];
	await page.route(`${origin}/?**`, async route => {
		if (!continuation(route.request())) return route.continue();
		const response = await route.fetch();
		const body = await response.text();
		const repeated = body.match(/<article class="card show-card">[\s\S]*?<\/article>/)?.[0] ?? "";
		await route.fulfill({ response, body: body.replace("</div></div>", `${overlap}${repeated}</div></div>`) });
	});
	await page.goto(`${origin}/?q=Pagination&limit=4`, {waitUntil: "commit"});
	await loadAll(page);
	await expect(page.locator("#library .show-card")).toHaveCount(30);
	await expect(page.locator("#library .card")).toHaveCount(36);
	const destinations = await page.locator("#library .show-details").evaluateAll(elements => elements.map(element => element.getAttribute("href")));
	expect(new Set(destinations).size).toBe(30);
});

test("malformed card identity rejects the whole fragment and keyboard retry recovers", async ({ page }, info) => {
	for (const invalid of ["https://invalid.example/watch/fixture", "/watch/fixture?token=private-fixture", "/watch/../show/fixture", "/watch/%2Ffixture", ""]) {
		await page.route(`${origin}/?**`, async route => {
			if (!continuation(route.request())) return route.continue();
			const response = await route.fetch();
			const body = (await response.text()).replace(/(<a class="card" href=")\/watch\/[a-f0-9]+(")/, `$1${invalid}$2`);
			await route.fulfill({ response, body });
		});
		await page.goto(`${origin}/?q=Pagination&limit=4`, {waitUntil: "commit"});
		await page.locator("[data-library-status]").scrollIntoViewIfNeeded();
		await expect(page.getByRole("link", { name: "Retry loading" })).toBeVisible();
		await expect(page.locator("#library .card")).toHaveCount(4);
		await expect(page.locator("[data-library-status]")).toHaveAttribute("data-failure", "invalid_fragment");
		await expect(page.locator("[data-library-status]")).toHaveAttribute("data-request-id", /^[A-Za-z0-9_-]{1,64}$/);
		await expect(page.locator("[data-library-pagination]")).not.toHaveAttribute("aria-busy", "true");
		await page.unroute(`${origin}/?**`);
	}

	await page.getByRole("link", { name: "Retry loading" }).focus();
	await expect(page.getByRole("link", { name: "Retry loading" })).toBeFocused();
	await page.screenshot({ path: info.outputPath("invalid-fragment-keyboard-retry.png"), fullPage: true });
	await page.keyboard.press("Enter");
	await loadAll(page);
	await expect(page.locator("#library .card")).toHaveCount(36);
});

for (const width of [390, 1440, 1920]) {
	test(`pending, failed, loaded and empty pagination stay usable at ${width}px`, async ({ page }, info) => {
		await page.setViewportSize({ width, height: 900 });
		let release!: () => void;
		const held = new Promise<void>(resolve => { release = resolve; });
		let pending = true;
		await page.route(`${origin}/?**`, async route => {
			if (!pending || !continuation(route.request())) return route.continue();
			await held;
			await route.fulfill({ status: 503, headers: { "X-Request-ID": "pagination-503" } });
		});
		await page.goto(pageURL, {waitUntil: "commit"});
		await page.locator("[data-library-status]").scrollIntoViewIfNeeded();
		await expect(page.locator("[data-library-status]")).toHaveText("Loading more titles…");
		await expect(page.locator("[data-library-pagination]")).toHaveAttribute("aria-busy", "true");
		await expect(page.locator("#library .card")).toHaveCount(4);
		await page.screenshot({ path: info.outputPath(`pending-${width}.png`), fullPage: true });
		release();
		const retry = page.getByRole("link", { name: "Retry loading" });
		await expect(retry).toBeVisible();
		await expect(page.locator("[data-library-status]")).toHaveText("Could not load more titles.");
		await expect(page.locator("[data-library-status]")).toHaveAttribute("data-failure", "http");
		await expect(page.locator("[data-library-status]")).toHaveAttribute("data-request-id", "pagination-503");
		await expect(page.locator("[data-library-pagination]")).not.toHaveAttribute("aria-busy", "true");
		await expect(page.locator("#library .card")).toHaveCount(4);
		await page.screenshot({ path: info.outputPath(`failed-${width}.png`), fullPage: true });
		pending = false;
		await retry.focus();
		await page.keyboard.press("Enter");
		await loadAll(page);
		await expect(page.locator("#library .show-card")).toHaveCount(30);
		await page.screenshot({ path: info.outputPath(`loaded-${width}.png`), fullPage: true });
		await page.goto(`${origin}/?q=NoPaginationFixtureMatch`, {waitUntil: "commit"});
		await expect(page.getByRole("heading", { name: "No matching titles." })).toBeVisible();
		await expect(page.locator("[data-library-next]")).toHaveCount(0);
		expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width);
		await page.screenshot({ path: info.outputPath(`empty-${width}.png`), fullPage: true });
	});
}

test("late Show continuation cannot append after the real HTMX search replacement", async ({ page }) => {
	let release!: () => void;
	const held = new Promise<void>(resolve => { release = resolve; });
	let finished!: () => void;
	const handled = new Promise<void>(resolve => { finished = resolve; });
	await page.route(`${origin}/?**`, async route => {
		if (!continuation(route.request())) return route.continue();
		const response = await route.fetch();
		await held;
		try { await route.fulfill({ response }); } catch { /* The replaced request may already be aborted. */ }
		finished();
	});
	await page.goto(pageURL, {waitUntil: "commit"});
	await page.locator("[data-library-status]").scrollIntoViewIfNeeded();
	await expect(page.locator("[data-library-status]")).toHaveText("Loading more titles…");
	await page.locator('form.search input[type="search"]').fill("Pagination Movie");
	await page.locator('form.search input[type="search"]').press("Enter");
	await expect(page).toHaveURL(/q=Pagination(?:\+|%20)Movie/);
	await expect(page.locator("#library .card")).toHaveCount(6);
	release();
	await handled;
	await expect(page.locator("#library .show-card")).toHaveCount(0);
	await expect(page.locator("#library .card")).toHaveCount(6);
	await expect(page.locator("[data-library-status]")).toBeEmpty();
});

test("without IntersectionObserver the Go pagination link remains an ordinary page fallback", async ({ page }) => {
	await page.addInitScript(() => { Reflect.deleteProperty(window, "IntersectionObserver"); });
	await page.goto(pageURL, {waitUntil: "commit"});
	const next = page.getByRole("link", { name: "Load more" });
	await expect(next).toBeVisible();
	await next.focus();
	await page.keyboard.press("Enter");
	await expect(page).toHaveURL(/offset=4/);
	await expect(page.locator("#library .show-card")).toHaveCount(4);
	await expect(page.getByRole("link", { name: "Previous" })).toBeVisible();
});
