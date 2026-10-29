import { expect, test, type Frame, type Page } from "@playwright/test";
import { configureTestInstance, firstPlayable, login } from "./test-instance-helpers";

configureTestInstance();
test.use({ serviceWorkers: "block" });

async function theme(page: Page, value: "dark" | "light") {
	await page.addInitScript((choice) => { try { localStorage.setItem("kinosail-theme", choice); } catch {} }, value);
}

async function linkTo(page: Page, from: string, pattern: RegExp, name?: RegExp) {
	await page.goto(from);
	const links = name ? page.locator("main a[href]").filter({ hasText: name }) : page.locator("main a[href]");
	const hrefs = await links.evaluateAll((elements) => elements.map((element) => element.getAttribute("href") || ""));
	const href = hrefs.find((candidate) => pattern.test(candidate));
	expect(href, `${pattern} on ${from}`).toBeTruthy();
	return href!;
}

async function chapter(page: Page): Promise<Frame> {
	const frame = await (await page.locator("iframe.book-reader").elementHandle())!.contentFrame();
	await frame!.waitForLoadState("load");
	await frame!.locator("img").evaluate((image: HTMLImageElement) => image.complete || new Promise((done) => { image.onload = image.onerror = done; }));
	await frame!.evaluate(async () => { await document.fonts.ready; });
	await frame!.waitForFunction(() => document.documentElement.hasAttribute("data-theme"));
	return frame!;
}

function luminance(color: string) {
	const [red, green, blue] = (color.match(/[\d.]+/g) || []).slice(0, 3).map(Number).map((channel) => {
		const unit = channel / 255;
		return unit <= .03928 ? unit / 12.92 : ((unit + .055) / 1.055) ** 2.4;
	});
	return .2126 * red + .7152 * green + .0722 * blue;
}

