import { expect, test } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { configureLayoutAudit, layoutProblems, login, presentationProblems, viewports } from "./layout-audit-helpers";

configureLayoutAudit();

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
