import { expect, test } from "@playwright/test";
import { configureTestInstance, login } from "./test-instance-helpers";

configureTestInstance();
test.use({ serviceWorkers: "block" });

test("Collections keep poster geometry while artwork loads", async ({ page, browserName }, testInfo) => {
	await login(page);
	let release!: () => void;
	const pending = new Promise<void>(resolve => { release = resolve; });
	await page.route("**/art/**", async route => {
		await pending;
		await route.continue();
	});
	await page.goto("/?view=collections", { waitUntil: "domcontentloaded" });
	const card = page.locator(".curation-card").filter({ has: page.locator("img") }).first();
	const poster = card.locator(".curation-poster");
	try {
		await expect(card).toBeVisible();
		await expect(card.locator("strong")).toBeVisible();
		const image = card.locator("img").first();
		await expect.poll(() => image.evaluate((image: HTMLImageElement) => image.complete)).toBe(false);
		const before = await poster.boundingBox();
		expect(before!.height / before!.width).toBeCloseTo(1.5, 1);
		if (browserName !== "webkit") await page.screenshot({ path: testInfo.outputPath("collections-pending.png") });
		release();
		await expect.poll(() => image.evaluate((image: HTMLImageElement) => image.complete && image.naturalWidth > 0)).toBe(true);
		const after = await poster.boundingBox();
		expect(after).toEqual(before);
		await testInfo.attach("artwork-geometry", { contentType: "application/json", body: Buffer.from(JSON.stringify({ revision: process.env.KINOSAIL_TEST_REVISION, browser: browserName, command: "playwright test collections-loading.spec.ts", data: "populated generated collection artwork", pending: { artworkComplete: false, box: before }, loaded: { artworkComplete: true, box: after }, result: "passed" })) });
		await page.screenshot({ path: testInfo.outputPath("collections-loaded.png") });
	} finally { release(); }
});

test("Failed artwork retries and heals without a reload", async ({ page, browserName }, testInfo) => {
	await login(page);
	const attempts = new Map<string, number>();
	await page.route("**/art/**", async route => {
		const path = new URL(route.request().url()).pathname;
		const attempt = (attempts.get(path) ?? 0) + 1;
		attempts.set(path, attempt);
		if (attempt === 1) await route.abort("connectionreset");
		else await route.continue();
	});
	await page.goto("/?view=collections", { waitUntil: "domcontentloaded" });
	const image = page.locator(".curation-card img").first();
	await expect(image).toBeVisible();
	await expect.poll(() => image.evaluate((image: HTMLImageElement) => image.complete && image.naturalWidth > 0), { timeout: 15_000 }).toBe(true);
	const path = await image.evaluate((image: HTMLImageElement) => new URL(image.src).pathname);
	expect(attempts.get(path)).toBe(2);
	expect(await image.getAttribute("data-artwork-retry")).toBe("1");
	await page.screenshot({ path: testInfo.outputPath("collections-healed.png") });
	await testInfo.attach("artwork-heal", { contentType: "application/json", body: Buffer.from(JSON.stringify({ revision: process.env.KINOSAIL_TEST_REVISION, browser: browserName, command: "playwright test collections-loading.spec.ts", data: "populated generated collection artwork", failure: "first /art/ request reset", requests: attempts.get(path), result: "passed" })) });
});