for (const scheme of ["light", "dark"] as const) {
	test(`EPUB chapters use reading typography in ${scheme} appearance`, async ({ page }) => {
		await theme(page, scheme);
		await login(page);
		const reader = await linkTo(page, await linkTo(page, "/?view=books", /^\/book\//, /Example EPUB Book/), /^\/read\//);
		for (const width of [390, 1440]) {
			await page.setViewportSize({ width, height: 900 });
			await page.goto(reader);
			const frame = await chapter(page);
			const chapterText = await frame.evaluate(() => {
				const style = (selector: string) => getComputedStyle(document.querySelector(selector)!);
				const paragraph = document.querySelector("p")!.getBoundingClientRect();
				const image = document.querySelector("img")!;
				const canvas = [document.body, document.documentElement].map((element) => getComputedStyle(element).backgroundColor).find((color) => !/rgba\(0, 0, 0, 0\)|transparent/.test(color)) || "";
				return {
					font: style("p").fontFamily, size: Number.parseFloat(style("p").fontSize), lineHeight: Number.parseFloat(style("p").lineHeight),
					measure: paragraph.width, left: paragraph.left, right: innerWidth - paragraph.right,
					title: Number.parseFloat(style("h1").fontSize), titleWeight: Number(style("h1").fontWeight), titleFont: style("h1").fontFamily, heading: Number.parseFloat(style("h2").fontSize),
					imageLoaded: image.naturalWidth > 0, imageRight: image.getBoundingClientRect().right,
					text: style("p").color, link: style("a").color, underline: style("a").textDecorationLine, canvas,
					publisher: style(".publisher-note").fontStyle, overflow: document.documentElement.scrollWidth - innerWidth,
					fontLoaded: document.fonts.check(`${style("p").fontSize} Manrope`) && [...document.fonts].some((face) => face.status === "loaded"),
					script: document.documentElement.getAttribute("data-book-script"),
				};
			});
			const label = `${scheme} ${width}px`;
			expect(chapterText.font, label).toContain("Manrope");
			expect(chapterText.titleFont, label).toContain("Manrope");
			expect(chapterText.fontLoaded, label).toBe(true);
			expect(chapterText.script, label).toBeNull();
			expect(chapterText.size, label).toBeGreaterThanOrEqual(16);
			expect(chapterText.lineHeight / chapterText.size, label).toBeGreaterThanOrEqual(1.5);
			expect(chapterText.measure / chapterText.size, label).toBeLessThanOrEqual(38);
			expect(Math.min(chapterText.left, chapterText.right), label).toBeGreaterThanOrEqual(16);
			expect(chapterText.title, label).toBeGreaterThan(chapterText.heading);
			expect(chapterText.heading, label).toBeGreaterThan(chapterText.size);
			expect(chapterText.titleWeight, label).toBeGreaterThanOrEqual(700);
			expect(chapterText.imageLoaded, label).toBe(true);
			expect(chapterText.imageRight, label).toBeLessThanOrEqual(width);
			expect(chapterText.link, label).not.toBe(chapterText.text);
			expect(chapterText.underline, label).toContain("underline");
			expect(chapterText.publisher, label).toBe("italic");
			expect(chapterText.overflow, label).toBeLessThanOrEqual(0);
			const [text, canvas] = [luminance(chapterText.text), luminance(chapterText.canvas)];
			expect((Math.max(text, canvas) + .05) / (Math.min(text, canvas) + .05), label).toBeGreaterThanOrEqual(7);
			if (scheme === "dark") expect(canvas, label).toBeLessThan(.05);
			else expect(canvas, label).toBeGreaterThan(.8);
		}
	});
}

test("System and remote access pages keep a readable measure and one divider style", async ({ page }) => {
	await login(page);
	for (const width of [390, 1440]) {
		await page.setViewportSize({ width, height: 900 });
		for (const route of ["/settings/system", "/settings/remote-readiness"]) {
			await page.goto(route);
			const layout = await page.evaluate(() => {
				const main = document.querySelector("main.settings-shell")!;
				const dividers: { y: number; left: number; right: number; name: string }[] = [];
				for (const element of main.querySelectorAll(":scope > *, :scope > .settings-flow > *")) {
					const box = element.getBoundingClientRect();
					const style = getComputedStyle(element);
					const width = (side: string) => style.getPropertyValue(`border-${side}-style`) === "none" ? 0 : Number.parseFloat(style.getPropertyValue(`border-${side}-width`));
					if (!box.width || width("left") || width("right")) continue;
					const name = `${element.tagName.toLowerCase()}.${element.className}#${element.id}`;
					if (width("top")) dividers.push({ y: box.top, left: box.left, right: box.right, name });
					if (width("bottom")) dividers.push({ y: box.bottom, left: box.left, right: box.right, name });
				}
				const measures = [...main.querySelectorAll("section p, section li > p")].filter((paragraph) => paragraph.getBoundingClientRect().width > 0)
					.map((paragraph) => paragraph.getBoundingClientRect().width / Number.parseFloat(getComputedStyle(paragraph).fontSize));
				return { dividers: dividers.sort((first, second) => first.y - second.y), measure: Math.max(...measures) };
			});
			const label = `${route} at ${width}px`;
			expect(layout.dividers.length, label).toBeGreaterThan(2);
			expect(layout.measure, label).toBeLessThanOrEqual(40);
			for (const divider of layout.dividers) {
				expect(Math.abs(divider.left - layout.dividers[0].left), `${label} ${divider.name}`).toBeLessThanOrEqual(1);
				expect(Math.abs(divider.right - layout.dividers[0].right), `${label} ${divider.name}`).toBeLessThanOrEqual(1);
			}
			for (let index = 1; index < layout.dividers.length; index++) {
				expect(layout.dividers[index].y - layout.dividers[index - 1].y, `${label} ${layout.dividers[index].name}`).toBeGreaterThan(24);
			}
		}
	}
});

test("Settings subpages lead back with the same arrow", async ({ page }) => {
	await login(page);
	for (const route of ["/settings/system", "/settings/remote-readiness", "/settings/management", "/settings/backups", "/settings/media-shares"]) {
		await page.goto(route);
		await expect(page.locator("main a.back i.i-back"), route).toHaveCount(1);
	}
});

test("playlist and Collection menus use one name for Collections", async ({ page }) => {
	await login(page);
	const book = await linkTo(page, "/?view=books", /^\/book\//, /Example EPUB Book/);
	for (const route of [book, await firstPlayable(page)]) {
		await page.goto(route);
		const menu = page.locator("details.curation-menu");
		await expect(menu.locator("summary")).toContainText("Add to playlist or Collection");
		const copy = await menu.evaluate((element) => [element.textContent || "", ...[...element.querySelectorAll("[aria-label]")].map((labelled) => labelled.getAttribute("aria-label") || "")].join(" "));
		expect(copy, route).not.toMatch(/collection/);
	}
});

test("enlarged text scales body copy and keeps library layouts inside the window", async ({ page }) => {
	await login(page);
	const routes = ["/", "/?view=collections", await linkTo(page, "/?view=shows", /^\/show\//), await firstPlayable(page), "/settings"];
	for (const width of [390, 1440]) {
		await page.setViewportSize({ width, height: 900 });
		for (const route of routes) {
			await page.goto(route);
			const base = await page.evaluate(() => Number.parseFloat(getComputedStyle(document.body).fontSize));
			await page.evaluate(() => document.documentElement.style.setProperty("font-size", "200%", "important"));
			const label = `${route} at ${width}px`;
			expect(await page.evaluate(() => Number.parseFloat(getComputedStyle(document.body).fontSize)) / base, label).toBeGreaterThanOrEqual(1.9);
			const issues = await page.evaluate(() => {
				const found: string[] = [];
				const overflow = document.documentElement.scrollWidth - document.documentElement.clientWidth;
				if (overflow > 0) found.push(`page overflows by ${overflow}px`);
				for (const summary of document.querySelectorAll(".player-disclosure>summary")) {
					const [title, hint] = [summary.querySelector("span"), summary.querySelector("small")].map((element) => element ? Number.parseFloat(getComputedStyle(element).fontSize) : 0);
					if (hint > title) found.push(`${summary.textContent} hint is larger than its title`);
				}
				for (const element of document.querySelectorAll("main :is(h1,h2,h3,strong,button,label,summary span)")) {
					if (!element.getClientRects().length) continue;
					for (const node of element.childNodes) {
						if (node.nodeType !== Node.TEXT_NODE) continue;
						for (const word of (node as Text).data.matchAll(/[\p{L}\d]{3,}/gu)) {
							const range = document.createRange();
							const tops = new Set<number>();
							for (let index = word.index!; index < word.index! + word[0].length; index++) {
								range.setStart(node, index);
								range.setEnd(node, index + 1);
								const rect = range.getBoundingClientRect();
								if (rect.width) tops.add(Math.round(rect.top));
							}
							if (tops.size > 1) found.push(`${element.tagName.toLowerCase()}.${element.className} splits ${word[0]}`);
						}
					}
				}
				return found;
			});
			expect(issues, label).toEqual([]);
		}
	}
});

test("enlarged phone text keeps the Server name and poster titles whole", async ({ page }) => {
	await login(page);
	await page.setViewportSize({ width: 390, height: 844 });
	for (const route of ["/", "/?view=movies"]) {
		await page.goto(route);
		await page.evaluate(() => document.documentElement.style.setProperty("font-size", "200%", "important"));
		const brand = await page.locator(".app-header .brand-lockup h1").evaluate((title) => {
			const box = title.getBoundingClientRect();
			return { clipped: title.scrollWidth > title.clientWidth + 1 || getComputedStyle(title).textOverflow === "ellipsis" && title.scrollWidth > title.clientWidth, left: box.left, right: box.right };
		});
		expect(brand.clipped, route).toBe(false);
		expect(brand.left, route).toBeGreaterThanOrEqual(0);
		expect(brand.right, route).toBeLessThanOrEqual(390);
		const splits = await page.locator(".app-header .brand-lockup h1, .home-feature h2, main a.card :is(h2,h3)").evaluateAll((titles) => titles.flatMap((title) => {
			const text = title.firstChild?.nodeType === Node.TEXT_NODE ? title.firstChild as Text : null;
			if (!text) return [];
			const style = getComputedStyle(title);
			const words = [...text.data.matchAll(/\S+/g)].flatMap((word) => {
				const range = document.createRange();
				const tops = new Set<number>();
				for (let index = word.index!; index < word.index! + word[0].length; index++) {
					range.setStart(text, index);
					range.setEnd(text, index + 1);
					const rect = range.getBoundingClientRect();
					if (rect.width) tops.add(Math.round(rect.top));
				}
				return tops.size > 1 ? [`${title.tagName}: ${word[0]}`] : [];
			});
			if (title.matches("a.card :is(h2,h3)") && title.clientWidth / Number.parseFloat(style.fontSize) < 6) words.push(`${title.tagName}: ${text.data} is ${title.clientWidth}px wide`);
			return words;
		}));
		expect(splits, route).toEqual([]);
		expect(await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth), route).toBeLessThanOrEqual(0);
	}
});
