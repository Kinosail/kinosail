import { expect, test } from "@playwright/test";
import { configureLayoutAudit, expectSearchControl, login, viewports } from "./layout-audit-helpers";

import { createViewer, loginViewer, newViewerPage, removeViewer } from "./test-instance-helpers";

configureLayoutAudit();

test("Owner actions remain reachable on desktop and phone", async ({ page }, testInfo) => {
  test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
  await login(page);
  for (const width of [1440, 1024, 390, 320]) {
    await page.setViewportSize({ width, height: 844 });
    await page.goto("/");
    await expectSearchControl(page, width);
    const menu = page.locator(width > 900 ? ".header-compact-menu" : ".nav-more");
    const toggle = menu.locator(":scope > summary");
    await toggle.click();
    const panel = menu.locator(width > 900 ? ".header-compact-panel" : ".nav-more-menu");
    await expect(panel.getByRole("link", { name: "Settings", exact: true })).toBeVisible();
    await expect(panel.getByRole("link", { name: "Quick Connect", exact: true })).toBeVisible();
    await expect(panel.getByRole("button", { name: "Sign out", exact: true })).toBeVisible();
    for (const control of await panel.locator("a:visible,button:visible").all()) {
      const box = await control.boundingBox();
      expect(box?.height, await control.innerText()).toBeGreaterThanOrEqual(44);
    }
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    await page.screenshot({ path: testInfo.outputPath(`${width}-owner-actions.png`) });
    await toggle.press("Enter");
    await expect(panel).toBeHidden();
  }
});

test("French global search remains readable and usable on narrow phones", async ({ page }, testInfo) => {
  test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
  await login(page);
  for (const width of [390, 320]) {
    await page.setViewportSize({ width, height: 844 });
    await page.goto("/?lang=fr");
    await expectSearchControl(page, width, "Rechercher dans toutes les bibliothèques");
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    await page.screenshot({ path: testInfo.outputPath(`${width}-french-global-search.png`) });
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
				const dock = document.querySelector(".mobile-navigation")!.getBoundingClientRect();
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
	expect(reduced).toMatchObject({ className: "nav-more-menu", animation: "none" });
	expect(Number.parseFloat(reduced.duration)).toBeLessThanOrEqual(0.001);
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
		const navigation = document.querySelector(".mobile-navigation")!.getBoundingClientRect();
		const intersects = picker.left < search.right && picker.right > search.left && picker.top < search.bottom && picker.bottom > search.top;
		return { pickerOverlapsSearch: intersects, searchBottom: search.bottom, navigationTop: navigation.top };
	});
	await page.screenshot({ path: testInfo.outputPath("language-picker-compact-short.png") });
	expect(geometry.pickerOverlapsSearch, "language picker clears the search control").toBeFalsy();
	expect(geometry.searchBottom, "search control clears bottom navigation").toBeLessThanOrEqual(geometry.navigationTop);
});

test("Viewer navigation exposes connection and account actions without Owner tools", async ({ page, browser }, testInfo) => {
  test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
  test.setTimeout(120_000);
  await login(page);
  const viewerName = `Rail Viewer ${Date.now()}`;
  const id = await createViewer(page, viewerName, "viewer-password");
  await page.goto("/settings#security");
  const required = await page.locator('form[action="/settings/mfa"] input[name="required"]').isChecked();
  const setMFA = async (enabled: boolean) => {
    const status = await page.evaluate(async required => {
      const csrf = document.querySelector<HTMLMetaElement>('meta[name="kinosail-csrf"]')!.content;
      return (await fetch("/api/v1/settings/mfa", { method: "PUT", headers: { "Content-Type": "application/json", "X-Kinosail-CSRF": csrf }, body: JSON.stringify({ required }) })).status;
    }, enabled);
    expect(status).toBe(200);
  };
  const viewer = await newViewerPage(browser, new URL(page.url()).origin);
  try {
    if (required) await setMFA(false);
    await loginViewer(viewer, viewerName, "viewer-password");
    for (const viewport of viewports) {
      await viewer.setViewportSize(viewport);
      await viewer.goto("/");
      await expect(viewer.getByRole("navigation", { name: "Main navigation" })).toBeVisible();
      await expect(viewer.locator('.app-header a[href="/settings"]')).toHaveCount(0);
      const menu = viewer.locator(viewport.width > 900 ? ".header-compact-menu" : ".nav-more");
      await menu.locator(":scope > summary").click();
      await expect(menu.getByRole("link", { name: "Quick Connect", exact: true })).toBeVisible();
      await expect(menu.getByRole("button", { name: "Sign out", exact: true })).toBeVisible();
      expect(await viewer.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      await viewer.screenshot({ path: testInfo.outputPath(`${viewport.width}-viewer-navigation.png`) });
    }
  } finally {
    await viewer.context().close();
    if (required) await setMFA(required);
    await removeViewer(page, id);
  }
});

test("desktop library navigation remains reachable in its own scroll region", async ({ page }, testInfo) => {
  test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
  await login(page);
  await page.setViewportSize({ width: 1024, height: 768 });
  await page.goto("/");
  const rail = page.locator(".desktop-sidebar");
  await expect(rail).toBeVisible();
  const edit = rail.getByRole("link", { name: "Edit navigation", exact: true });
  await edit.scrollIntoViewIfNeeded();
  await expect(edit).toBeInViewport();
  await edit.focus();
  await expect(edit).toBeFocused();
  expect(await rail.evaluate(element => element.scrollWidth <= element.clientWidth)).toBe(true);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({ path: testInfo.outputPath("1024-library-navigation-scroll.png") });
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
			return { buttonTops: buttons.map((top) => Math.round(top)), rowTops: cards.map(card => Math.round(card.getBoundingClientRect().top)), wrappedTitles };
		});
		expect(geometry.buttonTops.length, `${viewport.width}px card count`).toBeGreaterThan(1);
		for (const row of new Set(geometry.rowTops)) {
			const tops = geometry.buttonTops.filter((_, index) => geometry.rowTops[index] === row);
			expect(new Set(tops).size, `${viewport.width}px button baseline in row ${row}`).toBe(1);
		}
		if (viewport.width === 1440) expect(geometry.wrappedTitles).toBeGreaterThan(0);
	}
});
