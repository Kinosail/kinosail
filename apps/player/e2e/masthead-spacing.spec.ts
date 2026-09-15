import { expect, test, type Page } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { createHmac } from "node:crypto";

test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");

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
}

async function prepareContinuedMovie(page: Page) {
	await login(page);
	await page.goto("/");
	await expect(page.locator('meta[name="kinosail-csrf"]')).toHaveAttribute("content", /.+/);
	await page.evaluate(async () => {
		const csrf = document.querySelector<HTMLMetaElement>('meta[name="kinosail-csrf"]')!.content;
		const response = await fetch("/api/v1/library?view=all");
		if (!response.ok) throw new Error(`library failed: ${response.status}`);
		const catalog = await response.json() as { items: Array<{ id: string; title: string }> };
		const item = catalog.items.find((candidate) => candidate.title === "Example Movie");
		if (!item) throw new Error("Example Movie fixture is missing");
		const saved = await fetch(`/api/v1/items/${item.id}/progress`, { method: "PUT", headers: { "Content-Type": "application/json", "X-Kinosail-CSRF": csrf }, body: JSON.stringify({ seconds: 1, watched: false }) });
		if (!saved.ok) throw new Error(`progress failed: ${saved.status}`);
	});
}

test("home keeps featured content and resume actions reachable", async ({ page }, testInfo) => {
	await prepareContinuedMovie(page);
	for (const viewport of [{ width: 1440, height: 900 }, { width: 1024, height: 768 }, { width: 720, height: 450 }, { width: 390, height: 844 }, { width: 320, height: 800 }]) {
		await page.setViewportSize(viewport);
		await page.goto("/", { waitUntil: "domcontentloaded" });
		await page.screenshot({ path: testInfo.outputPath(`home-${viewport.width}.png`), fullPage: false });
		const composition = await page.evaluate(() => {
			const masthead = document.querySelector(".library-masthead:not(.browse-masthead)")!.getBoundingClientRect();
			const firstShelf = document.querySelector(".home-feature, .home-shelf, .destination-browser")!.getBoundingClientRect();
			return { masthead: masthead.height, firstShelfTop: firstShelf.top };
		});
		expect(composition.masthead, `Home masthead at ${viewport.width}px`).toBeLessThan(viewport.width <= 900 ? 180 : 140);
		expect(composition.firstShelfTop, `Home content at ${viewport.width}px`).toBeLessThan(viewport.height * 0.8);
		const artwork = page.locator(".home-feature > img");
		await expect(page.locator(".home-feature progress")).toBeVisible();
		await expect(artwork).toHaveAttribute("width", "1600");
		await expect(artwork).toHaveAttribute("height", "900");
		const feature = await artwork.evaluate((image) => {
			const frame = image.getBoundingClientRect();
			const heading = document.querySelector(".home-feature h2")!.getBoundingClientRect();
			const tabs = document.querySelector(".home-sections")!.getBoundingClientRect();
			const progress = document.querySelector(".home-feature progress")!.getBoundingClientRect();
			const remaining = document.querySelector(".home-feature [data-watch-remaining]")!.getBoundingClientRect();
			return { ratio: frame.width / frame.height, titleGap: heading.top - frame.bottom, tabGap: frame.top - tabs.bottom,
				progressCenter: progress.top + progress.height / 2, remainingCenter: remaining.top + remaining.height / 2 };
		});
		expect(feature.ratio).toBeCloseTo(16 / 9, 2);
		await expect(artwork).toHaveCSS("filter", "none");
		if (viewport.width <= 900) {
			expect(feature.titleGap).toBeGreaterThanOrEqual(8);
			expect(feature.titleGap).toBeLessThanOrEqual(16);
			expect(feature.tabGap).toBeGreaterThanOrEqual(8);
			expect(feature.tabGap).toBeLessThanOrEqual(16);
			expect(Math.abs(feature.progressCenter - feature.remainingCenter)).toBeLessThan(1);
		}
		const resume = page.locator(".resume-link").first();
		await expect(resume.locator(".resume-action")).toHaveAttribute("aria-label", "Resume");
		await expect(resume.getByRole("progressbar", { name: "Watch progress" })).toBeVisible();
		await expect(resume.getByRole("progressbar", { name: "Watch progress" })).toHaveAttribute("aria-valuetext", /min left/);
		await resume.locator(".resume-action").scrollIntoViewIfNeeded();
		await expect(resume.locator(".resume-action")).toBeInViewport();
		const reachable = await resume.locator(".resume-action").evaluate((action) => {
			const box = action.getBoundingClientRect();
			return [box.top + 2, (box.top + box.bottom) / 2, box.bottom - 2].every((y) => action.contains(document.elementFromPoint((box.left + box.right) / 2, y)));
		});
		expect(reachable, `Resume is unobscured at ${viewport.width}px`).toBe(true);
		await page.keyboard.press("j");
		await expect(resume).toBeFocused();
		await expect(resume).toHaveCSS("outline-style", "solid");
		const removal = page.locator(".resume-card form button").first();
		expect((await removal.boundingBox())!.height).toBeGreaterThanOrEqual(44);
		if (viewport.height >= 700) {
			const recent = page.getByRole("heading", { name: "Recently added", exact: true });
			await recent.scrollIntoViewIfNeeded();
			await expect(recent).toBeInViewport();
		}
		await resume.locator("h3").evaluate((heading) => { heading.textContent = "A very long continued episode title with an extended translated series name and additional episode information"; });
		expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(viewport.width);
		await page.screenshot({ path: testInfo.outputPath(`home-long-title-${viewport.width}.png`), fullPage: false });
	}
});


