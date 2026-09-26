import { expect, test } from "@playwright/test";
import type { Page } from "@playwright/test";
import { createHash, createHmac } from "node:crypto";

test.skip(!process.env.KINOSAIL_TEST_INSTANCE, "requires the populated test instance");
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

async function instrumentMedia(page: Page) {
	await page.addInitScript(() => {
		(window as Window & { mediaEvents: Array<{ name: string; at: number }> }).mediaEvents = [];
		for (const name of ["loadstart", "loadedmetadata", "canplay", "play", "playing", "waiting", "stalled", "error"]) {
			document.addEventListener(name, (event) => {
				if (event.target instanceof HTMLVideoElement) (window as Window & { mediaEvents: Array<{ name: string; at: number }> }).mediaEvents.push({ name, at: performance.timeOrigin + performance.now() });
			}, true);
		}
	});
}

test("selecting a movie starts moving playback promptly", async ({ page }, testInfo) => {
	await instrumentMedia(page);
	await page.goto("/login");
	await page.getByLabel("Name").fill("Owner");
	await page.getByLabel("Password", { exact: true }).fill("test-instance-password");
	await page.getByLabel("Authentication or recovery code").fill(totp());
	await page.getByRole("button", { name: "Sign in", exact: true }).click();
	if (await page.getByRole("link", { name: "Not now" }).isVisible()) await page.getByRole("link", { name: "Not now" }).click();
	await page.getByRole("link", { name: "Movies", exact: true }).click();

	await page.getByRole("link", { name: /Example Movie/ }).click();
	const started = Date.now();
	await page.getByRole("link", { name: /^(Play|Resume|Play again)$/ }).click();
	const video = page.locator("video");
	await expect.poll(() => video.evaluate((element: HTMLVideoElement) => element.currentTime), { timeout: 3_000 }).toBeGreaterThan(0.25);
	const result = await page.evaluate((click) => {
		const events = (window as Window & { mediaEvents: Array<{ name: string; at: number }> }).mediaEvents;
		return { clickToMovingMs: Date.now() - click, events: events.map(({ name, at }) => ({ name, ms: Math.round(at - click) })) };
	}, started);
	console.log(JSON.stringify(result));
	expect(result.clickToMovingMs).toBeLessThan(2_000);
	const frames: string[] = [];
	for (let index = 0; index < 3; index++) {
		frames.push(createHash("sha256").update(await video.screenshot()).digest("hex"));
		await page.waitForTimeout(200);
	}
	expect(new Set(frames).size).toBeGreaterThan(1);
	for (const viewport of [{ width: 1440, height: 900 }, { width: 1024, height: 768 }, { width: 720, height: 450 }, { width: 390, height: 844 }, { width: 320, height: 800 }]) {
		await page.setViewportSize(viewport);
		await expect(page.locator("[data-player-controls]")).toHaveClass(/is-idle/);
		await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-playing-player.png`), fullPage: true });
	}
});

test("restricted browser storage does not stop playback", async ({ page }) => {
	await page.addInitScript(() => Object.defineProperty(window, "localStorage", { configurable: true, get: () => { throw new DOMException("blocked", "SecurityError"); } }));
	const errors: string[] = [];
	page.on("pageerror", (error) => errors.push(error.message));
	await page.goto("/login");
	await page.getByLabel("Name").fill("Owner");
	await page.getByLabel("Password", { exact: true }).fill("test-instance-password");
	await page.getByLabel("Authentication or recovery code").fill(totp());
	await page.getByRole("button", { name: "Sign in", exact: true }).click();
	if (await page.getByRole("link", { name: "Not now" }).isVisible()) await page.getByRole("link", { name: "Not now" }).click();
	await page.getByRole("link", { name: "Movies", exact: true }).click();
	await page.getByRole("link", { name: /Example Movie/ }).click();
	await page.getByRole("link", { name: /^(Play|Resume|Play again)$/ }).click();
	const video = page.locator("video");
	await expect.poll(() => video.evaluate((element: HTMLVideoElement) => element.currentTime), { timeout: 5_000 }).toBeGreaterThan(0.25);
	expect(errors).toEqual([]);
});

test("failed progress save does not stop playback flow", async ({ page }) => {
	await page.goto("/login");
	await page.getByLabel("Name").fill("Owner");
	await page.getByLabel("Password", { exact: true }).fill("test-instance-password");
	await page.getByLabel("Authentication or recovery code").fill(totp());
	await page.getByRole("button", { name: "Sign in", exact: true }).click();
	if (await page.getByRole("link", { name: "Not now" }).isVisible()) await page.getByRole("link", { name: "Not now" }).click();
	await page.getByRole("link", { name: "Movies", exact: true }).click();
	await page.getByRole("link", { name: /Example Movie/ }).click();
	await page.getByRole("link", { name: /^(Play|Resume|Play again)$/ }).click();
	await page.route("**/progress/**", (route) => route.abort());
	await page.locator("video").evaluate((video: HTMLVideoElement) => {
		video.dataset.next = "/?view=movies&after=failed-save";
		video.dispatchEvent(new Event("ended"));
	});
	await page.waitForURL("**/?view=movies&after=failed-save");
});

test("selecting compatibility playback starts without a second play click", async ({ page }) => {
	await page.addInitScript(() => {
		Object.defineProperty(Object.getPrototypeOf(navigator), "mediaCapabilities", { configurable: true, get: () => ({ decodingInfo: async () => ({ supported: true, smooth: true, powerEfficient: true }) }) });
	});
	await instrumentMedia(page);
	await page.goto("/login");
	await page.getByLabel("Name").fill("Owner");
	await page.getByLabel("Password", { exact: true }).fill("test-instance-password");
	await page.getByLabel("Authentication or recovery code").fill(totp());
	await page.getByRole("button", { name: "Sign in", exact: true }).click();
	if (await page.getByRole("link", { name: "Not now" }).isVisible()) await page.getByRole("link", { name: "Not now" }).click();
	await page.getByRole("link", { name: "Movies", exact: true }).click();
	await page.getByRole("link", { name: /Example Movie/ }).click();
	await page.getByRole("link", { name: /^(Play|Resume|Play again)$/ }).click();

	const started = Date.now();
	await page.getByText("Playback & downloads", { exact: true }).click();
	await page.locator(".more-player-actions a.mode").filter({ hasText: "Playback" }).click();
	const video = page.locator("video");
	await expect.poll(() => video.evaluate((element: HTMLVideoElement) => element.currentTime), { timeout: 20_000 }).toBeGreaterThan(0.25);
	const state = await video.evaluate((element: HTMLVideoElement) => ({ paused: element.paused, seconds: element.currentTime }));
	const playing = await page.evaluate((click) => (window as Window & { mediaEvents: Array<{ name: string; at: number }> }).mediaEvents.find(({ name }) => name === "playing")!.at - click, started);
	console.log(JSON.stringify({ compatibilityClickToPlayingMs: Math.round(playing), ...state }));
	expect(state.paused).toBe(false);
	await expect(page.locator("[data-playback-mode-status]")).toHaveText(await video.getAttribute("data-compatibility-label") || "Compatibility");
	expect(playing).toBeLessThan(10_000);
});

test("trusted source opening sets the first media time", async ({ page, browserName }) => {
	await page.addInitScript(() => {
		(window as Window & { firstPresentedMediaTime?: number }).firstPresentedMediaTime = undefined;
		const observer = new MutationObserver(() => {
			const video = document.querySelector("video");
			if (!video) return;
			observer.disconnect();
			video.addEventListener("canplay", () => { (window as Window & { firstReadyMediaTime: number }).firstReadyMediaTime = video.currentTime; }, { once: true });
			video.requestVideoFrameCallback((_, { mediaTime }) => { (window as Window & { firstPresentedMediaTime: number }).firstPresentedMediaTime = mediaTime; });
		});
		observer.observe(document, { childList: true, subtree: true });
	});
	await page.goto("/login");
	await page.getByLabel("Name").fill("Owner");
	await page.getByLabel("Password", { exact: true }).fill("test-instance-password");
	await page.getByLabel("Authentication or recovery code").fill(totp());
	await page.getByRole("button", { name: "Sign in", exact: true }).click();
	if (await page.getByRole("link", { name: "Not now" }).isVisible()) await page.getByRole("link", { name: "Not now" }).click();
	await expect(page.locator('meta[name="kinosail-csrf"]')).toHaveAttribute("content", /.+/);
	const state = await page.evaluate(async () => {
		const csrf = document.querySelector<HTMLMetaElement>('meta[name="kinosail-csrf"]')?.content ?? "";
		const settings = await fetch("/api/v1/settings").then((response) => response.json());
		const shows = await fetch("/api/v1/shows").then((response) => response.json());
		const show = shows.shows.find((value: { title: string }) => value.title === "Example Show");
		const episodes = await fetch(`/api/v1/shows/${show.id}`).then((response) => response.json());
		const item = episodes.episodes.find((value: { episode: number }) => value.episode === 2);
		const change = async (path: string, method: string, body?: object) => {
			const response = await fetch(path, { method, headers: { "Content-Type": "application/json", "X-Kinosail-CSRF": csrf }, body: body === undefined ? undefined : JSON.stringify(body) });
			if (!response.ok) throw new Error(`${method} ${path}: ${response.status} ${await response.text()}`);
		};
		await change(`/api/v1/items/${item.id}/progress`, "PUT", { seconds: 1, watched: false });
		await change(`/api/v1/items/${item.id}/progress`, "PUT", { seconds: 0, watched: false });
		const playback = await fetch(`/api/v1/items/${item.id}/playback`).then((response) => response.json());
		const detail = await fetch(`/api/v1/items/${item.id}`).then((response) => response.json());
		if (!settings.autoSkip.includes("intro") || (playback.markers ?? []).some((marker: { type: string }) => marker.type === "intro")) throw new Error("opening-offset fixture state is invalid");
		if (detail.item.progress.seconds || detail.item.progress.watched || detail.item.progress.dismissed) throw new Error("opening-offset fixture progress is not empty");
		await change(`/api/v1/items/${item.id}/markers`, "PUT", { type: "intro", start: 0, end: 1 });
		return { id: item.id, csrf };
	});
	await page.route(`**/progress/${state.id}**`, (route) => route.fulfill({ status: 204 }));
	try {
		await page.goto(`/watch/${state.id}?direct=1`);
		const video = page.locator("video");
		await expect(video).toHaveAttribute("data-start", "1");
		await expect.poll(() => page.evaluate(() => (window as Window & { firstReadyMediaTime?: number }).firstReadyMediaTime ?? -1), { timeout: 30_000 }).toBeGreaterThanOrEqual(1);
		if (browserName === "chromium") await expect.poll(() => page.evaluate(() => (window as Window & { firstPresentedMediaTime?: number }).firstPresentedMediaTime ?? -1), { timeout: 30_000 }).toBeGreaterThanOrEqual(1);
	} finally {
		await page.goto("/");
		await page.evaluate(async ({ id, csrf }) => {
			const headers = { "Content-Type": "application/json", "X-Kinosail-CSRF": csrf };
			const response = await fetch(`/api/v1/items/${id}/markers/intro`, { method: "DELETE", headers });
			if (!response.ok) throw new Error(`DELETE marker: ${response.status} ${await response.text()}`);
		}, state);
	}
});
