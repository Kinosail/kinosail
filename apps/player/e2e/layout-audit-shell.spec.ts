import { expect, test } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { configureLayoutAudit, applicationShellProblems, layoutProblems, login, viewports } from "./layout-audit-helpers";

configureLayoutAudit();

test("signed-in application pages retain the navigation shell", async ({ page }, testInfo) => {
	test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
	await login(page);
	for (const viewport of viewports) {
		await page.setViewportSize(viewport);
		await page.goto("/settings", { waitUntil: "domcontentloaded" });
		await expect(page.getByRole("navigation", { name: "Main navigation" })).toBeVisible();
		await expect(page.locator(".brand-lockup")).toBeVisible();
		await expect(page.locator("body")).toHaveClass(/library-page/);
		await expect(page.locator("main#main")).toHaveCount(1);
		await expect(page.locator(".skip")).toHaveCount(1);
		await expect(page.locator("[data-command-open]")).toHaveCount(0);
		await expect(page.locator('a[href="/"]').filter({ hasText: "Browse library" }).first()).toHaveAttribute("href", "/");
		expect((await new AxeBuilder({ page }).analyze()).violations, `settings accessibility at ${viewport.width}px`).toEqual([]);
		expect(await layoutProblems(page), `settings shell at ${viewport.width}px`).toEqual({ documentOverflow: 0, outside: [], tinyControls: [], distortedChecks: [], clippedControls: [], overlappingStatuses: [] });
		const shell = await page.evaluate(() => {
			const navigation = document.querySelector(".app-header")!.getBoundingClientRect();
			const main = document.querySelector("main")!.getBoundingClientRect();
			return { navigation: { left: Math.round(navigation.left), right: Math.round(navigation.right), bottom: Math.round(navigation.bottom) }, main: { left: Math.round(main.left) }, viewport: { width: innerWidth, height: innerHeight } };
		});
		expect(await applicationShellProblems(page), `settings rail at ${viewport.width}px`).toEqual({ display: "grid", horizontalOverflow: 0, outside: [] });
		if (viewport.width > 900) {
			expect(shell.navigation).toMatchObject({ left: 0, right: 256, bottom: viewport.height });
			expect(shell.main.left).toBeGreaterThanOrEqual(256);
			expect(await page.locator(".app-header").evaluate((element) => element.scrollTop)).toBe(0);
		} else {
			const dock = await page.locator('.app-header nav').evaluate((element) => {
				const box = element.getBoundingClientRect();
				const header = getComputedStyle(element.closest('.app-header')!);
				return { bottom: Math.round(box.bottom), position: getComputedStyle(element).position, headerBackdrop: header.backdropFilter, headerTransform: header.transform };
			});
			expect(dock).toMatchObject({ bottom: viewport.height, position: "fixed", headerBackdrop: "none", headerTransform: "none" });
			expect(await page.locator('.app-header nav > a, .app-header nav > details').count()).toBeGreaterThanOrEqual(5);
		}
		if (viewport.width === 390) {
			await page.locator(".nav-more > summary").click();
			await expect(page.locator(".nav-more-menu")).toBeVisible();
		}
		await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-settings-navigation-shell.png`) });
	}

	await page.setViewportSize({ width: 1440, height: 900 });
	for (const route of ["/account", "/metadata/bulk", "/offline-downloads", "/quick-connect", "/settings", "/settings/agent-connections", "/settings/backups", "/settings/configuration", "/settings/media-shares", "/settings/remote-readiness", "/settings/system", "/supporter"]) {
		const response = await page.goto(route, { waitUntil: "domcontentloaded" });
		expect(response?.ok(), route).toBeTruthy();
		expect(await applicationShellProblems(page), `${route} rail`).toEqual({ display: "grid", horizontalOverflow: 0, outside: [] });
	}

	await page.goto("/");
	const show = await page.locator('a[href^="/show/"]').first().getAttribute("href");
	expect(show).toBeTruthy();
	await page.goto(show!);
	await expect(page.getByRole("navigation", { name: "Main navigation" }).getByRole("link", { name: "Shows", exact: true })).toHaveAttribute("aria-current", "page");
	await page.screenshot({ path: testInfo.outputPath("1440-show-navigation-shell.png") });

	await page.goto("/");
	const player = await page.locator('a[href^="/watch/"]').first().getAttribute("href");
	expect(player).toBeTruthy();
	await page.goto(player!);
	await expect(page.getByRole("navigation", { name: "Main navigation" })).toHaveCount(0);
});

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
		await expect(trustedHTTPS.locator(".settings-search-result-group")).toHaveText("Advanced · Connections");
		await expect(page.locator("[data-settings-nav]")).toBeVisible();
		await expect(page.locator("#onboarding")).toBeVisible();
		await expect(page.locator("#transcoder")).toBeHidden();
		expect((await new AxeBuilder({ page }).analyze()).violations, `${viewport.width}px settings search accessibility`).toEqual([]);
		expect(await layoutProblems(page), `${viewport.width}px settings search layout`).toEqual({ documentOverflow: 0, outside: [], tinyControls: [], distortedChecks: [], clippedControls: [], overlappingStatuses: [] });
		await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-settings-search-results.png`), fullPage: true });
		await trustedHTTPS.click();
		await expect(page).toHaveURL(/\/settings#trusted-https$/);
		await expect(page.locator("#trusted-https")).toBeVisible();
		await expect(page.locator('[data-settings-levels] [aria-current="page"]')).toHaveText("Advanced");
		await expect(page.locator("#trusted-https")).toBeFocused();
		await search.fill("not a real setting");
		await expect(page.locator("[data-settings-search-status]")).toHaveText("No settings match that search.");
		await expect(page.locator("[data-settings-search-results] a")).toHaveCount(0);
		await search.fill("setup");
		await expect(page.locator("[data-settings-search-status]")).toHaveText(/\d+ matching settings\./);
		await expect(page.locator("[data-settings-search-results] a span").first()).toHaveText("Setup guide");
		await expect(page.locator("[data-settings-search-results] a").first()).toHaveAttribute("href", "#onboarding");
		await expect(page.locator("#onboarding")).toBeHidden();
		await search.fill("subtitles");
		await expect(page.locator("[data-settings-search-results] a").first()).toHaveAttribute("href", "#settings-subtitles");
		await search.fill("not a real setting");
		await search.press("Escape");
		await expect(page.locator("[data-settings-nav]")).toBeVisible();
		await expect(page.locator("#onboarding")).toBeHidden();
		await expect(page.locator("#transcoder")).toBeHidden();
		await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-settings-search-cleared.png`), fullPage: true });
	}
});

test("transcoder support stays concise and accessible", async ({ page }, testInfo) => {
	test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
	await login(page);
	for (const viewport of [{ width: 1440, height: 900 }, { width: 1024, height: 768 }, { width: 390, height: 844 }, { width: 320, height: 800 }]) {
		await page.setViewportSize(viewport);
		await page.goto("/settings#transcoder", { waitUntil: "domcontentloaded" });
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
	await page.goto("/settings#transcoder", { waitUntil: "domcontentloaded" });
	await page.locator("#transcoder>.capability-report").getByText("Video compatibility", { exact: true }).click();
	expect(await layoutProblems(page), "320px forced colors transcoder support").toEqual({ documentOverflow: 0, outside: [], tinyControls: [], distortedChecks: [], clippedControls: [], overlappingStatuses: [] });
	expect((await new AxeBuilder({ page }).analyze()).violations, "320px forced colors transcoder accessibility").toEqual([]);
	await page.screenshot({ path: testInfo.outputPath("320-transcoder-support-forced-colors.png"), fullPage: true });
});
