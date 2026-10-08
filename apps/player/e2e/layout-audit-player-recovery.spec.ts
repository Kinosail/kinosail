import {expect, test} from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import {configureLayoutAudit, expectSettingsCloseFocused, expectTheaterEditingGuard, expectRecoveryContrast, layoutProblems, login, viewports} from "./layout-audit-helpers";
import {firstPlayable, saveSubtitleChoices} from "./test-instance-helpers";
import {attachResponsiveFailure} from "./responsive-failure-witness.mjs";
import {withRecoveryControls} from "./recovery-controls-owner.mjs";

configureLayoutAudit();

test("player explains an unconfirmed failure and offers a direct retry", async ({ page }, testInfo) => {
	test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
	await login(page);
	await page.goto("/?view=movies");
	const watch = await firstPlayable(page);
	await withRecoveryControls(page, saveSubtitleChoices, async (mode: "custom" | "native") => {
	for (const viewport of [viewports[0], viewports[viewports.length - 1]]) {
		await page.setViewportSize(viewport);
		await page.goto(watch!);
		await expect(page.locator("video")).toHaveAttribute("data-subtitle-picker-limited", String(mode === "custom"));
		await expect(page.locator("[data-theater]")).toHaveCount(mode === "custom" ? 1 : 0);
		await page.locator("video").evaluate((video) => {
			video.pause();
			video.dataset.compatibilityMode = "transcode";
			video.dataset.compatibilityLabel = "Transcoding video";
			video.dataset.compatibilityDescription = "This device cannot decode the original video.";
			Object.defineProperty(video, "error", {
				configurable: true,
				value: { code: 0 },
			});
			video.dispatchEvent(new Event("error"));
		});
		const method = page.locator("[data-playback-mode-status]");
		await expect(method).toHaveText("Direct Play stopped");
		await expect(page.locator("[data-player-status] [data-player-fallback]")).toHaveText("Retry playback");
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
			path: testInfo.outputPath(`${mode}-${viewport.width}-player-recovery-overlay.png`),
			fullPage: true,
		});
		await method.focus();
		await page.keyboard.press("Enter");
		await expect(page.locator("[data-playback-recovery]")).toBeVisible();
		expect((await page.locator(".player-settings").boundingBox())?.height).toBeGreaterThanOrEqual(200);
		await attachResponsiveFailure(page, testInfo, "player-recovery");
		await expectSettingsCloseFocused(page);
		await expect(page.locator("[data-player-status]")).toBeHidden();
		await expect(page.locator("[data-playback-recovery] [data-player-fallback]")).toHaveText("Retry playback");
		const recoveryColors = await page.locator("[data-playback-recovery] [data-player-fallback]").evaluate((button) => {
			const style = getComputedStyle(button);
			return { color: style.color, background: style.backgroundColor };
		});
		expect(recoveryColors.color).not.toBe(recoveryColors.background);
		expect(recoveryColors.background).not.toBe("rgba(0, 0, 0, 0)");
		await expectRecoveryContrast(page, `${viewport.width}px recovery settings`);
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
			path: testInfo.outputPath(`${mode}-${viewport.width}-player-recovery-settings.png`),
			fullPage: true,
		});
		if (mode === "custom") {
		await expectTheaterEditingGuard(page);
		await page.keyboard.press("t");
		await expect(page.locator("body")).toHaveClass(/player-theater/);
		await expect.poll(async () => Math.round((await page.locator(".media-stage").boundingBox())?.height ?? 0), { message: "Theater recovery keeps the full viewport" }).toBe(viewport.height);
		await expect(page.getByRole("button", { name: "Close playback settings" })).toBeVisible();
		await page.keyboard.press("t");
		} else {
			await expect(page.locator("video")).toHaveJSProperty("controls", true);
			const supported = await page.locator("video").evaluate((video: HTMLVideoElement & {webkitEnterFullscreen?: () => void}) => Boolean(document.fullscreenEnabled && video.requestFullscreen || video.webkitEnterFullscreen));
			const fullscreen = page.getByRole("button", {name: "Enter fullscreen", exact: true});
			if (supported) {
				await expect(fullscreen).toBeEnabled();
				await fullscreen.click();
				await expect.poll(() => page.locator("video").evaluate((video: HTMLVideoElement & {webkitDisplayingFullscreen?: boolean}) => document.fullscreenElement === video || video.webkitDisplayingFullscreen === true)).toBe(true);
				await page.locator("video").evaluate(async (video: HTMLVideoElement & {webkitExitFullscreen?: () => void}) => {if (document.fullscreenElement) await document.exitFullscreen(); else video.webkitExitFullscreen?.();});
			} else await expect(fullscreen).toBeDisabled();
			await expect(page.locator("body")).not.toHaveClass(/player-theater/);
			await expect(page.locator("[data-playback-recovery] [data-player-fallback]")).toBeVisible();
		}
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
			value: { code: 0 },
		});
		video.dispatchEvent(new Event("error"));
	});
	await page.locator("[data-playback-mode-status]").click();
	const lightColors = await page.locator("[data-playback-recovery] [data-player-fallback]").evaluate((button) => {
		const style = getComputedStyle(button);
		return { color: style.color, background: style.backgroundColor };
	});
	expect(lightColors.color).not.toBe(lightColors.background);
	expect(lightColors.background).not.toBe("rgba(0, 0, 0, 0)");
	await expectRecoveryContrast(page, "light recovery settings");
	const header = page.locator(".player-settings header");
	await header.scrollIntoViewIfNeeded();
	expect(await header.evaluate((element) => getComputedStyle(element).color === getComputedStyle(element).backgroundColor)).toBe(false);
	expect((await new AxeBuilder({ page }).analyze()).violations, "light recovery settings accessibility").toEqual([]);
	await page.screenshot({
		path: testInfo.outputPath(`${mode}-1440-light-recovery-settings.png`),
		fullPage: true,
	});
	await page.emulateMedia({ forcedColors: "active" });
	expect((await new AxeBuilder({ page }).analyze()).violations, "forced-colors recovery accessibility").toEqual([]);
	await page.screenshot({
		path: testInfo.outputPath(`${mode}-1440-light-forced-colors-recovery.png`),
		fullPage: true,
	});
	await page.emulateMedia({ forcedColors: "none" });
	});
});

