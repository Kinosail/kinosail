import AxeBuilder from "@axe-core/playwright";
import { chromium, expect, firefox, test, webkit, type Page, type TestInfo } from "@playwright/test";
import { layoutProblems, login, presentationProblems, totp, viewports } from "./layout-audit-helpers";

export function registerLayoutWorkflowTests() {
test("Owner settings search finds a setting across task families", async ({ page }, testInfo) => {
	test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
	await login(page);
	for (const viewport of [{ width: 1440, height: 900 }, { width: 390, height: 844 }, { width: 320, height: 800 }]) {
		await page.setViewportSize(viewport);
		await page.goto("/settings", { waitUntil: "domcontentloaded" });
		const search = page.getByRole("searchbox", { name: "Search settings" });
		await search.fill("certificate");
		await expect(page.locator("[data-settings-search-status]")).toHaveText(/\d+ matching settings\./);
		const trustedHTTPS = page.locator('[data-settings-search-results] a[href="#trusted-https"]');
		await expect(trustedHTTPS).toHaveCount(1);
		await expect(trustedHTTPS.locator("span").first()).not.toHaveText("");
		await expect(trustedHTTPS.locator(".settings-search-result-group")).toHaveText("Access");
		await expect(page.locator("[data-settings-nav]")).toBeVisible();
		await expect(page.locator("#onboarding")).toBeVisible();
		expect(await page.locator('[data-settings-flow]>[data-settings-group]:visible').count()).toBe(await page.locator('[data-settings-flow]>[data-settings-group]').count());
		expect((await new AxeBuilder({ page }).analyze()).violations, `${viewport.width}px settings search accessibility`).toEqual([]);
		expect(await layoutProblems(page), `${viewport.width}px settings search layout`).toEqual({ documentOverflow: 0, outside: [], tinyControls: [], distortedChecks: [], clippedControls: [], overlappingStatuses: [] });
		await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-settings-search-results.png`), fullPage: true });
		await trustedHTTPS.click();
		await expect(page).toHaveURL(/\/settings#trusted-https$/);
		await search.fill("not a real setting");
		await expect(page.locator("[data-settings-search-status]")).toHaveText("No settings match that search.");
		await expect(page.locator("[data-settings-search-results] a")).toHaveCount(0);
		await search.fill("setup");
		await expect(page.locator("[data-settings-search-status]")).toHaveText(/\d+ matching settings\./);
		await expect(page.locator("[data-settings-search-results] a span").first()).toHaveText("Setup guide");
		await expect(page.locator("[data-settings-search-results] a").first()).toHaveAttribute("href", "#onboarding");
		await expect(page.locator("#onboarding")).toBeVisible();
		await search.fill("subtitles");
		await expect(page.locator("[data-settings-search-results] a").first()).toHaveAttribute("href", "#playback");
		await search.fill("not a real setting");
		await search.press("Escape");
		await expect(page.locator("[data-settings-nav]")).toBeVisible();
		await expect(page.locator("#onboarding")).toBeVisible();
		expect(await page.locator('[data-settings-flow]>[data-settings-group]:visible').count()).toBe(await page.locator('[data-settings-flow]>[data-settings-group]').count());
		await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-settings-search-cleared.png`), fullPage: true });
	}
});

