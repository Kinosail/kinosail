import { expect, test } from "@playwright/test";
import { configureLayoutAudit, expectSearchControl, login, viewports } from "./layout-audit-helpers";

configureLayoutAudit();

test("rail actions stay compact and separate the account", async ({ page }, testInfo) => {
	test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
	await login(page);
	await page.setViewportSize({ width: 1440, height: 900 });
	await page.goto("/", { waitUntil: "domcontentloaded" });
	await expectSearchControl(page, 1440);
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
	await expectSearchControl(page, 390);
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
	expect(expanded).toMatchObject({ menuOpen: true, panelVisible: true, overflow: 0, libraryColumns: 2, actionColumns: 1, topRightActions: 0 });
	expect(expanded.smallestTarget).toBeGreaterThanOrEqual(43);
	expect(expanded.sheetBottom).toBeGreaterThanOrEqual(131);
	expect(expanded.sheetWidth).toBeGreaterThanOrEqual(370);
	await page.screenshot({ path: testInfo.outputPath("390-owner-menu-open.png"), fullPage: true });
	await page.keyboard.press("Escape");
	await expect(page.locator(".nav-more")).not.toHaveAttribute("open", "");
	await page.screenshot({ path: testInfo.outputPath("390-owner-menu-closed.png"), fullPage: true });
	await page.setViewportSize({ width: 320, height: 800 });
	await expectSearchControl(page, 320);
	const narrow = await page.evaluate(() => ({ overflow: document.documentElement.scrollWidth - innerWidth, searchWidth: document.querySelector('.search input[type="search"]')!.getBoundingClientRect().width, heroScrim: getComputedStyle(document.querySelector(".library-masthead")!, "::before").backgroundImage }));
	expect(narrow).toMatchObject({ overflow: 0 });
	expect(narrow.searchWidth).toBeGreaterThan(0);
	expect(narrow.heroScrim).toContain("rgba(5, 8, 10, 0.86)");
	await page.screenshot({ path: testInfo.outputPath("320-owner-controls.png") });
	for (const viewport of [{ width: 390, height: 844 }, { width: 320, height: 800 }]) {
		await page.setViewportSize(viewport);
		await page.goto("/?lang=fr", { waitUntil: "domcontentloaded" });
		await expectSearchControl(page, viewport.width, "Rechercher dans la bibliothèque");
		expect(await page.evaluate(() => document.documentElement.scrollWidth - innerWidth), `${viewport.width}px French search overflow`).toBe(0);
		await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-french-search.png`) });
	}
});

test("empty My List gives a keyboard recovery path", async ({ page }, testInfo) => {
	test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
	await login(page);
	for (const viewport of [{ width: 1440, height: 900 }, { width: 390, height: 844 }, { width: 320, height: 800 }]) {
		await page.setViewportSize(viewport);
		await page.goto("/?view=list", { waitUntil: "domcontentloaded" });
		await expect(page.getByRole("heading", { name: "My List is empty." })).toBeVisible();
		const recovery = page.getByRole("link", { name: "Browse library", exact: true });
		await expect(recovery).toHaveAttribute("href", "/");
		await recovery.focus();
		await expect(recovery).toBeFocused();
		const geometry = await recovery.evaluate((element) => {
			const box = element.getBoundingClientRect();
			const style = getComputedStyle(element);
			return { left: box.left, right: box.right, width: box.width, height: box.height, viewport: innerWidth, overflow: document.documentElement.scrollWidth - innerWidth, outlineStyle: style.outlineStyle, outlineWidth: Number.parseFloat(style.outlineWidth) };
		});
		expect(geometry.left, `${viewport.width}px recovery left`).toBeGreaterThanOrEqual(0);
		expect(geometry.right, `${viewport.width}px recovery right`).toBeLessThanOrEqual(geometry.viewport);
		expect(geometry.width, `${viewport.width}px recovery target`).toBeGreaterThanOrEqual(44);
		expect(geometry.height, `${viewport.width}px recovery target`).toBeGreaterThanOrEqual(44);
		expect(geometry.overflow, `${viewport.width}px recovery overflow`).toBe(0);
		expect(geometry.outlineStyle, `${viewport.width}px recovery focus style`).not.toBe("none");
		expect(geometry.outlineWidth, `${viewport.width}px recovery focus width`).toBeGreaterThanOrEqual(2);
		if (viewport.width === 320) {
			await page.evaluate(() => scrollTo(0, document.documentElement.scrollHeight));
			await expect.poll(() => page.evaluate(() => {
				const dock = document.querySelector(".app-header nav")!.getBoundingClientRect();
				const footer = document.querySelector(".library-shell footer")!.getBoundingClientRect();
				const language = document.querySelector("body > .language-picker")!.getBoundingClientRect();
				return Math.max(footer.bottom, language.bottom) - dock.top;
			}), { message: "320px footer and language picker clear the dock at scroll end" }).toBeLessThanOrEqual(0);
		}
		await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-my-list-recovery.png`), fullPage: true });
	}
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
		const intersects = picker.left < search.right && picker.right > search.left && picker.top < search.bottom && picker.bottom > search.top;
		return { pickerOverlapsSearch: intersects, searchBottom: search.bottom, navigationTop: navigation.top };
	});
	await page.screenshot({ path: testInfo.outputPath("language-picker-compact-short.png") });
	expect(geometry.pickerOverlapsSearch, "language picker clears the search control").toBeFalsy();
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
	const csrf = await page.locator('meta[name="kinosail-csrf"]').getAttribute("content");
	const mfa = await page.evaluate(async (csrf) => {
		const response = await fetch("/api/v1/settings/mfa", { method: "PUT", headers: { "Content-Type": "application/json", "X-Kinosail-CSRF": csrf ?? "" }, body: JSON.stringify({ required: false }) });
		return response.ok;
	}, csrf);
	expect(mfa).toBeTruthy();
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

