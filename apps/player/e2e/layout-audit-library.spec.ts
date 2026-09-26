import { expect, test } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { configureLayoutAudit, login } from "./layout-audit-helpers";

configureLayoutAudit();

test("browse shows artwork before scrolling on desktop and phone", async ({ page }, testInfo) => {
	test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
	await login(page);
	for (const viewport of [{ width: 1640, height: 600 }, { width: 1440, height: 900 }, { width: 390, height: 844 }]) {
		await page.setViewportSize(viewport);
		for (const view of ["shows", "movies"]) {
			await page.goto(`/?view=${view}`, { waitUntil: "domcontentloaded" });
			await expect(page.locator("#library .poster").first()).toBeVisible();
			const layout = await page.evaluate(() => ({
				mastheadHeight: document.querySelector(".library-masthead")!.getBoundingClientRect().height,
				posterTop: document.querySelector("#library .poster")!.getBoundingClientRect().top,
				pageWidth: document.documentElement.scrollWidth,
			}));
			expect(layout.mastheadHeight, `${view} heading at ${viewport.width}px`).toBeLessThan(160);
			expect(layout.posterTop, `${view} artwork at ${viewport.width}px`).toBeLessThan(viewport.height * .8);
			expect(layout.pageWidth, `${view} overflow at ${viewport.width}px`).toBeLessThanOrEqual(viewport.width);
			await page.screenshot({ path: testInfo.outputPath(`${view}-${viewport.width}x${viewport.height}.png`) });
		}
	}
});

test("ordinary browse keeps results in the first useful viewport", async ({ page }) => {
	test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
	await login(page);
	await page.setViewportSize({ width: 390, height: 844 });
	await page.goto("/?view=movies", { waitUntil: "domcontentloaded" });
	await expect(page.locator(".browse-toolbar>span")).toBeVisible();
	const composition = await page.evaluate(() => ({
		masthead: document.querySelector(".browse-masthead")!.getBoundingClientRect().height,
		posterTop: document.querySelector("#library .poster")!.getBoundingClientRect().top,
		navTop: document.querySelector(".app-header nav")!.getBoundingClientRect().top,
	}));
	expect(composition.masthead).toBeLessThan(240);
	expect(composition.posterTop).toBeLessThan(composition.navTop);
	await page.evaluate(() => {
		document.documentElement.style.scrollBehavior = "auto";
		scrollTo(0, document.documentElement.scrollHeight);
	});
	await expect.poll(() => page.evaluate(() => scrollY + innerHeight >= document.documentElement.scrollHeight - 1)).toBeTruthy();
	const dock = await page.locator(".app-header nav").boundingBox();
	const lastControl = await page.locator(".language-picker").boundingBox();
	expect(dock).not.toBeNull();
	expect(lastControl).not.toBeNull();
	expect(lastControl!.y + lastControl!.height).toBeLessThanOrEqual(dock!.y);
});

test("mobile browse spacing holds across every media page", async ({ page }) => {
	test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
	await login(page);
	for (const viewport of [{ width: 390, height: 844 }, { width: 320, height: 800 }]) {
		await page.setViewportSize(viewport);
		for (const view of ["movies", "shows", "music", "audiobooks", "books", "photos"]) {
			await page.goto(`/?view=${view}`, { waitUntil: "domcontentloaded" });
			await expect(page.locator(".browse-toolbar")).toBeVisible();
			const layout = await page.evaluate(() => {
				const intersects = (first: DOMRect, second: DOMRect) => first.left < second.right && first.right > second.left && first.top < second.bottom && first.bottom > second.top;
				const search = document.querySelector(".search")!.getBoundingClientRect();
				const toolbar = document.querySelector(".browse-toolbar")!;
				const grid = document.querySelector(".library-group>.grid")!;
				const toolbarItems = [...toolbar.querySelectorAll(":scope > span, :scope > label, :scope > button")].map((item) => item.getBoundingClientRect());
				const content = [...document.querySelectorAll(".library-shell .card, .library-shell footer")].map((item) => item.getBoundingClientRect());
				return {
					toolbarInside: toolbarItems.every((box) => box.left >= -1 && box.right <= innerWidth + 1),
					countVisible: (toolbar.querySelector(":scope > span") as HTMLElement).getBoundingClientRect().width > 0,
					searchOverlapsContent: content.some((box) => intersects(search, box)),
					gridColumns: getComputedStyle(grid).gridTemplateColumns.split(" ").length,
					firstCardWidth: grid.querySelector(".card")!.getBoundingClientRect().width,
				};
			});
			expect(layout.toolbarInside, `${view} toolbar at ${viewport.width}px`).toBeTruthy();
			expect(layout.countVisible, `${view} count at ${viewport.width}px`).toBe(viewport.width > 360);
			expect(layout.searchOverlapsContent, `${view} search at ${viewport.width}px`).toBeFalsy();
			expect(layout.gridColumns, `${view} columns at ${viewport.width}px`).toBe(2);
			expect(layout.firstCardWidth, `${view} card width at ${viewport.width}px`).toBeLessThan(viewport.width - 40);
		}
	}
});

