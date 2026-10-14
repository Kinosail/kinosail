import { submitFixtureLogin } from "./fixture-credential-login";
import { expect, test } from "@playwright/test";
import { createHmac } from "node:crypto";

test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
test.beforeEach(async ({ page }) => page.addInitScript(() => Object.defineProperty(PublicKeyCredential, "isConditionalMediationAvailable", { value: async () => false })));

function totp(): string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567";
	const bits = [...(process.env.KINOSAIL_TEST_TOTP_SECRET ?? "")].map((character) => alphabet.indexOf(character).toString(2).padStart(5, "0")).join("");
	const secret = Buffer.from(bits.match(/.{8}/g)?.map((byte) => Number.parseInt(byte, 2)) ?? []);
	const counter = Buffer.alloc(8);
	counter.writeBigUInt64BE(BigInt(Math.floor(Date.now() / 30_000)));
	const digest = createHmac("sha1", secret).update(counter).digest();
	const offset = digest[19] & 15;
	return ((digest.readUInt32BE(offset) & 0x7fffffff) % 1_000_000).toString().padStart(6, "0");
}

test("player stage stays visible across loading and bandwidth changes", async ({ page }, testInfo) => {
	await page.goto("/login");
	await page.getByLabel("Name").fill("Owner");
	await page.getByLabel("Password", { exact: true }).fill("test-instance-password");
	await submitFixtureLogin(page, totp);
	if (await page.getByRole("link", { name: "Not now" }).isVisible()) await page.getByRole("link", { name: "Not now" }).click();
	await page.getByRole("link", { name: "Movies", exact: true }).click();
	await page.getByRole("link", { name: /Example Movie/ }).click();
	const video = page.locator("video");
	await video.waitFor({ state: "visible" });
	const stage = await page.locator(".media-stage").boundingBox();
	expect(await video.evaluate((element) => element.dataset.hls || element.dataset.adaptive || element.getAttribute("src"))).toMatch(/\/(media|stream|hls)\//);
	expect(stage?.width).toBeGreaterThan(300);
	expect(stage?.height).toBeGreaterThan(150);
	const inherited = await video.evaluate(async (element: HTMLVideoElement) => {
		element.muted = true;
		await element.play();
		const state = { currentTime: element.currentTime, duration: element.duration };
		element.pause();
		// Exercise the retained failure's late-resume condition on every run.
		element.currentTime = element.duration * 0.85;
		return state;
	});
	await expect.poll(() => video.evaluate((element: HTMLVideoElement) =>
		!element.seeking && Math.abs(element.currentTime - element.duration * 0.85) < 0.05 && element.readyState >= HTMLMediaElement.HAVE_CURRENT_DATA)).toBe(true);
	const resumed = await video.evaluate((element: HTMLVideoElement) => {
		const state = { currentTime: element.currentTime, duration: element.duration };
		// Earlier journeys retain real progress. Start this synthetic buffer review
		// at zero so its fixed 60% buffered range contains the playback position.
		element.currentTime = 0;
		return state;
	});
	await expect.poll(() => video.evaluate((element: HTMLVideoElement) =>
		!element.seeking && element.currentTime <= 0.05 && element.readyState >= HTMLMediaElement.HAVE_CURRENT_DATA)).toBe(true);
	const reset = await video.evaluate((element: HTMLVideoElement) => ({ currentTime: element.currentTime, readyState: element.readyState, paused: element.paused, seeking: element.seeking }));
	expect(reset).toMatchObject({ paused: true, seeking: false });
	await video.evaluate((element: HTMLVideoElement) => element.play());
	await expect.poll(() => video.evaluate((element: HTMLVideoElement) => element.currentTime)).toBeGreaterThan(reset.currentTime + 0.25);
	await expect.poll(() => video.evaluate((element: HTMLVideoElement) => element.readyState)).toBeGreaterThanOrEqual(3);
	await video.dispatchEvent("stalled");
	await expect(page.locator("[data-player-status]")).toBeHidden();
	const bufferedFixture = await video.evaluate((element) => {
		element.pause();
		return { currentTime: element.currentTime, duration: element.duration, end: element.duration * 0.6,
			readyState: element.readyState, seeking: element.seeking, paused: element.paused, errorCode: element.error?.code ?? 0 };
	});
	await testInfo.attach("loading-review-buffer-fixture", { contentType: "application/json", body: JSON.stringify({ inherited, resumed, reset, bufferedFixture }) });
	expect(Number.isFinite(bufferedFixture.duration)).toBe(true);
	expect(bufferedFixture).toMatchObject({ seeking: false, paused: true, errorCode: 0 });
	expect(bufferedFixture.readyState).toBeGreaterThanOrEqual(3);
	expect(bufferedFixture.end - bufferedFixture.currentTime, "Synthetic range must contain playable forward data").toBeGreaterThanOrEqual(2);
	await video.evaluate((element) => {
		const end = element.duration * 0.6;
		Object.defineProperty(element, "paused", { configurable: true, get: () => false });
		Object.defineProperty(element, "buffered", { configurable: true, get: () => ({ length: 1, start: () => 0, end: () => end }) });
	});
	await video.dispatchEvent("loadstart");
	await expect(page.getByRole("status").filter({ hasText: "Loading video…" })).toBeVisible();
	await expect(page.locator("[data-buffered]")).toBeHidden();
	for (const viewport of [{ width: 1440, height: 900 }, { width: 1024, height: 768 }, { width: 720, height: 450 }, { width: 390, height: 844 }, { width: 320, height: 800 }]) {
		await page.setViewportSize(viewport);
		await video.dispatchEvent("loadstart");
		expect((await page.locator("[data-player-status]").boundingBox())?.width).toBeLessThanOrEqual(Math.min(320, viewport.width - 16) + 0.01);
		await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-loading.png`), fullPage: true });
		await video.dispatchEvent("canplay");
		await video.dispatchEvent("stalled");
		await expect(page.locator("[data-player-status]")).toBeHidden();
		await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-playing-stall.png`), fullPage: true });
		await video.dispatchEvent("waiting");
		await expect(page.getByRole("status").filter({ hasText: "Buffering" })).toBeVisible();
		await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-buffering.png`), fullPage: true });
		await video.dispatchEvent("playing");
		await expect(page.locator("[data-player-status]")).toBeHidden();
		await page.locator(".media-stage").dispatchEvent("pointermove");
		await page.getByRole("button", { name: "Settings", exact: true }).click();
		const settings = await page.locator(".player-settings").boundingBox();
		expect(settings?.x).toBeGreaterThanOrEqual(0);
		expect((settings?.x ?? 0) + (settings?.width ?? 0)).toBeLessThanOrEqual(viewport.width);
		await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-settings.png`), fullPage: true });
		await page.getByRole("button", { name: "Close playback settings" }).click();
	}
});