test("continue watching actions share a baseline when titles wrap", async ({ page }) => {
	test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
	await login(page);
	await page.goto("/");
	const hrefs = await page.evaluate(async () => {
		const response = await fetch("/api/v1/library?view=all");
		if (!response.ok) throw new Error(`library failed: ${response.status}`);
		const catalog = await response.json() as { items?: Array<{ id?: string; kind?: string }> };
		return (catalog.items ?? []).filter((item) => item.kind === "video" && item.id).map((item) => `/watch/${item.id}`);
	});
	expect(hrefs.length).toBeGreaterThan(1);
	const csrf = await page.locator('meta[name="kinosail-csrf"]').getAttribute("content");
	expect(csrf).toBeTruthy();
	await page.evaluate(async ({ paths, csrf }) => {
		for (const path of paths) {
			const id = path.slice(path.lastIndexOf("/") + 1);
			const response = await fetch(`/api/v1/items/${id}/progress`, { method: "PUT", headers: { "Content-Type": "application/json", "X-Kinosail-CSRF": csrf }, body: JSON.stringify({ seconds: 61 }) });
			if (!response.ok) throw new Error(`progress failed: ${response.status}`);
		}
	}, { paths: hrefs, csrf: csrf! });
	await page.reload();
	const shelf = page.locator(".home-shelf").filter({ hasText: "Continue watching" }).first();
	for (const viewport of [{ width: 1440, height: 900 }, { width: 390, height: 844 }]) {
		await page.setViewportSize(viewport);
		const geometry = await shelf.locator("article.card").evaluateAll((cards) => {
			const buttons = cards.map((card) => card.querySelector("button")!.getBoundingClientRect().top);
			const wrappedTitles = cards.filter((card) => {
				const title = card.querySelector("h3")!;
				return title.getBoundingClientRect().height > Number.parseFloat(getComputedStyle(title).lineHeight) * 1.5;
			}).length;
			return { buttonTops: buttons.map((top) => Math.round(top)), wrappedTitles };
		});
		expect(geometry.buttonTops.length, `${viewport.width}px card count`).toBeGreaterThan(1);
		expect(new Set(geometry.buttonTops).size, `${viewport.width}px button baseline`).toBe(1);
		if (viewport.width === 1440) expect(geometry.wrappedTitles).toBeGreaterThan(0);
	}
});
