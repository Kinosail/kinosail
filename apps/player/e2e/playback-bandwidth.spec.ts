import { expect, test, type CDPSession, type Page } from "@playwright/test";
import { createHmac } from "node:crypto";

test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
test.beforeEach(async ({ page }) => page.addInitScript(() => {
	Object.defineProperty(PublicKeyCredential, "isConditionalMediationAvailable", { value: async () => false });
	(window as Window & { playbackEvents: Array<{ name: string; seconds: number; readyState: number; status?: string }> }).playbackEvents = [];
	for (const name of ["loadstart", "loadedmetadata", "canplay", "play", "playing", "waiting", "stalled", "seeking", "seeked", "pause", "error"]) {
		document.addEventListener(name, (event) => {
			if (!(event.target instanceof HTMLVideoElement)) return;
			const status = document.querySelector<HTMLElement>("[data-player-status]");
			(window as Window & { playbackEvents: Array<{ name: string; seconds: number; readyState: number; status?: string }> }).playbackEvents.push({ name, seconds: event.target.currentTime, readyState: event.target.readyState, status: status?.hidden ? "hidden" : status?.textContent?.trim() });
		}, true);
	}
}));

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
	await page.goto("/login");
	await page.getByLabel("Name").fill("Owner");
	await page.getByLabel("Password", { exact: true }).fill("test-instance-password");
	await page.getByLabel("Authentication or recovery code").fill(totp());
	await page.getByRole("button", { name: "Sign in", exact: true }).click();
	if (await page.getByRole("link", { name: "Not now" }).isVisible()) await page.getByRole("link", { name: "Not now" }).click();
	await page.getByRole("link", { name: "Movies", exact: true }).click();
}

async function network(session: CDPSession, kilobitsPerSecond: number, latency = 0, offline = false) {
	await session.send("Network.emulateNetworkConditions", {
		offline,
		latency,
		downloadThroughput: offline ? 0 : kilobitsPerSecond * 1024 / 8,
		uploadThroughput: offline ? 0 : Math.max(32, kilobitsPerSecond / 4) * 1024 / 8,
	});
}

async function openMovie(page: Page) {
	await page.locator('a.card[href^="/watch/"]').first().click();
	return page.locator("video");
}

async function playbackState(page: Page) {
	return page.evaluate(() => {
		const video = document.querySelector("video")!;
		const status = document.querySelector<HTMLElement>("[data-player-status]")!;
		return {
			currentTime: video.currentTime,
			paused: video.paused,
			readyState: video.readyState,
			error: video.error?.message,
			status: status.hidden ? "hidden" : status.textContent?.trim(),
			events: (window as Window & { playbackEvents: Array<{ name: string; seconds: number; readyState: number; status?: string }> }).playbackEvents,
		};
	});
}

test("ordinary mobile bandwidth keeps moving video unobstructed", async ({ page, browserName }) => {
	test.skip(browserName !== "chromium", "network emulation uses Chromium DevTools");
	await login(page);
	const session = await page.context().newCDPSession(page);
	await session.send("Network.enable");
	try {
		await network(session, 750, 120);
		const video = await openMovie(page);
		await expect.poll(() => video.evaluate((element) => element.currentTime), { timeout: 15_000 }).toBeGreaterThan(1);
		await expect(page.locator("[data-player-status]")).toBeHidden();
		await page.waitForTimeout(1_500);
		const state = await playbackState(page);
		console.log(JSON.stringify(state));
		expect(state.currentTime).toBeGreaterThan(2);
		expect(state.paused).toBe(false);
		expect(state.error).toBeUndefined();
		expect(state.status).toBe("hidden");
	} finally {
		await network(session, 100_000);
		await session.detach();
	}
});

