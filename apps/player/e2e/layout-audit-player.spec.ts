import { expect, test } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { configureLayoutAudit, layoutProblems, login, viewports } from "./layout-audit-helpers";

configureLayoutAudit();

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
			return {
				height: box.height,
				inside: box.left >= toolbar.left && box.right <= toolbar.right,
			};
		});
		expect(methodGeometry.height).toBeGreaterThanOrEqual(44);
		expect(methodGeometry.inside).toBeTruthy();
		await method.click();
		const playback = page.getByRole("group", {
			name: "Playback policy",
			exact: true,
		});
		await expect(playback).toBeVisible();
		const compatibleLabel = (await page.locator("video").getAttribute("data-compatibility-label")) || "Compatibility";
		await playback.locator('input[value="compatible"]').check();
		await expect(method).toHaveText(new RegExp(`^(?:Starting )?${compatibleLabel}$`));
		await page.waitForTimeout(250);
		const settingsGeometry = await page.locator(".player-settings").evaluate((panel) => {
			const box = panel.getBoundingClientRect();
			const actions = document.querySelector(".primary-player-actions")!.getBoundingClientRect();
			return {
				clear: box.bottom + 8 <= actions.top,
				height: box.height,
				background: getComputedStyle(panel).backgroundColor,
			};
		});
		expect(settingsGeometry.clear).toBeTruthy();
		expect(settingsGeometry.height).toBeGreaterThanOrEqual(200);
		expect(settingsGeometry.background).not.toBe("rgba(0, 0, 0, 0)");
		expect((await new AxeBuilder({ page }).analyze()).violations, `playback settings accessibility at ${viewport.width}px`).toEqual([]);
		await page.screenshot({
			path: testInfo.outputPath(`${viewport.width}-player-compatibility.png`),
			fullPage: true,
		});
		await playback.locator('input[value="direct-first"]').check();
		await expect(method).toHaveText("Direct Play");
		await page.keyboard.press("Escape");
		await expect(playback).toBeHidden();
		await expect(actions.getByRole("button", { name: /My List$/ })).toBeVisible();
		await expect(actions.getByRole("button", { name: /^Mark / })).toBeVisible();
		await expect(actions.getByRole("link", { name: compatibleLabel })).toBeHidden();
		expect(await layoutProblems(page), `closed actions at ${viewport.width}px`).toEqual({
			documentOverflow: 0,
			outside: [],
			tinyControls: [],
			distortedChecks: [],
			clippedControls: [],
			overlappingStatuses: [],
		});
		await page.screenshot({
			path: testInfo.outputPath(`${viewport.width}-player-actions-closed.png`),
			fullPage: true,
		});
		await actions.getByText("Playback & downloads", { exact: true }).click();
		await expect(actions.getByRole("link", { name: compatibleLabel })).toBeVisible();
		await expect(actions.getByRole("button", { name: /Prepare .* offline/ })).toBeVisible();
		expect((await new AxeBuilder({ page }).analyze()).violations, `open actions accessibility at ${viewport.width}px`).toEqual([]);
		expect(await layoutProblems(page), `open actions at ${viewport.width}px`).toEqual({
			documentOverflow: 0,
			outside: [],
			tinyControls: [],
			distortedChecks: [],
			clippedControls: [],
			overlappingStatuses: [],
		});
		await page.screenshot({
			path: testInfo.outputPath(`${viewport.width}-player-actions-open.png`),
			fullPage: true,
		});
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
		expect(await layoutProblems(page), `${viewport.width}px dark player`).toEqual({
			documentOverflow: 0,
			outside: [],
			tinyControls: [],
			distortedChecks: [],
			clippedControls: [],
			overlappingStatuses: [],
		});
		expect(
			await page.evaluate(() =>
				[...document.querySelectorAll("*")].filter((element) =>
					getComputedStyle(element)
						.transitionDuration.split(",")
						.some((duration) => Number.parseFloat(duration) > 0),
				),
			),
			`${viewport.width}px reduced motion`,
		).toEqual([]);
		await page.screenshot({
			path: testInfo.outputPath(`${viewport.width}-dark-reduced-player.png`),
			fullPage: true,
		});
	}
	await page.emulateMedia({ forcedColors: "active", reducedMotion: "reduce" });
	await page.goto(watch!);
	expect((await new AxeBuilder({ page }).analyze()).violations, "forced-colors player accessibility").toEqual([]);
	expect(await layoutProblems(page), "forced-colors player").toEqual({
		documentOverflow: 0,
		outside: [],
		tinyControls: [],
		distortedChecks: [],
		clippedControls: [],
		overlappingStatuses: [],
	});
	await page.keyboard.press("Tab");
	await expect(page.locator(":focus-visible")).toBeVisible();
	await page.screenshot({
		path: testInfo.outputPath("320-forced-colors-player.png"),
		fullPage: true,
	});
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
			Object.defineProperty(video, "error", {
				configurable: true,
				value: { code: 4 },
			});
			video.dispatchEvent(new Event("error"));
		});
		const method = page.locator("[data-playback-mode-status]");
		await expect(method).toHaveText("Direct Play stopped");
		await expect(page.locator("[data-player-status] [data-player-fallback]")).toHaveText("Start video transcode");
		expect((await new AxeBuilder({ page }).analyze()).violations, `${viewport.width}px recovery accessibility`).toEqual([]);
		expect(await layoutProblems(page), `${viewport.width}px recovery overlay`).toEqual({
			documentOverflow: 0,
			outside: [],
			tinyControls: [],
			distortedChecks: [],
			clippedControls: [],
			overlappingStatuses: [],
		});
		await page.screenshot({
			path: testInfo.outputPath(`${viewport.width}-player-recovery-overlay.png`),
			fullPage: true,
		});
		await method.focus();
		await page.keyboard.press("Enter");
		await expect(page.locator("[data-playback-recovery]")).toBeVisible();
		expect((await page.locator(".player-settings").boundingBox())?.height).toBeGreaterThanOrEqual(200);
		await expect(page.locator("[data-player-status]")).toBeHidden();
		await expect(page.locator("[data-playback-recovery] [data-player-fallback]")).toHaveText("Start video transcode");
		const recoveryColors = await page.locator("[data-playback-recovery] [data-player-fallback]").evaluate((button) => {
			const style = getComputedStyle(button);
			return { color: style.color, background: style.backgroundColor };
		});
		expect(recoveryColors, `${viewport.width}px recovery button contrast tokens`).toEqual({ color: "rgb(17, 21, 10)", background: "rgb(200, 241, 105)" });
		expect((await new AxeBuilder({ page }).analyze()).violations, `${viewport.width}px recovery settings accessibility`).toEqual([]);
		expect(await layoutProblems(page), `${viewport.width}px recovery settings`).toEqual({
			documentOverflow: 0,
			outside: [],
			tinyControls: [],
			distortedChecks: [],
			clippedControls: [],
			overlappingStatuses: [],
		});
		await page.screenshot({
			path: testInfo.outputPath(`${viewport.width}-player-recovery-settings.png`),
			fullPage: true,
		});
		await page.keyboard.press("t");
		await expect(page.locator("body")).toHaveClass(/player-theater/);
		await expect.poll(async () => Math.round((await page.locator(".media-stage").boundingBox())?.height ?? 0), { message: "Theater recovery keeps the full viewport" }).toBe(viewport.height);
		await expect(page.getByRole("button", { name: "Close playback settings" })).toBeVisible();
		await page.keyboard.press("t");
	}
	await page.evaluate(() => localStorage.setItem("kinosail-theme", "light"));
	await page.setViewportSize(viewports[0]);
	await page.goto(watch!);
	await page.locator("video").evaluate((video) => {
		video.pause();
		video.dataset.compatibilityMode = "transcode";
		video.dataset.compatibilityLabel = "Transcoding video";
		video.dataset.compatibilityDescription = "This device cannot decode the original video.";
		Object.defineProperty(video, "error", {
			configurable: true,
			value: { code: 4 },
		});
		video.dispatchEvent(new Event("error"));
	});
	await page.locator("[data-playback-mode-status]").click();
	const lightColors = await page.locator("[data-playback-recovery] [data-player-fallback]").evaluate((button) => {
		const style = getComputedStyle(button);
		return { color: style.color, background: style.backgroundColor };
	});
	expect(lightColors).toEqual({
		color: "rgb(255, 255, 255)",
		background: "rgb(34, 56, 0)",
	});
	const header = page.locator(".player-settings header");
	await header.scrollIntoViewIfNeeded();
	expect(await header.evaluate((element) => getComputedStyle(element).color === getComputedStyle(element).backgroundColor)).toBe(false);
	expect((await new AxeBuilder({ page }).analyze()).violations, "light recovery settings accessibility").toEqual([]);
	await page.screenshot({
		path: testInfo.outputPath("1440-light-recovery-settings.png"),
		fullPage: true,
	});
	await page.emulateMedia({ forcedColors: "active" });
	expect((await new AxeBuilder({ page }).analyze()).violations, "forced-colors recovery accessibility").toEqual([]);
	await page.screenshot({
		path: testInfo.outputPath("1440-light-forced-colors-recovery.png"),
		fullPage: true,
	});
	await page.emulateMedia({ forcedColors: "none" });
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
	for (const viewport of [
		{ width: 1440, height: 900 },
		{ width: 1024, height: 768 },
		{ width: 390, height: 844 },
		{ width: 320, height: 800 },
	]) {
		await page.setViewportSize(viewport);
		for (const [route, selector] of [
			[show!, ".media-hero.has-media-backdrop>.media-hero-copy"],
			[watch!, ".player-page.has-media-backdrop .title-block"],
		] as const) {
			await page.goto(route);
			const copy = page.locator(selector);
			await expect(copy).toBeVisible();
			const styles = await copy.evaluate((element) => {
				const computed = getComputedStyle(element);
				return {
					background: computed.backgroundColor,
					padding: computed.padding,
				};
			});
			expect(styles.background, `${route} at ${viewport.width}px copy background`).not.toBe("rgba(0, 0, 0, 0)");
			expect(styles.padding, `${route} at ${viewport.width}px copy padding`).not.toBe("0px");
		}
	}
});
