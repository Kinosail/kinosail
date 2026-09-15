import AxeBuilder from "@axe-core/playwright";
import { chromium, expect, firefox, test, webkit, type Page, type TestInfo } from "@playwright/test";
import { layoutProblems, login, presentationProblems, totp, viewports } from "./layout-audit-helpers";

export function registerLayoutEntryTests() {
test("public entry pages fit target responsive viewports", async ({ page }) => {
	for (const viewport of viewports) {
		await page.setViewportSize(viewport);
		for (const route of ["/login", "/setup", "/offline", "/share"]) {
			const response = await page.goto(route);
			expect(response?.ok(), route).toBeTruthy();
			await expect(page.locator("main")).toBeVisible();
			expect((await new AxeBuilder({ page }).analyze()).violations, `${route} accessibility`).toEqual([]);
			expect(await layoutProblems(page), `${route} at ${viewport.width}px`).toEqual({ documentOverflow: 0, outside: [], tinyControls: [], distortedChecks: [], clippedControls: [], overlappingStatuses: [] });
			expect(await presentationProblems(page), `${route} presentation at ${viewport.width}px`).toEqual({ blurred: [], shadowed: [], overRounded: [], settingsBackdrop: null, oversizedSettingsHeading: false });
			const setupIsPublic = route === "/setup" && new URL(page.url()).pathname === "/setup";
			const stylesheetVersion = route === "/offline" ? 65 : setupIsPublic ? 63 : 62;
			await expect(page.locator('link[rel="stylesheet"]')).toHaveAttribute("href", `/static/app.css?v=${stylesheetVersion}`);
		}
	}
});

test("sign in keeps the language preference in one calm composition", async ({ page }, testInfo) => {
	await page.addInitScript(() => localStorage.setItem("kinosail-theme", "dark"));
	for (const viewport of viewports) {
		await page.setViewportSize(viewport);
		await page.goto(viewport.width === 320 ? "/login?lang=ar" : "/login", { waitUntil: "domcontentloaded" });
		const composition = await page.evaluate(() => {
			const login = document.querySelector("main>form:not(.language-picker)")!.getBoundingClientRect();
			const languageElement = document.querySelector("main>.language-picker")!;
			const language = languageElement.getBoundingClientRect();
			const style = getComputedStyle(languageElement);
			return {
				gap: language.top - login.bottom,
				aligned: Math.abs(language.left - login.left) <= 1 && Math.abs(language.width - login.width) <= 1,
				insideDocument: login.top >= 0 && language.bottom <= document.documentElement.scrollHeight,
				horizontalOverflow: document.documentElement.scrollWidth - document.documentElement.clientWidth,
				separatePanel: style.borderTopWidth !== "0px" || style.boxShadow !== "none" || style.backgroundColor !== "rgba(0, 0, 0, 0)",
			};
		});
		expect(composition.gap, `${viewport.width}px gap`).toBeGreaterThanOrEqual(0);
		expect(composition.gap, `${viewport.width}px gap`).toBeLessThanOrEqual(24);
		expect(composition.aligned, `${viewport.width}px alignment`).toBeTruthy();
		expect(composition.insideDocument, `${viewport.width}px document bounds`).toBeTruthy();
		expect(composition.horizontalOverflow, `${viewport.width}px overflow`).toBe(0);
		expect(composition.separatePanel, `${viewport.width}px surface`).toBeFalsy();
		await page.screenshot({ path: testInfo.outputPath(`login-${viewport.width}.png`), fullPage: true });
	}
});

test("rail actions stay compact and separate the account", async ({ page }, testInfo) => {
	test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
	await login(page);
	await page.setViewportSize({ width: 1440, height: 900 });
	await page.goto("/", { waitUntil: "domcontentloaded" });
	const composition = await page.evaluate(() => {
		const actions = document.querySelector(".header-actions")!.getBoundingClientRect();
		const utilities = document.querySelector(".header-utility-links")!.getBoundingClientRect();
		const account = document.querySelector(".header-account")!.getBoundingClientRect();
		const targets = [...document.querySelectorAll(".header-actions a,.header-actions button")].filter((target) => (target as HTMLElement).offsetParent).map((target) => target.getBoundingClientRect());
		return { height: actions.height, utilityColumns: [...document.querySelectorAll(".header-utility-links>a,.header-utility-links>form")].length, accountBelowUtilities: account.top > utilities.bottom, smallestTarget: Math.min(...targets.map((target) => target.height)) };
	});
	expect(composition.height).toBeLessThanOrEqual(210);
	expect(composition.utilityColumns).toBe(3);
	expect(composition.accountBelowUtilities).toBeTruthy();
	expect(composition.smallestTarget).toBeGreaterThanOrEqual(44);
	const actionInsets = await page.evaluate(() => [...document.querySelectorAll(".header-compact-panel .command-open,.header-utility-links a,.header-utility-links button,.header-account a,.header-account button")].filter((target) => (target as HTMLElement).offsetParent).map((target) => getComputedStyle(target).paddingInline));
	expect(new Set(actionInsets), "desktop Actions controls share one inset").toEqual(new Set(["6.4px"]));
	await expect(page.locator(".header-compact-panel .command-open")).toBeVisible();
	await page.screenshot({ path: testInfo.outputPath("1440-owner-rail.png"), fullPage: true });
	await page.setViewportSize({ width: 390, height: 844 });
	const search = page.getByRole("searchbox", { name: "Search library" });
	await search.evaluate((element) => (element as HTMLInputElement).blur());
	const compact = await page.evaluate(() => {
		const menu = document.querySelector(".nav-more") as HTMLDetailsElement;
		const summary = menu.querySelector("summary")!;
		const panel = menu.querySelector(".nav-more-menu")!;
		const nav = document.querySelector('.app-header nav')!;
		const search = document.querySelector('.search input[type="search"]')!.getBoundingClientRect();
		const headerActions = document.querySelector(".header-actions")!;
		const dock = nav.getBoundingClientRect();
		const panelVisible = [...panel.querySelectorAll("a,button")].some((target) => target.getClientRects().length > 0);
		return { overflow: document.documentElement.scrollWidth - document.documentElement.clientWidth, menuClosed: !menu.open, summaryVisible: summary.getClientRects().length > 0, panelVisible, summaryTarget: summary.getBoundingClientRect().height, dockItems: [...nav.children].filter((item) => item.getClientRects().length).length, activeBorder: getComputedStyle(nav.querySelector("a.active")!).borderTopColor, controlsAboveDock: search.bottom <= dock.top, searchWidth: search.width, headerActionsHidden: getComputedStyle(headerActions).display === "none" };
	});
	expect(compact).toMatchObject({ overflow: 0, menuClosed: true, summaryVisible: true, panelVisible: false, summaryTarget: 52, dockItems: 5, activeBorder: "rgba(0, 0, 0, 0)", controlsAboveDock: true, headerActionsHidden: true });
	expect(compact.searchWidth).toBeGreaterThan(0);
	await page.locator(".nav-more > summary").click();
	const expanded = await page.evaluate(() => {
		const menu = document.querySelector(".nav-more") as HTMLDetailsElement;
		const panel = menu.querySelector(".nav-more-menu")!;
		const targets = [...panel.querySelectorAll("a,button")].filter((target) => target.getClientRects().length).map((target) => target.getBoundingClientRect().height);
		const box = panel.getBoundingClientRect();
		return { menuOpen: menu.open, panelVisible: targets.length > 0, smallestTarget: Math.min(...targets), overflow: document.documentElement.scrollWidth - document.documentElement.clientWidth, sheetBottom: Math.round(innerHeight - box.bottom), sheetWidth: Math.round(box.width), libraryColumns: getComputedStyle(panel.querySelector(".nav-more-library")!).gridTemplateColumns.split(" ").length, actionColumns: getComputedStyle(panel.querySelector(".nav-more-actions")!).gridTemplateColumns.split(" ").length, topRightActions: document.querySelector(".header-compact-menu > summary")?.getClientRects().length ?? 0 };
	});
	expect(expanded).toMatchObject({ menuOpen: true, panelVisible: true, smallestTarget: 44, overflow: 0, sheetBottom: 132, sheetWidth: 374, libraryColumns: 2, actionColumns: 1, topRightActions: 0 });
	await page.screenshot({ path: testInfo.outputPath("390-owner-menu-open.png"), fullPage: true });
	await page.keyboard.press("Escape");
	await expect(page.locator(".nav-more")).not.toHaveAttribute("open", "");
	await page.screenshot({ path: testInfo.outputPath("390-owner-menu-closed.png"), fullPage: true });
	await page.setViewportSize({ width: 320, height: 800 });
	const narrow = await page.evaluate(() => ({ overflow: document.documentElement.scrollWidth - innerWidth, searchWidth: document.querySelector('.search input[type="search"]')!.getBoundingClientRect().width, heroScrim: getComputedStyle(document.querySelector(".library-masthead")!, "::before").backgroundImage }));
	expect(narrow).toMatchObject({ overflow: 0 });
	expect(narrow.searchWidth).toBeGreaterThan(0);
	expect(narrow.heroScrim).toContain("rgba(5, 8, 10, 0.86)");
	await page.screenshot({ path: testInfo.outputPath("320-owner-controls.png") });
});

test("functional motion stays restrained and respects reduced motion", async ({ page }, testInfo) => {
	test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
	await login(page);
	await page.setViewportSize({ width: 390, height: 844 });
	await page.goto("/", { waitUntil: "domcontentloaded" });
	const panel = page.locator(".nav-more-menu");
	await page.locator(".nav-more > summary").click();
	await expect.poll(() => panel.getAttribute("class")).toContain("motion-panel-open");
	const motion = await panel.evaluate((element) => ({ animation: getComputedStyle(element).animationName, duration: getComputedStyle(element).animationDuration }));
	expect(motion).toEqual({ animation: "kinosail-panel-enter", duration: "0.18s" });
	await page.screenshot({ path: testInfo.outputPath("390-motion-menu-open.png"), fullPage: true });

	await page.emulateMedia({ reducedMotion: "reduce" });
	await page.reload({ waitUntil: "domcontentloaded" });
	await page.locator(".nav-more > summary").click();
	const reduced = await panel.evaluate((element) => ({ className: element.className, animation: getComputedStyle(element).animationName, duration: getComputedStyle(element).animationDuration }));
	expect(reduced).toEqual({ className: "nav-more-menu", animation: "none", duration: "0s" });
});

test("library language picker stays clear of compact fixed controls", async ({ page }, testInfo) => {
	test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
	await login(page);
	await page.setViewportSize({ width: 390, height: 175 });
	await page.goto("/", { waitUntil: "domcontentloaded" });
	await page.evaluate(() => window.scrollTo(0, document.documentElement.scrollHeight));
	const geometry = await page.evaluate(() => {
		const picker = document.querySelector(".library-page>.language-picker")!.getBoundingClientRect();
		const search = document.querySelector(".search")!.getBoundingClientRect();
		const navigation = document.querySelector(".app-header nav")!.getBoundingClientRect();
		return { pickerBottom: picker.bottom, searchTop: search.top, searchBottom: search.bottom, navigationTop: navigation.top };
	});
	await page.screenshot({ path: testInfo.outputPath("language-picker-compact-short.png") });
	expect(geometry.pickerBottom, "language picker clears the search control").toBeLessThanOrEqual(geometry.searchTop);
	expect(geometry.searchBottom, "search control clears bottom navigation").toBeLessThanOrEqual(geometry.navigationTop);
});

test("viewer rail adapts across desktop, tablet, and compact widths", async ({ page }, testInfo) => {
	test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
	const viewerName = `Rail Viewer ${Date.now()}`;
	await login(page);
	await page.goto("/settings#access");
	const profile = page.locator('form[action="/settings/profiles"]');
	await profile.getByLabel("New profile name").fill(viewerName);
	await profile.getByLabel("New profile password").fill("viewer-password");
	await profile.getByLabel("Libraries").fill("all");
	await profile.getByRole("button", { name: "Add Profile" }).click();
	await page.goto("/");
	if (await page.locator(".nav-more > summary").isVisible()) await page.locator(".nav-more > summary").click();
	await page.getByRole("button", { name: "Sign out", exact: true }).click();
	await expect(page).toHaveURL("/login");
	await login(page, viewerName, "viewer-password");
	await expect(page).toHaveURL("/");
	for (const viewport of viewports) {
		await page.setViewportSize(viewport);
		const state = await page.evaluate(() => {
			const menu = document.querySelector(".nav-more") as HTMLDetailsElement;
			const summary = menu.querySelector("summary")!;
			const panel = menu.querySelector(".nav-more-menu")!;
			const panelVisible = [...panel.querySelectorAll("a,button")].some((target) => target.getClientRects().length > 0);
			const utilities = document.querySelector(".header-utility-links")!.getBoundingClientRect();
			const connect = document.querySelector('.header-utility-links a[href="/quick-connect"]')!.getBoundingClientRect();
			const mobileConnect = document.querySelector('.nav-more-actions a[href="/quick-connect"]')!.getBoundingClientRect();
			const actions = document.querySelector(".header-actions")!.getBoundingClientRect();
			return {
				ownerLayout: document.querySelector(".owner-utilities") !== null,
				columns: getComputedStyle(document.querySelector(".header-utility-links")!).gridTemplateColumns.split(" ").length,
				fillsRow: Math.abs(utilities.width - connect.width) <= 1,
				headerActionsHidden: actions.height === 0,
				desktopTarget: connect.height,
				mobileConnectTarget: mobileConnect.height,
				overflow: document.documentElement.scrollWidth - document.documentElement.clientWidth,
				menuClosed: !menu.open,
				summaryVisible: summary.getClientRects().length > 0,
				panelVisible,
				summaryTarget: summary.getBoundingClientRect().height,
			};
		});
		expect(state.ownerLayout, `${viewport.width}px owner layout`).toBeFalsy();
		if (viewport.width > 900) {
			expect(state).toMatchObject({ columns: 1, fillsRow: true, headerActionsHidden: false, desktopTarget: 44 });
		} else {
			expect(state).toMatchObject({ overflow: 0, headerActionsHidden: true, menuClosed: true, summaryVisible: true, panelVisible: false, summaryTarget: 52 });
			if (viewport.width === 720) {
				await page.locator(".nav-more > summary").click();
				await page.locator(".nav-more-menu").evaluate(async (element) => {
					await Promise.all(element.getAnimations().map((animation) => animation.finished));
				});
				expect(await page.evaluate(() => {
					const panel = document.querySelector(".nav-more-menu")!;
					const connect = document.querySelector('.nav-more-actions a[href="/quick-connect"]')!;
					return { panelVisible: panel.getClientRects().length > 0, target: connect.getBoundingClientRect().height, overflow: document.documentElement.scrollWidth - document.documentElement.clientWidth };
				})).toEqual({ panelVisible: true, target: 44, overflow: 0 });
				await page.locator(".nav-more > summary").click();
			}
		}
		await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-viewer-rail.png`), fullPage: true });
	}
});

test("desktop rail keeps navigation and utilities in one scroll context", async ({ page }, testInfo) => {
	test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
	await login(page);
	await page.setViewportSize({ width: 1024, height: 768 });
	await page.goto("/");
	const state = await page.evaluate(() => {
		const rail = document.querySelector(".app-header")!;
		const navigation = rail.querySelector("nav")!;
		const actions = rail.querySelector(".header-actions")!;
		return {
			railOverflowing: rail.scrollHeight > rail.clientHeight,
			navigationOverflow: getComputedStyle(navigation).overflowY,
			actionsAfterNavigation: actions.getBoundingClientRect().top >= navigation.getBoundingClientRect().bottom,
		};
	});
	expect(state).toEqual({ railOverflowing: true, navigationOverflow: "visible", actionsAfterNavigation: true });
	await page.screenshot({ path: testInfo.outputPath("1024-desktop-rail-scroll-context.png"), fullPage: true });
});
}
