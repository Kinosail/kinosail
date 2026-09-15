import AxeBuilder from "@axe-core/playwright";
import { chromium, expect, firefox, test, webkit, type Page, type TestInfo } from "@playwright/test";
import { layoutProblems, login, presentationProblems, totp, viewports } from "./layout-audit-helpers";

export function registerLayoutMediaTests() {
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
			const wrappedTitles = cards.filter((card) => card.querySelector("h2")!.getBoundingClientRect().height > 20).length;
			return { buttonTops: buttons.map((top) => Math.round(top)), wrappedTitles };
		});
		expect(geometry.buttonTops.length, `${viewport.width}px card count`).toBeGreaterThan(1);
		expect(new Set(geometry.buttonTops).size, `${viewport.width}px button baseline`).toBe(1);
		if (viewport.width === 1440) expect(geometry.wrappedTitles).toBeGreaterThan(0);
	}
});

test("player shows and switches its playback method without crowding actions", async ({ page }, testInfo) => {
	test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
	await login(page);
	await page.goto("/?view=movies");
	const watch = await page.locator('a.card[href^="/watch/"]').first().getAttribute("href");
	expect(watch).toBeTruthy();
	for (const viewport of viewports) {
		await page.setViewportSize(viewport);
		await page.goto(watch!);
		const actions = page.locator(".primary-player-actions");
		const method = page.locator("[data-playback-mode-status]");
		await expect(method).toHaveText("Direct Play");
		const methodGeometry = await method.evaluate((element) => {
			const box = element.getBoundingClientRect();
			const toolbar = element.parentElement!.getBoundingClientRect();
			return { height: box.height, inside: box.left >= toolbar.left && box.right <= toolbar.right };
		});
		expect(methodGeometry.height).toBeGreaterThanOrEqual(44);
		expect(methodGeometry.inside).toBeTruthy();
		await method.click();
		const playback = page.getByRole("group", { name: "Playback policy", exact: true });
		await expect(playback).toBeVisible();
		const compatibleLabel = await page.locator("video").getAttribute("data-compatibility-label") || "Compatibility";
		await playback.locator('input[value="compatible"]').check();
		await expect(method).toHaveText(compatibleLabel);
		await page.waitForTimeout(250);
		const settingsGeometry = await page.locator(".player-settings").evaluate((panel) => {
			const box = panel.getBoundingClientRect();
			const actions = document.querySelector(".primary-player-actions")!.getBoundingClientRect();
			return { clear: box.bottom + 8 <= actions.top, background: getComputedStyle(panel).backgroundColor };
		});
		expect(settingsGeometry.clear).toBeTruthy();
		expect(settingsGeometry.background).not.toBe("rgba(0, 0, 0, 0)");
		expect((await new AxeBuilder({ page }).analyze()).violations, `playback settings accessibility at ${viewport.width}px`).toEqual([]);
		await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-player-compatibility.png`), fullPage: true });
		await playback.locator('input[value="direct-first"]').check();
		await expect(method).toHaveText("Direct Play");
		await page.keyboard.press("Escape");
		await expect(playback).toBeHidden();
		await expect(actions.getByRole("button", { name: /My List$/ })).toBeVisible();
		await expect(actions.getByRole("button", { name: /^Mark / })).toBeVisible();
		await expect(actions.getByRole("link", { name: compatibleLabel })).toBeHidden();
		expect(await layoutProblems(page), `closed actions at ${viewport.width}px`).toEqual({ documentOverflow: 0, outside: [], tinyControls: [], distortedChecks: [], clippedControls: [], overlappingStatuses: [] });
		await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-player-actions-closed.png`), fullPage: true });
		await actions.getByText("Playback & downloads", { exact: true }).click();
		await expect(actions.getByRole("link", { name: compatibleLabel })).toBeVisible();
		await expect(actions.getByRole("button", { name: /Prepare .* offline/ })).toBeVisible();
		expect((await new AxeBuilder({ page }).analyze()).violations, `open actions accessibility at ${viewport.width}px`).toEqual([]);
		expect(await layoutProblems(page), `open actions at ${viewport.width}px`).toEqual({ documentOverflow: 0, outside: [], tinyControls: [], distortedChecks: [], clippedControls: [], overlappingStatuses: [] });
		await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-player-actions-open.png`), fullPage: true });
	}
});

test("player stays accessible in alternate display modes", async ({ page }, testInfo) => {
	test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
	await page.addInitScript(() => localStorage.setItem("kinosail-theme", "dark"));
	await page.emulateMedia({ reducedMotion: "reduce" });
	await login(page);
	await page.goto("/?view=movies");
	const watch = await page.locator('a.card[href^="/watch/"]').first().getAttribute("href");
	expect(watch).toBeTruthy();
	for (const viewport of [viewports[0], viewports[viewports.length - 1]]) {
		await page.setViewportSize(viewport);
		await page.goto(watch!);
		expect((await new AxeBuilder({ page }).analyze()).violations, `${viewport.width}px dark player accessibility`).toEqual([]);
		expect(await layoutProblems(page), `${viewport.width}px dark player`).toEqual({ documentOverflow: 0, outside: [], tinyControls: [], distortedChecks: [], clippedControls: [], overlappingStatuses: [] });
		expect(await page.evaluate(() => [...document.querySelectorAll("*")].filter((element) => getComputedStyle(element).transitionDuration.split(",").some((duration) => Number.parseFloat(duration) > 0))), `${viewport.width}px reduced motion`).toEqual([]);
		await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-dark-reduced-player.png`), fullPage: true });
	}
	await page.emulateMedia({ forcedColors: "active", reducedMotion: "reduce" });
	await page.goto(watch!);
	expect((await new AxeBuilder({ page }).analyze()).violations, "forced-colors player accessibility").toEqual([]);
	expect(await layoutProblems(page), "forced-colors player").toEqual({ documentOverflow: 0, outside: [], tinyControls: [], distortedChecks: [], clippedControls: [], overlappingStatuses: [] });
	await page.keyboard.press("Tab");
	await expect(page.locator(":focus-visible")).toBeVisible();
	await page.screenshot({ path: testInfo.outputPath("320-forced-colors-player.png"), fullPage: true });
});

