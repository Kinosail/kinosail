import { expect, test, type Page } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { configureTestInstance, firstPlayable, login } from "./test-instance-helpers";

configureTestInstance();
test.use({ serviceWorkers: "block" });

async function theme(page: Page, value: "dark" | "light") {
	await page.addInitScript((choice) => { try { localStorage.setItem("kinosail-theme", choice); } catch {} }, value);
}

async function firstLink(page: Page, from: string, pattern: RegExp) {
	await page.goto(from);
	const hrefs = await page.locator("main a[href]").evaluateAll((links) => links.map((link) => link.getAttribute("href") || ""));
	const href = hrefs.find((candidate) => pattern.test(candidate));
	expect(href, `${pattern} on ${from}`).toBeTruthy();
	return href!;
}

function contrastScript() {
	return (element: Element) => {
		const channels = (value: string) => (value.match(/[\d.]+/g) || []).slice(0, 3).map(Number).map((channel) => {
			const unit = channel / 255;
			return unit <= .03928 ? unit / 12.92 : ((unit + .055) / 1.055) ** 2.4;
		});
		const luminance = (value: string) => { const [red, green, blue] = channels(value); return .2126 * red + .7152 * green + .0722 * blue; };
		const text = luminance(getComputedStyle(element).color);
		const canvas = luminance(getComputedStyle(document.documentElement).backgroundColor);
		return (Math.max(text, canvas) + .05) / (Math.min(text, canvas) + .05);
	};
}

for (const scheme of ["light", "dark"] as const) {
	test(`library page titles stay readable in ${scheme} appearance`, async ({ page }) => {
		await theme(page, scheme);
		await login(page);
		for (const width of [390, 1440]) {
			await page.setViewportSize({ width, height: 900 });
			for (const route of ["/?view=movies", "/?view=playlists", "/?q=Example"]) {
				await page.goto(route);
				const title = page.locator(".library-masthead h2");
				await expect(title).toBeVisible();
				expect(await title.evaluate(contrastScript()), `${route} at ${width}px`).toBeGreaterThanOrEqual(4.5);
			}
		}
	});
}

test("light appearance keeps the header and mobile tabs opaque over scrolled content", async ({ page }) => {
	await theme(page, "light");
	await login(page);
	for (const width of [390, 1440]) {
		await page.setViewportSize({ width, height: 900 });
		await page.goto("/");
		const alpha = (selector: string) => page.locator(selector).first().evaluate((element) => {
			const parts = getComputedStyle(element).backgroundColor.match(/[\d.]+/g) || [];
			return parts.length > 3 ? Number(parts[3]) : 1;
		});
		expect(await alpha(".app-header"), `header at ${width}px`).toBe(1);
		if (width === 390) expect(await alpha(".mobile-navigation"), "mobile tabs").toBe(1);
	}
});

test("request errors keep a page gutter on phones", async ({ page }) => {
	await login(page);
	await page.setViewportSize({ width: 390, height: 844 });
	await page.goto("/does-not-exist");
	const card = await page.locator("main.grant-card").boundingBox();
	expect(card!.x).toBeGreaterThanOrEqual(12);
	expect(card!.x + card!.width).toBeLessThanOrEqual(390 - 12);
});

test("subtitle choice help reads below the choices on desktop", async ({ page }) => {
	await login(page);
	await page.setViewportSize({ width: 1440, height: 900 });
	await page.goto("/settings");
	const set = page.locator("fieldset.segmented-choice").filter({ has: page.locator('input[name="subtitles"]') });
	const help = await set.locator("small").boundingBox();
	const choices = await set.locator(".choice-option").last().boundingBox();
	const box = await set.boundingBox();
	expect(help!.y).toBeGreaterThanOrEqual(choices!.y + choices!.height);
	expect(help!.width).toBeGreaterThan(box!.width * .8);
});