test("home resume remains accessible across appearance and input preferences", async ({ page }, testInfo) => {
	await prepareContinuedMovie(page);
	for (const theme of ["dark", "light"]) {
		for (const width of [1440, 390, 320]) {
			await page.setViewportSize({ width, height: 900 });
			await page.goto("/");
			await page.evaluate((appearance) => { document.documentElement.dataset.theme = appearance; }, theme);
			await page.evaluate(async () => {
				getComputedStyle(document.body).color;
				await Promise.all(document.getAnimations().filter((animation) => animation instanceof CSSTransition).map((animation) => animation.finished.catch(() => {})));
			});
			expect((await new AxeBuilder({ page }).analyze()).violations, `${theme} home at ${width}px`).toEqual([]);
			await page.screenshot({ path: testInfo.outputPath(`home-${theme}-${width}.png`), fullPage: false });
		}
	}
	for (const language of ["en", "de"]) {
		await page.goto(`/?lang=${language}`);
		await page.evaluate(() => { document.documentElement.style.fontSize = "200%"; });
		expect(await page.evaluate(() => document.documentElement.scrollWidth), `${language} 200% text reflow`).toBeLessThanOrEqual(320);
		expect((await page.locator(".home-feature .button").first().boundingBox())!.width, `${language} enlarged primary action remains usable`).toBeGreaterThanOrEqual(128);
		const card = page.locator(".resume-link").first();
		expect(await card.evaluate((element) => element.scrollWidth <= element.clientWidth + 1), "200% text fits the resume card").toBe(true);
		const navigationFits = await page.locator(".app-header nav").evaluate((navigation) => [...navigation.children].filter((child) => child.getBoundingClientRect().width > 0).every((child) => {
			const box = child.getBoundingClientRect();
			return box.left >= -1 && box.right <= innerWidth + 1 && child.scrollWidth <= child.clientWidth + 1;
		}));
		expect(navigationFits, `${language} enlarged navigation labels fit`).toBe(true);
		await card.locator(".resume-action").scrollIntoViewIfNeeded();
		await page.screenshot({ path: testInfo.outputPath(`home-large-text-${language}-320.png`), fullPage: false });
	}
	const resume = page.locator(".resume-link").first();
	await page.evaluate(() => { document.documentElement.style.fontSize = ""; });
	await page.emulateMedia({ forcedColors: "active", reducedMotion: "reduce" });
	await resume.focus();
	await expect(resume).toBeFocused();
	await expect(resume).toHaveCSS("outline-style", "solid");
	await resume.hover();
	await expect(resume.locator(".poster")).toHaveCSS("transform", "none");
	expect((await new AxeBuilder({ page }).analyze()).violations, "forced colors home").toEqual([]);
	await page.screenshot({ path: testInfo.outputPath("home-forced-colors-320.png"), fullPage: false });
});
