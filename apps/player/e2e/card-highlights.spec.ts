import { readFile, writeFile } from "node:fs/promises";
import { expect, test, type Locator, type Page } from "@playwright/test";

const css = (await Promise.all([
	"../../../packages/webassets/static/player-app.css",
	"../../../packages/webassets/static/last-light.css",
	"../internal/server/static/home.css",
].map(path => readFile(new URL(path, import.meta.url), "utf8")))).join("\n");
const artwork = `data:image/svg+xml,${encodeURIComponent('<svg xmlns="http://www.w3.org/2000/svg" width="400" height="600"><rect width="400" height="600" fill="#345678"/></svg>')}`;
const image = `<img src="${artwork}" alt="" width="400" height="600">`;

// Check the painted perimeter: a computed outline can be hidden beneath artwork.
async function perimeter(page: Page, poster: Locator, token: string) {
	const color = await poster.evaluate((element, name) => {
		const canvas = document.createElement("canvas");
		canvas.width = canvas.height = 1;
		const context = canvas.getContext("2d")!;
		context.fillStyle = getComputedStyle(element).getPropertyValue(name).trim();
		context.fillRect(0, 0, 1, 1);
		return [...context.getImageData(0, 0, 1, 1).data].slice(0, 3);
	}, token);
	const capture = await poster.screenshot();
	return page.evaluate(async ({ png, expected }) => {
		const img = new Image();
		img.src = `data:image/png;base64,${png}`;
		await img.decode();
		const canvas = document.createElement("canvas");
		canvas.width = img.width;
		canvas.height = img.height;
		const context = canvas.getContext("2d")!;
		context.drawImage(img, 0, 0);
		return ["left", "right", "top", "bottom"].map(side => [0, 1, 2, 3, 4].some(inset => {
			const x = side === "left" ? inset : side === "right" ? img.width - 1 - inset : Math.floor(img.width / 2);
			const y = side === "top" ? inset : side === "bottom" ? img.height - 1 - inset : Math.floor(img.height / 2);
			const pixel = [...context.getImageData(x, y, 1, 1).data];
			return expected.every((channel, index) => Math.abs(channel - pixel[index]) <= 2);
		}));
	}, { png: capture.toString("base64"), expected: color });
}