test("player explains and recovers from a required video transcode", async ({ page }, testInfo) => {
	test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
	await login(page);
	await page.goto("/?view=movies");
	const watch = await page.locator('a.card[href^="/watch/"]').first().getAttribute("href");
	expect(watch).toBeTruthy();
	for (const viewport of [viewports[0], viewports[viewports.length - 1]]) {
		await page.setViewportSize(viewport);
		await page.goto(watch!);
		await page.locator("video").evaluate((video) => {
			video.pause();
			video.dataset.compatibilityMode = "transcode";
			video.dataset.compatibilityLabel = "Transcoding video";
			video.dataset.compatibilityDescription = "This device cannot decode the original video.";
			Object.defineProperty(video, "error", { configurable: true, value: { code: 4 } });
			video.dispatchEvent(new Event("error"));
		});
		const method = page.locator("[data-playback-mode-status]");
		await expect(method).toHaveText("Direct Play stopped");
		await expect(page.locator("[data-player-status] [data-player-fallback]")).toHaveText("Start video transcode");
		expect((await new AxeBuilder({ page }).analyze()).violations, `${viewport.width}px recovery accessibility`).toEqual([]);
		expect(await layoutProblems(page), `${viewport.width}px recovery overlay`).toEqual({ documentOverflow: 0, outside: [], tinyControls: [], distortedChecks: [], clippedControls: [], overlappingStatuses: [] });
		await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-player-recovery-overlay.png`), fullPage: true });
		await method.focus();
		await page.keyboard.press("Enter");
		await expect(page.locator("[data-playback-recovery]")).toBeVisible();
		await expect(page.locator("[data-player-status]")).toBeHidden();
		await expect(page.locator("[data-playback-recovery] [data-player-fallback]")).toHaveText("Start video transcode");
		expect((await new AxeBuilder({ page }).analyze()).violations, `${viewport.width}px recovery settings accessibility`).toEqual([]);
		expect(await layoutProblems(page), `${viewport.width}px recovery settings`).toEqual({ documentOverflow: 0, outside: [], tinyControls: [], distortedChecks: [], clippedControls: [], overlappingStatuses: [] });
		await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-player-recovery-settings.png`), fullPage: true });
	}
});