test("compact landscape shell keeps search and primary actions reachable", async ({ page }, testInfo) => {
	test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
	await login(page);
	await page.setViewportSize({ width: 720, height: 450 });
	for (const view of ["all", "movies", "shows"]) {
		await page.goto(`/?view=${view}`, { waitUntil: "domcontentloaded" });
		const search = page.getByRole("searchbox", { name: "Search library" });
		const actions = page.locator(".header-compact-menu > summary");
		await search.evaluate((element) => (element as HTMLInputElement).blur());
		const compact = await page.evaluate(() => {
			const intersects = (first: DOMRect, second: DOMRect) => first.left < second.right && first.right > second.left && first.top < second.bottom && first.bottom > second.top;
			const searchBox = document.querySelector(".search")!.getBoundingClientRect();
			const actionsBox = document.querySelector(".header-compact-menu > summary")!.getBoundingClientRect();
			const content = [...document.querySelectorAll(".browse-toolbar,.library-shell .card,.library-shell .poster")].map((element) => element.getBoundingClientRect());
			return { width: searchBox.width, overlaps: content.some((box) => intersects(searchBox, box)), actionsOverlap: intersects(searchBox, actionsBox) };
		});
		expect(compact.width, `${view} collapsed search width`).toBeLessThanOrEqual(50);
		expect(compact.overlaps, `${view} collapsed search overlap`).toBeFalsy();
		expect(compact.actionsOverlap, `${view} collapsed search and Actions overlap`).toBeFalsy();
		await expect.poll(() => actions.evaluate((element) => {
			const box = element.getBoundingClientRect();
			const hit = document.elementFromPoint(box.left + box.width / 2, box.top + box.height / 2);
			return hit === element || element.contains(hit);
		}), { message: `${view} collapsed Actions hit target` }).toBeTruthy();
		await search.focus();
		expect((await page.locator(".search").boundingBox())?.width, `${view} focused search width`).toBeGreaterThan(300);
		await expect(actions, `${view} focused Actions visibility`).toBeVisible();
		const focusedOverlap = await page.evaluate(() => {
			const intersects = (first: DOMRect, second: DOMRect) => first.left < second.right && first.right > second.left && first.top < second.bottom && first.bottom > second.top;
			return intersects(document.querySelector(".search")!.getBoundingClientRect(), document.querySelector(".header-compact-menu > summary")!.getBoundingClientRect());
		});
		expect(focusedOverlap, `${view} focused search and Actions overlap`).toBeFalsy();
		await expect.poll(() => actions.evaluate((element) => {
			const box = element.getBoundingClientRect();
			const hit = document.elementFromPoint(box.left + box.width / 2, box.top + box.height / 2);
			return hit === element || element.contains(hit);
		}), { message: `${view} focused Actions hit target` }).toBeTruthy();
		expect(await page.evaluate(() => document.documentElement.scrollWidth - innerWidth), `${view} focused search overflow`).toBe(0);
		await page.screenshot({ path: testInfo.outputPath(`720-${view}-compact-search.png`) });
	}

	await page.goto("/?view=movies", { waitUntil: "domcontentloaded" });
	await page.evaluate(() => {
		document.documentElement.style.setProperty("--safe-left", "44px");
		document.documentElement.style.setProperty("--safe-right", "44px");
	});
	const insetSearch = page.getByRole("searchbox", { name: "Search library" });
	const insetActions = page.locator(".header-compact-menu > summary");
	await insetSearch.evaluate((element) => (element as HTMLInputElement).blur());
	for (const state of ["collapsed", "focused"] as const) {
		if (state === "focused") await insetSearch.focus();
		const geometry = await page.evaluate(() => {
			const search = document.querySelector(".search")!.getBoundingClientRect();
			const actions = document.querySelector(".header-compact-menu > summary")!.getBoundingClientRect();
			return { gap: actions.left - search.right, overflow: document.documentElement.scrollWidth - innerWidth };
		});
		expect(geometry.gap, `${state} search clears a 44px landscape safe area`).toBeGreaterThanOrEqual(8);
		expect(geometry.overflow, `${state} safe-area search overflow`).toBe(0);
		await expect(insetActions, `${state} safe-area Actions visibility`).toBeVisible();
		await expect.poll(() => insetActions.evaluate((element) => {
			const box = element.getBoundingClientRect();
			const hit = document.elementFromPoint(box.left + box.width / 2, box.top + box.height / 2);
			return hit === element || element.contains(hit);
		}), { message: `${state} safe-area Actions hit target` }).toBeTruthy();
	}
	await page.screenshot({ path: testInfo.outputPath("720-movies-compact-search-safe-area.png") });

	for (const [route, selectors] of [
		["/quick-connect", ["main input[data-quick-connect-digit]", "main button", 'main a[href="/"]']],
		["/settings/system", ['main a[href="/settings/diagnostics.json"]', 'main a[href="/settings/metrics"]']],
		["/account", ['main select[name="language"]', 'main form[action="/language"] button', 'main a[href="/"]']],
	] as const) {
		await page.goto(route, { waitUntil: "domcontentloaded" });
		await page.getByRole("searchbox", { name: "Search library" }).evaluate((element) => (element as HTMLInputElement).blur());
		if (route === "/quick-connect") {
			const initial = await page.evaluate(() => {
				const header = document.querySelector(".app-header")!.getBoundingClientRect();
				const form = document.querySelector(".auth > main form")!.getBoundingClientRect();
				return { headerBottom: header.bottom, formTop: form.top, scrollY };
			});
			expect(initial.scrollY, "Quick Connect initial scroll").toBeLessThanOrEqual(10);
			expect(initial.formTop, "Quick Connect initial shell overlap").toBeGreaterThanOrEqual(initial.headerBottom);
		}
		for (const selector of selectors) {
			const target = page.locator(selector).first();
			await target.evaluate((element) => {
				document.documentElement.style.scrollBehavior = "auto";
				element.scrollIntoView({ block: "center" });
			});
			await expect.poll(() => target.evaluate((element) => {
				const box = element.getBoundingClientRect();
				const hit = document.elementFromPoint(box.left + box.width / 2, box.top + box.height / 2);
				return hit === element || element.contains(hit);
			}), { message: `${route} ${selector} action` }).toBeTruthy();
		}
		if (route === "/quick-connect") {
			const shellSearch = await page.locator(".app-header .search").evaluate((element) => {
				const style = getComputedStyle(element);
				return { padding: style.padding, shadow: style.boxShadow, width: element.getBoundingClientRect().width };
			});
			expect(shellSearch).toEqual({ padding: "0px", shadow: "none", width: 48 });
		}
		await page.screenshot({ path: testInfo.outputPath(`720-${route.replace(/[^a-z]+/gi, "-")}.png`) });
	}

	await page.emulateMedia({ forcedColors: "active", reducedMotion: "reduce" });
	await page.goto("/?view=all", { waitUntil: "domcontentloaded" });
	const forcedSearch = page.getByRole("searchbox", { name: "Search library" });
	const forcedBox = await page.locator(".search").boundingBox();
	expect(forcedBox?.width, "forced-colors search width").toBeGreaterThan(300);
	await expect(forcedSearch).toBeVisible();
	expect(await page.evaluate(() => document.documentElement.scrollWidth - innerWidth), "forced-colors overflow").toBe(0);
	expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
	await page.screenshot({ path: testInfo.outputPath("720-forced-colors-search.png") });
	await page.emulateMedia({ forcedColors: "none", reducedMotion: "no-preference" });
});
