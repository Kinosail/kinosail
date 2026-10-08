import {expect, test} from "@playwright/test";
import {login} from "./test-instance-helpers";
import {attachResponsiveFailure} from "./responsive-failure-witness.mjs";
import {holdStartupMedia} from "./startup-media-hold.mjs";

test.skip(!process.env.KINOSAIL_TEST_INSTANCE, "requires the populated test instance");
test.use({serviceWorkers: "block"});
test.beforeEach(async ({page}) => page.addInitScript(() => Object.defineProperty(PublicKeyCredential, "isConditionalMediationAvailable", {value: async () => false})));

for (const source of ["direct", "compatible", "automatic"]) for (const savedPosition of [0, 1]) test(`blocked autoplay leaves one Play control that starts ${source} video from ${savedPosition ? "saved progress" : "the beginning"}`, { tag: ["compatible", "automatic"].includes(source) && savedPosition ? ["@smoke"] : [] }, async ({ page, browserName }, testInfo) => {
	await page.setViewportSize({ width: 390, height: 844 });
	// Exercise WebKit's native HLS adapter, as mobile Safari does for automatic compatibility.
	if (browserName === "webkit") await page.route("**/static/hls.min.js*", (route) => route.fulfill({ contentType: "application/javascript", body: "" }));
	await page.addInitScript((apple) => {
		if (apple) Object.defineProperty(navigator, "userAgent", { configurable: true, value: "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 Mobile/15E148" });
		const nativePlay = HTMLMediaElement.prototype.play;
		let blocked = true;
		const observer = new MutationObserver(() => {
			const video = document.querySelector("video[autoplay]");
			if (!video) return;
			video.removeAttribute("autoplay");
			video.setAttribute("data-autoplay", "");
			observer.disconnect();
		});
		observer.observe(document, { childList: true, subtree: true });
		HTMLMediaElement.prototype.play = function () {
			return blocked && !this.muted ? Promise.reject(new DOMException("A tap is required", "NotAllowedError")) : nativePlay.call(this);
		};
		(window as Window & { allowVideoPlay: () => void }).allowVideoPlay = () => { blocked = false; };
	}, browserName === "webkit");
	await login(page);
	await page.goto("/?view=movies");
	const hold = await holdStartupMedia(page, source);
	let phase = "opening", failed = false;
	try {
		const movie = page.getByRole("link", { name: /Example Movie/ });
	  const watch = (await movie.getAttribute("href"))!;
	  await page.evaluate(async ({ id, seconds }) => {
	    const csrf = document.querySelector<HTMLMetaElement>('meta[name="kinosail-csrf"]')!.content;
	    const response = await fetch(`/api/v1/items/${id}/progress`, { method: "PUT",
	      headers: { "Content-Type": "application/json", "X-Kinosail-CSRF": csrf }, body: JSON.stringify({ seconds, watched: false }) });
	    if (!response.ok) throw new Error(`reset isolated movie progress: ${response.status}`);
	  }, { id: watch.split("/").at(-1)!, seconds: savedPosition });
		if (source === "compatible") await page.goto(`${await movie.getAttribute("href")}?compatible=1`, { waitUntil: "domcontentloaded" });
		else await movie.click({ noWaitAfter: true });
		await expect(page).toHaveURL(/\/watch\/[a-f0-9]+(?:\?compatible=1)?$/);
		phase = "pending";
		if (browserName === "webkit") {
			const appleNativePlayback = await page.locator("video").evaluate((video: HTMLVideoElement & {webkitEnterFullscreen?: () => void}) => /iPhone/.test(navigator.userAgent) && typeof video.webkitEnterFullscreen === "function");
			expect(appleNativePlayback).toBe(true);
			await expect(page.locator("video")).toHaveJSProperty("autoplay", false);
			await expect(page.locator("[data-player-status]")).toBeHidden();
			await expect(page.locator(".player-center-control[data-player-toggle]")).toBeVisible();
			await page.evaluate(() => (window as Window & {allowVideoPlay: () => void}).allowVideoPlay());
			await page.locator(".player-center-control[data-player-toggle]").click();
		}
		await expect.poll(() => hold.snapshot().mediaEntered + hold.snapshot().hlsEntered, { message: "owned startup media route must be held" }).toBeGreaterThan(0);
		if (browserName !== "webkit") {
			await expect(page.locator("[data-player-status]")).toBeVisible();
			await page.waitForTimeout(2_000);
			await expect(page.locator("[data-player-status]")).toBeVisible();
			await expect(page.locator(".player-center-control[data-player-toggle]")).toBeHidden();
		}
		// WebKit waits for document.fonts.ready, which needs the held media load to finish.
		if (browserName !== "webkit") await page.screenshot({ path: testInfo.outputPath("390-media-pending.png"), fullPage: true });
		// Deliver a reachable original that the real decoder rejects; compatibility media
		// still comes from the real server and FFmpeg, with the full preparation UI.
		if (source === "automatic") await page.locator("video").evaluate((video) => { video.dataset.compatibilityMode = "transcode"; });
		phase = "readiness"; hold.release();
		if (browserName === "webkit") {
			await expect.poll(() => page.locator("video").evaluate((video: HTMLVideoElement) => video.currentTime), {timeout: 20_000}).toBeGreaterThan(savedPosition + 0.25);
			await page.locator("video").evaluate((video: HTMLVideoElement & {webkitDisplayingFullscreen?: boolean; webkitExitFullscreen?: () => void}) => {if (video.webkitDisplayingFullscreen) video.webkitExitFullscreen?.(); video.pause();});
			await expect(page.locator("video")).toHaveJSProperty("paused", true);
		}
		await expect(page.locator("[data-player-status]")).toBeHidden();
		await expect(page.locator("[data-player-status] [data-player-fallback]")).toBeHidden();
		const readiness = await page.locator("video").evaluate((video: HTMLVideoElement) => {
			const mediaTime = Object.getOwnPropertyDescriptor(HTMLMediaElement.prototype, "currentTime")!.get!.call(video) as number;
			let ahead = 0, gap = Infinity;
			for (let index = 0; index < video.buffered.length; index++) {
				const start = video.buffered.start(index);
				if (video.buffered.end(index) >= mediaTime) {
					gap = Math.min(gap, Math.max(0, start - mediaTime));
					ahead = Math.max(ahead, video.buffered.end(index) - Math.max(mediaTime, start));
				}
			}
			return { readyState: video.readyState, ahead, gap, remaining: video.duration - video.currentTime, position: video.currentTime };
		});
		// Paused WebKit can settle at HAVE_CURRENT_DATA after a buffered seek.
		// Prove the required tap advances from the resume position below.
		expect(readiness.readyState).toBeGreaterThanOrEqual(browserName === "webkit" ? 2 : 3);
		expect(readiness.gap).toBeLessThanOrEqual(0.05);
		expect(readiness.ahead).toBeGreaterThanOrEqual(Math.min(2, readiness.remaining));
		phase = "play-control";
		await expect(page.locator(".player-center-control[data-player-toggle]")).toBeVisible();
		await expect(page.getByRole("button", { name: "Play video" })).toHaveCount(0);
		await page.screenshot({ path: testInfo.outputPath("390-play-control.png"), fullPage: true });
		await page.getByRole("button", { name: "Settings", exact: true }).click();
		const speed = page.getByRole("combobox", { name: "Playback speed" });
		await expect(speed).toBeVisible();
		await speed.selectOption("1.5");
		await expect(page.locator("video")).toHaveJSProperty("playbackRate", 1.5);
		await expect(page.locator("video")).toHaveJSProperty("paused", true);
		await page.screenshot({ path: testInfo.outputPath("390-playback-settings.png"), fullPage: true });
		await speed.selectOption("1");
		await page.getByRole("button", { name: "Close playback settings" }).click();
		phase = "gesture";
		await page.evaluate(() => (window as Window & { allowVideoPlay: () => void }).allowVideoPlay());
		await page.locator(".player-center-control[data-player-toggle]").click();
		await expect.poll(() => page.locator("video").evaluate((video: HTMLVideoElement) => video.currentTime)).toBeGreaterThan(readiness.position + 0.25);
		await expect(page.locator("[data-player-status]")).toBeHidden();
		await expect(page.locator("video")).toHaveJSProperty("muted", false);
		phase = "seek";
		if (source === "compatible" && savedPosition && browserName === "webkit") {
			// Safari preparation also restores the start of the loaded native HLS window.
			// Use real media seeking and decoding so a frozen application timeline is observable.
			const originalSource = await page.locator("video").evaluate((video: HTMLVideoElement) => video.currentSrc);
			await page.locator("video").evaluate((video: HTMLVideoElement) => { video.pause(); video.currentTime = Number(video.dataset.start) + 0.5; });
			await expect.poll(() => page.locator("video").evaluate((video: HTMLVideoElement) => video.seeking)).toBe(false);
			await expect(page.locator("video")).toHaveJSProperty("currentSrc", originalSource);
			await page.locator(".player-center-control[data-player-toggle]").click();
			await expect.poll(() => page.locator("video").evaluate((video: HTMLVideoElement) => video.currentTime), { timeout: 5_000 }).toBeGreaterThan(savedPosition + 0.5);
			await expect(page.locator("[data-player-status]")).toBeHidden();
			await page.screenshot({ path: testInfo.outputPath("390-resumed-after-seek.png"), fullPage: true });
			// A paused seek outside this offset window loads another native stream.
			// Its first playable sample can be later than the requested timestamp.
			await page.locator("video").evaluate((video: HTMLVideoElement) => { video.pause(); video.currentTime = 0.3; });
			await expect.poll(() => page.locator("video").evaluate((video: HTMLVideoElement) => video.currentSrc)).not.toBe(originalSource);
			await expect(page.locator("[data-player-status]")).toBeHidden();
			await expect(page.locator("video")).toHaveJSProperty("paused", true);
			const seekPosition = await page.locator("video").evaluate((video: HTMLVideoElement) => video.currentTime);
			await page.locator(".player-center-control[data-player-toggle]").click();
			await expect.poll(() => page.locator("video").evaluate((video: HTMLVideoElement) => video.currentTime)).toBeGreaterThan(seekPosition + 0.25);
			await expect(page.locator("[data-player-status]")).toBeHidden();
			await page.screenshot({ path: testInfo.outputPath("390-resumed-after-far-seek.png"), fullPage: true });
		}
	} catch (error) {
		failed = true;
		await attachResponsiveFailure(page, testInfo, "player-startup");
		await hold.attach(testInfo, phase); // Snapshot before cleanup releases the owned hold.
		throw error;
	} finally {
		try {if (!await hold.close() && !failed) throw new Error("startup route cleanup did not join");}
		catch (error) {if (!failed) throw error;}
	}
});