test("artwork-backed media copy keeps a protected reading surface", async ({ page }) => {
	test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
	await login(page);
	await page.goto("/?view=shows");
	const show = await page.locator('a.show-details[href^="/show/"]').first().getAttribute("href");
	await page.goto("/?view=movies");
	const watch = await page.locator('a.card[href^="/watch/"]').first().getAttribute("href");
	expect(show).toBeTruthy();
	expect(watch).toBeTruthy();
	for (const viewport of [{ width: 1440, height: 900 }, { width: 1024, height: 768 }, { width: 390, height: 844 }, { width: 320, height: 800 }]) {
		await page.setViewportSize(viewport);
		for (const [route, selector] of [[show!, ".media-hero.has-media-backdrop>.media-hero-copy"], [watch!, ".player-page.has-media-backdrop .title-block"]] as const) {
			await page.goto(route);
			const copy = page.locator(selector);
			await expect(copy).toBeVisible();
			const styles = await copy.evaluate((element) => {
				const computed = getComputedStyle(element);
				return { background: computed.backgroundColor, padding: computed.padding };
			});
			expect(styles.background, `${route} at ${viewport.width}px copy background`).not.toBe("rgba(0, 0, 0, 0)");
			expect(styles.padding, `${route} at ${viewport.width}px copy padding`).not.toBe("0px");
		}
	}
});

const betaRouteGroups = [
	{ name: "library", routes: ["/", "/?view=list", "/?view=movies", "/?view=shows", "/?view=collections", "/?view=playlists"] },
	{ name: "library more", routes: ["/?view=music", "/?view=audiobooks", "/?view=books", "/?view=photos", "/?view=history", "/?view=unwatched"] },
	{ name: "utilities", routes: ["/quick-connect", "/offline-downloads", "/supporter"] },
	{ name: "settings", routes: ["/settings", "/settings#playback", "/settings#access", "/settings#system"] },
	{ name: "system settings", routes: ["/settings/configuration", "/settings/backups", "/settings/system", "/settings/agent-connections"] },
	{ name: "connection settings", routes: ["/settings/media-shares", "/settings/remote-readiness"] },
	{ name: "onboarding", routes: ["/onboarding/connection", "/onboarding/household", "/onboarding/migrate", "/account"] },
] as const;

