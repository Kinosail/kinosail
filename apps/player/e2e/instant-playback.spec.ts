import { expect, test, type Page } from "@playwright/test";
import { createHash, createHmac } from "node:crypto";

test.skip(!process.env.KINOSAIL_TEST_INSTANCE, "requires the populated test instance");

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

async function login(page: Page) {
	await page.addInitScript(() => {
		Object.defineProperty(PublicKeyCredential, "isConditionalMediationAvailable", { value: async () => false });
		(window as Window & { mediaEvents: Array<{ name: string; at: number }> }).mediaEvents = [];
		(window as Window & { mediaFrames: Array<{ at: number; mediaTime: number; presentedFrames: number }> }).mediaFrames = [];
		for (const name of ["loadstart", "loadedmetadata", "canplay", "play", "playing", "waiting", "stalled", "error"]) {
			document.addEventListener(name, (event) => {
				if (!(event.target instanceof HTMLVideoElement)) return;
				(window as Window & { mediaEvents: Array<{ name: string; at: number }> }).mediaEvents.push({ name, at: performance.timeOrigin + performance.now() });
				if (name !== "loadstart" || !event.target.requestVideoFrameCallback) return;
				const record = (_: number, metadata: VideoFrameCallbackMetadata) => {
					(window as Window & { mediaFrames: Array<{ at: number; mediaTime: number; presentedFrames: number }> }).mediaFrames.push({ at: performance.timeOrigin + performance.now(), mediaTime: metadata.mediaTime, presentedFrames: metadata.presentedFrames });
					event.target.requestVideoFrameCallback(record);
				};
				event.target.requestVideoFrameCallback(record);
			}, true);
		}
	});
	await page.goto("/login");
	await page.getByLabel("Name").fill("Owner");
	await page.getByLabel("Password").fill("test-instance-password");
	await page.getByLabel("Authentication or recovery code").fill(totp());
	await page.getByRole("button", { name: "Sign in", exact: true }).click();
	if (await page.getByRole("link", { name: "Not now" }).isVisible()) await page.getByRole("link", { name: "Not now" }).click();
}

test("Play on a movie detail reaches moving video in under two seconds", async ({ page }) => {
	await login(page);
	await page.getByRole("link", { name: "Movies", exact: true }).click();
	await page.getByRole("link", { name: /Example Movie/ }).click();
	const started = Date.now();
	await page.getByRole("link", { name: /^(Play|Resume)$/ }).click();
	const video = page.locator("video");
	await expect.poll(() => page.evaluate(() => (window as Window & { mediaFrames: Array<{ mediaTime: number }> }).mediaFrames.some((frame, index, frames) => index > 0 && frame.mediaTime > frames[0].mediaTime)), { timeout: 3_000 }).toBe(true);
	const result = await page.evaluate((click) => {
		const navigation = performance.getEntriesByType("navigation")[0] as PerformanceNavigationTiming;
		const events = (window as Window & { mediaEvents: Array<{ name: string; at: number }> }).mediaEvents;
		const frames = (window as Window & { mediaFrames: Array<{ at: number; mediaTime: number; presentedFrames: number }> }).mediaFrames;
		return {
			clickToMovingMs: Math.round(frames.find((frame, index) => index > 0 && frame.mediaTime > frames[0].mediaTime)!.at - click),
			navigation: {
				serverMs: Math.round(navigation.responseStart - navigation.requestStart),
				responseMs: Math.round(navigation.responseEnd - navigation.requestStart),
				domContentLoadedMs: Math.round(navigation.domContentLoadedEventEnd - navigation.startTime),
			},
			events: events.map(({ name, at }) => ({ name, ms: Math.round(at - click) })),
			frames: frames.slice(0, 3).map(({ at, mediaTime, presentedFrames }) => ({ ms: Math.round(at - click), mediaTime, presentedFrames })),
			resources: performance.getEntriesByType("resource").map((entry) => entry as PerformanceResourceTiming).filter(({ name }) => name.includes("/static/") || name.includes("/media/")).map(({ name, initiatorType, startTime, responseEnd, transferSize }) => ({ name: new URL(name).pathname, initiatorType, startMs: Math.round(startTime), endMs: Math.round(responseEnd), transferSize })),
		};
	}, started);
	console.log(JSON.stringify(result));
	expect(result.clickToMovingMs).toBeLessThan(2_000);
	expect(result.resources.some(({ name }) => name.endsWith("/hls.min.js"))).toBe(false);
	const frames: string[] = [];
	for (let index = 0; index < 3; index++) {
		frames.push(createHash("sha256").update(await video.screenshot()).digest("hex"));
		await page.waitForTimeout(200);
	}
	expect(new Set(frames).size).toBeGreaterThan(1);
});