test("detail pages without a backdrop keep an open hero aligned with their sections", async ({ page }) => {
	await login(page);
	await page.setViewportSize({ width: 1440, height: 900 });
	await page.goto(await firstLink(page, "/?view=music", /^\/album\//));
	const hero = page.locator(".media-hero");
	const geometry = await hero.evaluate((element) => {
		const style = getComputedStyle(element);
		const title = element.querySelector("h1")!;
		const section = element.parentElement!.querySelector(".detail-section, section:not(.media-hero)");
		return {
			border: Number.parseFloat(style.borderBottomWidth),
			title: Number.parseFloat(getComputedStyle(title).fontSize),
			gap: element.querySelector(":scope > :first-child")!.getBoundingClientRect().top - element.getBoundingClientRect().top,
			left: element.getBoundingClientRect().left,
			sectionLeft: section ? section.getBoundingClientRect().left + Number.parseFloat(getComputedStyle(section).paddingLeft) : element.getBoundingClientRect().left,
			art: element.querySelector(":scope > :first-child")!.getBoundingClientRect().left,
		};
	});
	expect(geometry.border).toBe(0);
	expect(geometry.title).toBeLessThanOrEqual(80);
	expect(geometry.gap).toBeLessThanOrEqual(32);
	expect(Math.abs(geometry.art - geometry.sectionLeft)).toBeLessThanOrEqual(1);
});

test("show details align the hero, seasons and episode rows", async ({ page }) => {
	await login(page);
	await page.setViewportSize({ width: 1440, height: 900 });
	await page.goto(await firstLink(page, "/?view=shows", /^\/show\//));
	const geometry = await page.evaluate(() => {
		const hero = document.querySelector(".media-hero")!.getBoundingClientRect().left;
		const section = document.querySelector(".detail-section")!;
		const sectionLeft = section.getBoundingClientRect().left + Number.parseFloat(getComputedStyle(section).paddingLeft);
		const stills = [...document.querySelectorAll(".episode-ledger .episode")].map((episode) => episode.querySelector("img, .episode-still")!.getBoundingClientRect().left);
		return { hero, sectionLeft, stills };
	});
	expect(Math.abs(geometry.hero - geometry.sectionLeft)).toBeLessThanOrEqual(1);
	expect(geometry.stills.length).toBeGreaterThan(1);
	expect(Math.max(...geometry.stills) - Math.min(...geometry.stills)).toBeLessThanOrEqual(1);
});

test("placeholder artwork uses one quiet surface in dark appearance", async ({ page }) => {
	await theme(page, "dark");
	await login(page);
	await page.goto("/?view=books");
	const fills = await page.locator("a.card div.poster").evaluateAll((posters) => posters.map((poster) => {
		const style = getComputedStyle(poster);
		return `${style.backgroundColor} ${style.backgroundImage}`;
	}));
	expect(fills.length).toBeGreaterThan(1);
	expect(new Set(fills).size).toBe(1);
	expect(fills[0]).toContain("none");
});

test("watch page backdrop fades evenly on phones", async ({ page }) => {
	await theme(page, "dark");
	await login(page);
	await page.setViewportSize({ width: 390, height: 844 });
	await page.goto(await firstPlayable(page));
	const wash = await page.evaluate(() => getComputedStyle(document.body, "::before").backgroundImage);
	expect(wash).toContain("linear-gradient");
	expect(wash.match(/(rgb|color)\(/g)).toHaveLength(2);
});

test("watch toolbar controls share one height", async ({ page }) => {
	await login(page);
	const watch = await firstPlayable(page);
	for (const width of [390, 1440]) {
		await page.setViewportSize({ width, height: 900 });
		await page.goto(watch);
		const cast = page.getByRole("button", { name: "Play on another device" });
		await expect(cast).toBeVisible();
		const heights = await cast.evaluate((button) => [...button.closest(".player-stage-toolbar")!.querySelectorAll("button")]
			.filter((control) => control.getBoundingClientRect().height > 0)
			.map((control) => Math.round(control.getBoundingClientRect().height)));
		expect(new Set(heights).size, `${width}px ${heights}`).toBe(1);
	}
});

test("reader and offline pages start flush with the top of the window", async ({ page }) => {
	await login(page);
	await page.setViewportSize({ width: 1920, height: 1080 });
	const book = await firstLink(page, "/?view=books", /^\/book\//);
	const reader = await firstLink(page, book, /^\/read\//);
	for (const route of [reader, "/offline"]) {
		await page.goto(route);
		expect(await page.evaluate(() => document.body.getBoundingClientRect().top), route).toBe(0);
	}
	await page.goto(reader);
	expect(await page.locator("main h1").evaluate((title) => Number.parseFloat(getComputedStyle(title).fontSize))).toBeLessThanOrEqual(80);
});

test("library footer aligns with the content column on TV-size windows", async ({ page }) => {
	await login(page);
	await page.setViewportSize({ width: 1920, height: 1080 });
	await page.goto("/?view=shows");
	const edges = await page.evaluate(() => {
		const box = (selector: string) => document.querySelector(selector)!.getBoundingClientRect();
		return { content: [box(".library-masthead").left, box(".library-masthead").right], footer: [box("body > .language-picker").left, box("body > .language-picker").right] };
	});
	expect(Math.abs(edges.footer[0] - edges.content[0])).toBeLessThanOrEqual(1);
	expect(Math.abs(edges.footer[1] - edges.content[1])).toBeLessThanOrEqual(1);
});

test("remote access help disclosures meet the target size", async ({ page }) => {
	await login(page);
	await page.setViewportSize({ width: 1440, height: 900 });
	await page.goto("/settings/remote-readiness");
	const result = await new AxeBuilder({ page }).withRules(["target-size"]).analyze();
	expect(result.violations.map((violation) => violation.nodes.map((node) => node.target).join(" "))).toEqual([]);
});

test("enlarged phone text keeps browse sorting, quick connect and backups usable", async ({ page }) => {
	await login(page);
	await page.setViewportSize({ width: 390, height: 844 });
	const enlarge = () => page.evaluate(() => document.documentElement.style.setProperty("font-size", "200%", "important"));
	await page.goto("/?view=movies");
	await enlarge();
	const toolbar = await page.locator(".browse-toolbar :is(span,label,select,button)").evaluateAll((elements) => elements
		.filter((element) => element.getBoundingClientRect().width > 0)
		.map((element) => [element.getBoundingClientRect().left, element.getBoundingClientRect().right]));
	for (const [left, right] of toolbar) {
		expect(left).toBeGreaterThanOrEqual(0);
		expect(right).toBeLessThanOrEqual(390);
	}
	await page.goto("/quick-connect");
	await enlarge();
	for (const digit of await page.locator(".quick-connect-digit").all()) expect((await digit.boundingBox())!.width).toBeGreaterThanOrEqual(24);
	await page.goto("/settings/backups");
	await enlarge();
	expect(await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBeLessThanOrEqual(0);
});