for (const width of [390, 1440]) {
	for (const theme of ["dark", "light"]) {
		test(`cards keep complete hover and keyboard highlights at ${width}px in ${theme}`, async ({ page }, testInfo) => {
			await page.setViewportSize({ width, height: 900 });
			await page.emulateMedia({ reducedMotion: "reduce" });
			const cards = [
				`<a class="card" href="#target">${image.replace('<img ', '<img class="poster" ')}<h3>Movie</h3></a>`,
				`<article class="card show-card"><a class="show-details" href="#target">${image.replace('<img ', '<img class="poster" ')}<h3>Show</h3></a><a class="show-play" href="#play">Play next</a></article>`,
				`<article class="card"><a href="#target"><div class="poster art"></div><h3>Missing artwork</h3></a><form><button type="button">Remove</button></form></article>`,
				...[["loaded", image], ["missing", '<span class="recent-stack-placeholder">TV</span>'], ["failed", image.replace(artwork, "data:image/png;base64,broken")]].map(([state, art]) => `<a class="card recent-card stacked" href="#target"><span class="poster recent-stack"><span class="recent-stack-layer" aria-hidden="true"></span><span class="recent-stack-layer" aria-hidden="true"></span>${art}<span class="recent-stack-count">3</span></span><h3>Stack ${state} artwork</h3><small>3 episodes stacked</small></a>`),
				...[0, 1, 2, 3, 4].map(count => `<a class="curation-card" href="#target"><span class="curation-poster">${image.repeat(count)}</span><strong>Collection ${count}</strong><small>${count} items</small></a>`),
				`<a class="collection-card" href="#target"><span class="collection-poster">${image.repeat(4)}</span><strong>Collection mosaic</strong></a>`,
			];
			await page.setContent(`<!doctype html><html data-theme="${theme}"><head><style>${css}</style></head><body><main style="padding:12px"><div class="curation-grid">${cards.join("")}</div><p id="target">Target</p></main></body></html>`);
			const links = page.locator("a:has(.poster,.curation-poster,.collection-poster)");
			for (const link of await links.all()) {
				const poster = link.locator(".poster,.curation-poster,.collection-poster");
				await page.evaluate(() => (document.activeElement as HTMLElement)?.blur());
				await link.hover();
				expect(await perimeter(page, poster, "--signal"), `${await link.textContent()} hover perimeter`).toEqual([true, true, true, true]);
				await page.mouse.move(0, 0);
				await page.keyboard.press("Tab");
				await link.focus();
				expect(await link.evaluate(element => element.matches(":focus-visible"))).toBeTruthy();
				expect(await perimeter(page, poster, "--focus"), `${await link.textContent()} keyboard perimeter`).toEqual([true, true, true, true]);
				await page.keyboard.press("Tab");
			}
			await page.locator(".stacked").first().click();
			await expect(page).toHaveURL(/#target$/);
			await page.evaluate(() => (document.activeElement as HTMLElement)?.blur());
			await page.getByRole("button", { name: "Remove", exact: true }).hover();
			expect(await perimeter(page, page.locator(".poster.art"), "--signal"), "Remove highlights its own action").toEqual([false, false, false, false]);
			await page.emulateMedia({ forcedColors: "active" });
			await page.mouse.move(0, 0);
			await page.keyboard.press("Tab");
			await page.locator(".curation-card").nth(4).focus();
			await expect(page.locator(".curation-poster").nth(4)).toHaveCSS("outline-style", "solid");
			await page.screenshot({ path: testInfo.outputPath("card-highlights-forced-colors.png"), fullPage: true });
			await page.emulateMedia({ forcedColors: "none" });
			await page.locator(".stacked").first().focus();
			await page.screenshot({ path: testInfo.outputPath("card-highlights.png"), fullPage: true });
			const evidence = testInfo.outputPath("environment.json");
			await writeFile(evidence, JSON.stringify({ width, theme, browser: testInfo.project.name, data: "synthetic 0–4 poster mosaics, nested and direct card links, episode stacks with loaded, missing, and failed artwork", command: "pnpm exec playwright test card-highlights.spec.ts", revision: process.env.KINOSAIL_TEST_REVISION ?? process.env.GITHUB_SHA, result: "passed" }, null, 2));
			await testInfo.attach("environment.json", { path: evidence, contentType: "application/json" });
		});
	}
}

test("collection artwork stays inside its reserved cells while pending, loaded, or failed", async ({ page }, testInfo) => {
	let release!: () => void;
	const pending = new Promise<void>(resolve => { release = resolve; });
	await page.route("https://artwork.test/**", async route => {
		await pending;
		if (route.request().url().endsWith("failed")) await route.abort();
		else await route.fulfill({ contentType: "image/svg+xml", body: '<svg xmlns="http://www.w3.org/2000/svg" width="400" height="600"><rect width="400" height="600" fill="#345678"/></svg>' });
	});
	await page.setContent(`<!doctype html><html><head><style>${css}</style></head><body><main style="padding:12px"><div class="curation-grid">${[0, 1, 2, 3, 4].map(count => `<a class="curation-card" href="#target"><span class="curation-poster">${Array.from({ length: count }, (_, index) => `<img src="https://artwork.test/${count}/${index === 1 ? "failed" : "loaded"}" alt="" width="400" height="600">`).join("")}</span><strong>Collection ${count}</strong></a>`).join("")}</div></main></body></html>`, { waitUntil: "domcontentloaded" });
	const geometry = () => page.locator(".curation-poster").evaluateAll(posters => posters.map(poster => {
		const box = poster.getBoundingClientRect();
		const images = [...poster.querySelectorAll("img")].map(img => { const b = img.getBoundingClientRect(); return { x: b.left - box.left, y: b.top - box.top, width: b.width, height: b.height }; });
		return { width: box.width, height: box.height, images };
	}));
	const before = await geometry();
	await page.screenshot({ path: testInfo.outputPath("collections-pending.png"), fullPage: true });
	release();
	await expect.poll(() => page.locator(".curation-poster img").evaluateAll(images => images.every(image => (image as HTMLImageElement).complete))).toBeTruthy();
	const after = await geometry();
	expect(after).toEqual(before);
	for (const [count, poster] of after.entries()) {
		expect(poster.height / poster.width).toBeCloseTo(1.5, 1);
		for (const [index, img] of poster.images.entries()) {
			expect(img.x, `${count} artwork ${index} horizontal cell`).toBeCloseTo(count === 1 ? 0 : index % 2 * poster.width / 2, 0);
			expect(img.y, `${count} artwork ${index} vertical cell`).toBeCloseTo(count === 1 ? 0 : Math.floor(index / 2) * poster.height / 2, 0);
			expect(img.y + img.height, `${count} artwork stays inside poster`).toBeLessThanOrEqual(poster.height + 1);
		}
	}
	await page.screenshot({ path: testInfo.outputPath("collections-loaded-and-failed.png"), fullPage: true });
});