test("transcoder support stays concise and accessible", async ({ page }, testInfo) => {
	test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
	await login(page);
	for (const viewport of [{ width: 1440, height: 900 }, { width: 1024, height: 768 }, { width: 390, height: 844 }, { width: 320, height: 800 }]) {
		await page.setViewportSize(viewport);
		await page.goto("/settings#playback", { waitUntil: "domcontentloaded" });
		await expect(page.getByLabel("Video format").locator("option:checked")).toHaveText("Automatic per device (Recommended)");
		await expect(page.locator("#transcoder>p")).toContainText("It tries AV1, HEVC, then VP9");
		await expect(page.locator("#transcoder .automatic-hardware")).toHaveText("Automatic will use: Processor");
		const support = page.locator("#transcoder>.capability-report");
		await expect(support).not.toHaveAttribute("open", "");
		await support.getByText("Video compatibility", { exact: true }).click();
		await expect(support.getByText("VVC / H.266", { exact: false })).toBeVisible();
		await expect(support.getByText("AV2", { exact: true })).toBeVisible();
		await expect(support.getByText("Works with the widest range", { exact: false })).toBeVisible();
		await expect(support.getByText("Built-in Linux video hardware", { exact: true })).toBeVisible();
		await expect(support.getByText("FFmpeg option:", { exact: false }).first()).not.toBeVisible();
		const hardwareDetails = support.locator(".capability-list").nth(1).getByText("Technical details", { exact: true }).first();
		await hardwareDetails.click();
		await expect(hardwareDetails.locator("..").getByText("FFmpeg option:", { exact: false })).toBeVisible();
		await expect(support.getByText("Apple VideoToolbox", { exact: true })).toHaveCount(0);
		expect(await layoutProblems(page), `${viewport.width}px open transcoder support`).toEqual({ documentOverflow: 0, outside: [], tinyControls: [], distortedChecks: [], clippedControls: [], overlappingStatuses: [] });
		expect((await new AxeBuilder({ page }).analyze()).violations, `${viewport.width}px transcoder accessibility`).toEqual([]);
		await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-transcoder-support-open.png`), fullPage: true });
		await hardwareDetails.click();
		await support.getByText("Video compatibility", { exact: true }).click();
		await expect(support).not.toHaveAttribute("open", "");
	}
	await page.emulateMedia({ forcedColors: "active", reducedMotion: "reduce" });
	await page.setViewportSize({ width: 320, height: 800 });
	await page.goto("/settings#playback", { waitUntil: "domcontentloaded" });
	await page.locator("#transcoder>.capability-report").getByText("Video compatibility", { exact: true }).click();
	expect(await layoutProblems(page), "320px forced colors transcoder support").toEqual({ documentOverflow: 0, outside: [], tinyControls: [], distortedChecks: [], clippedControls: [], overlappingStatuses: [] });
	expect((await new AxeBuilder({ page }).analyze()).violations, "320px forced colors transcoder accessibility").toEqual([]);
	await page.screenshot({ path: testInfo.outputPath("320-transcoder-support-forced-colors.png"), fullPage: true });
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

test("representative workflows hold in dark theme with reduced motion", async ({ page }, testInfo) => {
	test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
	test.setTimeout(90_000);
	await page.addInitScript(() => localStorage.setItem("kinosail-theme", "dark"));
	await page.emulateMedia({ reducedMotion: "reduce" });
	await login(page);
	await page.goto("/?view=movies");
	const watch = await page.locator('a.card[href^="/watch/"]').first().getAttribute("href");
	expect(watch).toBeTruthy();
	for (const viewport of [viewports[0], viewports[viewports.length - 1]]) {
		await page.setViewportSize(viewport);
		for (const route of ["/", "/settings", "/settings/configuration", watch!]) {
			const response = await page.goto(route, { waitUntil: "domcontentloaded" });
			expect(response?.ok(), route).toBeTruthy();
			await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
			expect((await new AxeBuilder({ page }).analyze()).violations, `${route} dark accessibility`).toEqual([]);
			expect(await layoutProblems(page), `${route} dark at ${viewport.width}px`).toEqual({ documentOverflow: 0, outside: [], tinyControls: [], distortedChecks: [], clippedControls: [], overlappingStatuses: [] });
			expect(await presentationProblems(page), `${route} dark presentation at ${viewport.width}px`).toEqual({ blurred: [], shadowed: [], overRounded: [], settingsBackdrop: route.startsWith("/settings") ? false : null, oversizedSettingsHeading: false });
			expect(await page.evaluate(() => [...document.querySelectorAll("*")].filter((element) => {
				const style = getComputedStyle(element);
				return (style.animationName && style.animationName !== "none") || style.transitionDuration.split(",").some((duration) => Number.parseFloat(duration) > 0);
			}).map((element) => `${element.tagName.toLowerCase()}.${element.className}: ${getComputedStyle(element).animationName} ${getComputedStyle(element).transitionDuration}`)), `${route} reduced motion`).toEqual([]);
			await page.screenshot({ path: testInfo.outputPath(`dark-${viewport.width}-${route.replace(/[^a-z0-9]+/gi, "-") || "home"}.png`), fullPage: true });
		}
	}
	await page.emulateMedia({ forcedColors: "active", reducedMotion: "reduce" });
	await page.setViewportSize(viewports[viewports.length - 1]);
	for (const route of ["/", "/settings", "/settings/configuration", watch!]) {
		const response = await page.goto(route, { waitUntil: "domcontentloaded" });
		expect(response?.ok(), route).toBeTruthy();
		expect((await new AxeBuilder({ page }).analyze()).violations, `${route} forced-colors accessibility`).toEqual([]);
		expect(await layoutProblems(page), `${route} forced colors`).toEqual({ documentOverflow: 0, outside: [], tinyControls: [], distortedChecks: [], clippedControls: [], overlappingStatuses: [] });
		for (const key of ["Tab", "Tab", "Tab", "Alt+Tab", "Alt+Tab", "Alt+Tab"]) {
			if (await page.evaluate(() => document.activeElement !== document.body)) break;
			await page.keyboard.press(key);
		}
		const focus = await page.evaluate(() => {
			const element = document.activeElement as HTMLElement;
			const style = getComputedStyle(element);
			return {
				interactive: element !== document.body && element !== document.documentElement,
				visible: element.getClientRects().length > 0,
				indicated: element.matches(":focus-visible") || style.outlineStyle !== "none" && Number.parseFloat(style.outlineWidth) > 0 || style.boxShadow !== "none",
			};
		});
		expect(focus, `${route} forced-colors keyboard focus`).toEqual({ interactive: true, visible: true, indicated: true });
		await page.screenshot({ path: testInfo.outputPath(`forced-colors-${route.replace(/[^a-z0-9]+/gi, "-") || "home"}.png`), fullPage: true });
	}
});
}