test("bandwidth starvation keeps Direct Play and recovers in place", async ({ page, browserName }) => {
	test.skip(browserName !== "chromium", "network emulation uses Chromium DevTools");
	await login(page);
	const session = await page.context().newCDPSession(page);
	await session.send("Network.enable");
	try {
		await network(session, 32, 250);
		const video = await openMovie(page);
		await expect(page.locator("[data-playback-mode-status]")).toHaveText(/Direct Play/);
		await expect(video).toHaveAttribute("src", /\/media\//);
		await expect(page.locator("[data-player-status]")).toBeVisible();
		await expect(page.locator("[data-player-message]")).toContainText(/Loading|Buffering/);
		await network(session, 100_000, 5);
		await video.evaluate((element) => element.play());
		await expect.poll(() => video.evaluate((element) => element.currentTime), { timeout: 20_000 }).toBeGreaterThan(0.5);
		await expect(page.locator("[data-player-status]")).toBeHidden();
		const state = await playbackState(page);
		console.log(JSON.stringify(state));
		expect(state.error).toBeUndefined();
	} finally {
		await network(session, 100_000);
		await session.detach();
	}
});

test("an interrupted Direct Play request retries without transcoding", async ({ page, browserName }) => {
	test.skip(browserName !== "chromium", "network emulation uses Chromium DevTools");
	await login(page);
	const session = await page.context().newCDPSession(page);
	await session.send("Network.enable");
	try {
		const video = await openMovie(page);
		await expect.poll(() => video.evaluate((element) => element.currentTime), { timeout: 10_000 }).toBeGreaterThan(0.25);
		await page.route("**/media/**", (route) => route.abort("internetdisconnected"));
		await video.evaluate((element) => { element.load(); void element.play(); });
		await expect(page.locator("[data-player-status]")).toBeVisible();
		await expect(page.locator("[data-player-message]")).toContainText("did not start transcoding");
		await expect(page.locator("[data-player-status] [data-player-fallback]")).toHaveText("Retry Direct Play");
		await page.unroute("**/media/**");
		await page.locator("[data-player-status] [data-player-fallback]").click();
		await expect.poll(() => video.evaluate((element) => element.currentTime), { timeout: 20_000 }).toBeGreaterThan(0.25);
		await expect(page.locator("[data-player-status]")).toBeHidden();
		await expect(page.locator("[data-playback-mode-status]")).toHaveText("Direct Play");
		const state = await playbackState(page);
		console.log(JSON.stringify(state));
		expect(state.paused).toBe(false);
		expect(state.error).toBeUndefined();
		expect(state.events.some(({ name }: { name: string }) => name === "error")).toBe(true);
	} finally {
		await network(session, 100_000);
		await session.detach();
	}
});

test("repeated bandwidth changes do not leave a stale player state", async ({ page, browserName }) => {
	test.skip(browserName !== "chromium", "network emulation uses Chromium DevTools");
	await login(page);
	const session = await page.context().newCDPSession(page);
	await session.send("Network.enable");
	try {
		await network(session, 96, 100);
		const video = await openMovie(page);
		await expect.poll(() => video.evaluate((element) => element.currentTime), { timeout: 25_000 }).toBeGreaterThan(0.25);
		for (const [kilobits, latency] of [[32, 400], [2_000, 40], [16, 600], [750, 120], [48, 250], [100_000, 5]]) {
			await network(session, kilobits, latency);
			await page.waitForTimeout(350);
		}
		await expect.poll(() => video.evaluate((element) => element.currentTime), { timeout: 20_000 }).toBeGreaterThan(3);
		await expect(page.locator("[data-player-status]")).toBeHidden();
		const state = await playbackState(page);
		console.log(JSON.stringify(state));
		expect(state.paused).toBe(false);
		expect(state.error).toBeUndefined();
	} finally {
		await network(session, 100_000);
		await session.detach();
	}
});

test("compatibility streaming recovers after the network disappears", async ({ page, browserName }) => {
	test.skip(browserName !== "chromium", "network emulation uses Chromium DevTools");
	await login(page);
	const session = await page.context().newCDPSession(page);
	await session.send("Network.enable");
	try {
		await network(session, 750, 120);
		const watch = await page.locator('a.card[href^="/watch/"]').first().getAttribute("href");
		expect(watch).toBeTruthy();
		await page.goto(`${watch}?compatible=1`);
		const video = page.locator("video");
		await expect(page.locator("[data-playback-mode-status]")).toHaveText(await video.getAttribute("data-compatibility-label") || "Compatibility");
		await expect.poll(() => video.evaluate((element) => element.currentTime), { timeout: 30_000 }).toBeGreaterThan(0.25);
		await network(session, 0, 0, true);
		await video.evaluate((element) => { element.currentTime = 10; void element.play(); });
		const beforeRecovery = await video.evaluate((element) => element.currentTime);
		await network(session, 100_000, 5);
		await expect.poll(() => video.evaluate((element) => element.currentTime), { timeout: 30_000 }).toBeGreaterThan(beforeRecovery + 0.25);
		await expect(page.locator("[data-player-status]")).toBeHidden();
		const state = await playbackState(page);
		console.log(JSON.stringify(state));
		expect(state.paused).toBe(false);
		expect(state.error).toBeUndefined();
	} finally {
		await network(session, 100_000);
		await session.detach();
	}
});
