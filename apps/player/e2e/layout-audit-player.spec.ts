import { expect, test } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { configureLayoutAudit, layoutProblems, login, viewports } from "./layout-audit-helpers";
import { firstPlayable } from "./test-instance-helpers";
import {attachPlaybackState} from "./playback-state-witness.mjs";
import { attachResponsiveFailure } from "./responsive-failure-witness.mjs";
configureLayoutAudit();
test("player shows and switches its playback method without crowding actions", async ({ page }, testInfo) => {
	test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
	await login(page);
	await page.goto("/?view=movies");
	const watch = await firstPlayable(page);
	for (const viewport of viewports) {
		await page.setViewportSize(viewport);
		await page.evaluate(() => localStorage.removeItem("kinosail.playback-policy-v2"));
		await attachPlaybackState(page, testInfo, watch, "before-method");
		await page.goto(watch);
		const video = page.locator("video");
		const stage = page.locator(".media-stage");
		const activePlayback = (media: HTMLVideoElement) =>
			!media.paused && !media.ended && !media.seeking && !media.error && media.readyState >= 2 &&
			Number.isFinite(media.currentTime) && Number.isFinite(media.duration) && media.duration > 0 &&
			media.currentTime >= 0 && media.currentTime < media.duration;
		await stage.focus();
		await expect(stage).toBeFocused();
		if (await video.evaluate((media: HTMLVideoElement) => media.paused)) await page.keyboard.press("Space");
		await expect.poll(() => video.evaluate(activePlayback)).toBe(true);
		await page.keyboard.press("Space");
		await expect(video).toHaveJSProperty("paused", true);
		const actions = page.locator(".primary-player-actions:not([data-progress-notice])");
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
		const playback = page.locator(".player-settings");
		await expect(playback).toBeVisible();
		const compatibleLabel = (await page.locator("video").getAttribute("data-compatibility-label")) || "Compatibility";
		await page.waitForTimeout(250);
		const settingsGeometry = await page.locator(".player-settings").evaluate((panel) => {
			const box = panel.getBoundingClientRect();
			const actions = document.querySelector(".primary-player-actions:not([data-progress-notice])")!.getBoundingClientRect();
			return {
				clear: box.bottom + 8 <= actions.top,
				height: box.height,
				background: getComputedStyle(panel).backgroundColor,
			};
		});
		await attachResponsiveFailure(page, testInfo, "player-settings");
		expect(settingsGeometry.clear).toBeTruthy();
		expect(settingsGeometry.height).toBeGreaterThanOrEqual(200);
		expect(settingsGeometry.background).not.toBe("rgba(0, 0, 0, 0)");
		expect((await new AxeBuilder({ page }).analyze()).violations, `playback settings accessibility at ${viewport.width}px`).toEqual([]);
		await page.screenshot({
			path: testInfo.outputPath(`${viewport.width}-player-compatibility.png`),
			fullPage: true,
		});
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
		await stage.focus();
		await expect(stage).toBeFocused();
		await expect(video).toHaveJSProperty("paused", true);
		const beforePlay = await video.evaluate((media: HTMLVideoElement) => media.currentTime);
		await page.keyboard.press("Space");
		await expect.poll(() => video.evaluate(activePlayback)).toBe(true);
		await expect.poll(() => video.evaluate((media: HTMLVideoElement) => media.currentTime)).toBeGreaterThan(beforePlay);
		await attachResponsiveFailure(page, testInfo, "player-method-before-switch");
		await attachPlaybackState(page, testInfo, watch, "at-method-switch");
		await actions.getByRole("link", { name: compatibleLabel }).click();
		await expect(method).toHaveText(compatibleLabel);
		await attachPlaybackState(page, testInfo, watch, "after-method-switch");
		await attachResponsiveFailure(page, testInfo, "player-method-after-switch");
		try {
			await expect.poll(() => page.locator("video").evaluate((video: HTMLVideoElement) => video.currentTime), { timeout: 20_000 }).toBeGreaterThan(0.25);
		} catch (error) {
			await attachResponsiveFailure(page, testInfo, "player-settings"); throw error;
		}
	}
});

test("player stays accessible in alternate display modes", async ({ page }, testInfo) => {
	test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
	await page.addInitScript(() => localStorage.setItem("kinosail-theme", "dark"));
	await page.emulateMedia({ reducedMotion: "reduce" });
	await login(page);
	await page.goto("/?view=movies");
	const watch = await firstPlayable(page);
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
						.some((duration) => Number.parseFloat(duration) > 0.01),
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

test("artwork-backed media copy remains readable", async ({ page }) => {
	test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
	await login(page);
	await page.goto("/?view=shows");
	const show = await page.locator('a.show-details[href^="/show/"]').first().getAttribute("href");
	await page.goto("/?view=movies");
	const watch = await firstPlayable(page);
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
			if (route === show) {
				const art = await page.locator(".media-hero>.media-backdrop").boundingBox();
				const text = await copy.boundingBox();
				expect(art && text && (art.x + art.width <= text.x || art.y + art.height <= text.y), `${route} at ${viewport.width}px artwork stays clear of text`).toBe(true);
			} else {
				expect(styles.background, `${route} at ${viewport.width}px copy background`).not.toBe("rgba(0, 0, 0, 0)");
				expect(styles.padding, `${route} at ${viewport.width}px copy padding`).not.toBe("0px");
			}
			expect((await new AxeBuilder({ page }).include(selector).analyze()).violations).toEqual([]);
		}
	}
});