async function auditBetaRoutes(page: Page, testInfo: TestInfo, viewport: typeof viewports[number], routes: string[]) {
	const violations: { route: string; viewport: number; problems: Awaited<ReturnType<typeof layoutProblems>> }[] = [];
	const presentationViolations: { route: string; viewport: number; problems: Awaited<ReturnType<typeof presentationProblems>> }[] = [];
	const accessibilityViolations: { route: string; rules: string[] }[] = [];
	const storageState = await page.context().storageState();
	const browserType = { chromium, firefox, webkit }[testInfo.project.name as "chromium" | "firefox" | "webkit"];
	const routeBrowser = await browserType.launch();
	try {
		for (const route of routes) {
			const routeContext = await routeBrowser.newContext({ baseURL: process.env.KINOSAIL_E2E_URL ?? "https://127.0.0.1:38128", ignoreHTTPSErrors: true, storageState, viewport });
			const routePage = await routeContext.newPage();
			try {
				const response = await routePage.goto(route, { waitUntil: "domcontentloaded" });
				expect(response?.ok(), route).toBeTruthy();
				await expect(routePage.locator("main")).toBeVisible();
				const stylesheet = /\/static\/app\.css\?v=\d+$/;
				await expect(routePage.locator('link[rel="stylesheet"][href^="/static/app.css"]')).toHaveAttribute("href", stylesheet);
				await expect(routePage.locator('link[rel="stylesheet"][href^="/static/app.css"]')).toHaveCount(1);
				if (route === "/settings/configuration") await expect(routePage.getByLabel("SCIM token expiration date")).toBeVisible();
				if (route === "/settings") {
					for (const key of ["integrations.tmdb.token", "integrations.oidc", "integrations.scim", "integrations.webhook.url", "dlna.url", "backup.key"]) {
						await expect(routePage.locator(`a[href="/settings/configuration#${key}"]`)).toHaveCount(1);
					}
				}
				const problems = await layoutProblems(routePage);
				await routePage.screenshot({ path: testInfo.outputPath(`${viewport.width}-${route.replace(/[^a-z0-9]+/gi, "-") || "home"}.png`), fullPage: true });
				if (problems.documentOverflow || problems.outside.length || problems.tinyControls.length || problems.distortedChecks.length || problems.clippedControls.length || problems.overlappingStatuses.length) violations.push({ route, viewport: viewport.width, problems });
				const presentation = await presentationProblems(routePage);
				if (presentation.blurred.length || presentation.shadowed.length || presentation.overRounded.length || presentation.settingsBackdrop || presentation.oversizedSettingsHeading) presentationViolations.push({ route, viewport: viewport.width, problems: presentation });
				const rules = (await new AxeBuilder({ page: routePage }).analyze()).violations.map(({ id }) => id);
				if (rules.length) accessibilityViolations.push({ route: `${route} at ${viewport.width}px`, rules });
			} finally {
				await routeContext.close();
			}
		}
	} finally {
		await routeBrowser.close();
	}
	expect(violations).toEqual([]);
	expect(presentationViolations).toEqual([]);
	expect(accessibilityViolations).toEqual([]);
}

for (const viewport of viewports) {
	for (const group of betaRouteGroups) {
		test(`beta ${group.name} pages fit the ${viewport.width}px viewport`, async ({ page }, testInfo) => {
			test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
			test.setTimeout(180_000);
			await login(page);
			await auditBetaRoutes(page, testInfo, viewport, [...group.routes]);
		});
	}

	test(`beta media pages fit the ${viewport.width}px viewport`, async ({ page }, testInfo) => {
		test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
		test.setTimeout(180_000);
		await login(page);
		const href = async (route: string, selector: string) => { await page.goto(route); const value = await page.locator(selector).first().getAttribute("href"); expect(value, selector).toBeTruthy(); return value!; };
		const show = await href("/?view=shows", 'a.show-details[href^="/show/"]');
		const album = await href("/?view=music", 'a.card[href^="/album/"]');
		const book = await href("/?view=books", 'a.card[href^="/book/"]');
		const watch = await href("/?view=movies", 'a.card[href^="/watch/"]');
		const reader = await href(book, 'a[href^="/read/"]');
		await auditBetaRoutes(page, testInfo, viewport, [watch, show, album, book, reader]);
	});

	test(`beta created pages fit the ${viewport.width}px viewport`, async ({ page }, testInfo) => {
		test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
		test.setTimeout(120_000);
		await login(page);
		await page.goto("/?view=playlists");
		const playlistForm = page.locator('form[action="/playlists"]');
		await playlistForm.getByLabel("New playlist name").fill(`Layout-${testInfo.project.name}-${viewport.width}-${Date.now()}`);
		await playlistForm.getByRole("button", { name: "Create" }).click();
		const playlist = new URL(page.url()).pathname;
		await page.goto("/?view=collections");
		await page.getByLabel("New Collection name").fill(`Layout collection-${testInfo.project.name}-${viewport.width}-${Date.now()}`);
		await page.getByLabel("New Collection name").press("Enter");
		await auditBetaRoutes(page, testInfo, viewport, [playlist, new URL(page.url()).pathname]);
	});
}
}
