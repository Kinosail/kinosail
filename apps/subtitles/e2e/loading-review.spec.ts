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
	await page.getByLabel("6-digit code").fill(totp());
	await page.getByRole("button", { name: "Sign in", exact: true }).click();
	if (await page.getByRole("link", { name: "Not now" }).isVisible()) await page.getByRole("link", { name: "Not now" }).click();
	await page.getByRole("link", { name: "Movies", exact: true }).click();
	await page.getByRole("link", { name: /Example Movie/ }).click();
	const video = page.locator("video");
	await video.waitFor({ state: "visible" });
	const stage = await page.locator(".media-stage").boundingBox();
	expect(await video.evaluate((element) => element.dataset.hls || element.dataset.adaptive || element.getAttribute("src"))).toMatch(/\/(media|stream|hls)\//);
	expect(stage?.width).toBeGreaterThan(300);
	expect(stage?.height).toBeGreaterThan(150);
	await expect.poll(() => video.evaluate((element: HTMLVideoElement) => element.currentTime)).toBeGreaterThan(0.25);
	await video.dispatchEvent("stalled");
	await expect(page.locator("[data-player-status]")).toBeHidden();
	await video.dispatchEvent("loadstart");
	await expect(page.getByRole("status").filter({ hasText: "Loading video…" })).toBeVisible();
	await expect(page.locator("[data-buffered]")).toBeHidden();
	await video.evaluate((element) => {
		element.pause();
		const end = element.duration * 0.6;
		Object.defineProperty(element, "paused", { configurable: true, get: () => false });
		Object.defineProperty(element, "buffered", { configurable: true, get: () => ({ length: 1, start: () => 0, end: () => end }) });
	});
	for (const viewport of [{ width: 1440, height: 900 }, { width: 1024, height: 768 }, { width: 720, height: 450 }, { width: 390, height: 844 }, { width: 320, height: 800 }]) {
		await page.setViewportSize(viewport);
		await video.dispatchEvent("loadstart");
		expect((await page.locator("[data-player-status]").boundingBox())?.width).toBeLessThanOrEqual(Math.min(256, viewport.width - 16) + 0.01);
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
