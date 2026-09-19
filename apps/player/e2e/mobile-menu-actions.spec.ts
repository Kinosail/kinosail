import { readFileSync } from "node:fs";
import { expect, test, type Page } from "@playwright/test";

const asset = (name: string) => readFileSync(new URL(`../../../packages/webassets/static/${name}`, import.meta.url), "utf8");
const css = asset("player-app.css") + asset("player-stage.css") + asset("last-light.css") + readFileSync(new URL("../internal/server/static/home.css", import.meta.url), "utf8");
const navigation = asset("pwa-navigation.js");
const tabs = asset("mobile-tabs.js");
const origin = "http://menu.test";

async function loadMenu(page: Page) {
	await page.route(`${origin}/**`, route => route.fulfill({ contentType: "text/html", body: `<!doctype html>
		<meta name="viewport" content="width=device-width,initial-scale=1"><style>${css}</style>
		<body class="library-page"><header class="app-header">
		<form class="search"><input id="library-search" type="search" aria-label="Search all libraries"></form>
		<nav aria-label="Main navigation" data-mobile-tabs data-nav-profile="fixture">
			<a href="/?view=movies">Movies</a>
			<details class="nav-more"><summary>More</summary><div class="nav-more-menu">
				<section class="nav-more-section nav-more-account">
					<a href="/account">Profile</a><form action="/logout" method="post"><button>Sign out</button></form>
				</section>
				<section class="nav-more-section nav-more-actions">
					<a href="/quick-connect">Quick Connect</a><form action="/scan" method="post"><button>Sync</button></form>
					<a href="/supporter">Supporter</a><a href="/settings">Settings</a>
					<a class="nav-edit-menu" href="/settings#navigation">Edit navigation</a>
				</section>
			</div></details>
		</nav></header><main id="main"><h1>Movies</h1></main>
		<script>const animateMotion = () => {};</script><script>${navigation}</script><script>${tabs}</script>` }));
	await page.goto(`${origin}/?view=movies`);
}

test.use({ viewport: { width: 390, height: 844 }, hasTouch: true });

for (const [name, path, role] of [
	["Profile", "/account", "link"], ["Quick Connect", "/quick-connect", "link"],
	["Supporter", "/supporter", "link"], ["Settings", "/settings", "link"],
	["Edit navigation", "/settings#navigation", "link"], ["My List", "/?view=list", "link"],
	["Music", "/?view=music", "link"], ["Sign out", "/logout", "button"], ["Sync", "/scan", "button"],
] as const) {
	test(`Search leaves ${name} tappable in More`, async ({ page }) => {
		await loadMenu(page);
		await page.getByRole("link", { name: "Search", exact: true }).tap();
		await expect(page.getByRole("searchbox")).toBeFocused();
		await page.locator(".nav-more > summary").tap();
		await page.locator(".nav-more-menu").getByRole(role, { name, exact: true }).tap();
		await expect(page).toHaveURL(origin + path);
	});
}

test("fragment navigation does not lock subsequent menu links", async ({ page }) => {
	await loadMenu(page);
	// Remove the Search handler to exercise ordinary same-document fragment navigation.
	await page.locator('a[href="#library-search"]').evaluate(link => link.replaceWith(link.cloneNode(true)));
	await page.getByRole("link", { name: "Search", exact: true }).tap();
	await expect(page).toHaveURL(`${origin}/?view=movies#library-search`);
	await page.locator(".nav-more > summary").tap();
	await page.getByRole("link", { name: "Settings", exact: true }).tap();
	await expect(page).toHaveURL(`${origin}/settings`);
});

test("a handled link does not lock navigation to another document", async ({ page }) => {
	await loadMenu(page);
	await page.getByRole("link", { name: "TV Shows", exact: true }).evaluate(link => {
		link.addEventListener("click", event => event.preventDefault());
	});
	await page.getByRole("link", { name: "TV Shows", exact: true }).tap();
	await page.locator(".nav-more > summary").tap();
	await page.getByRole("link", { name: "Settings", exact: true }).tap();
	await expect(page).toHaveURL(`${origin}/settings`);
});

test("duplicate navigation is blocked until the next pageshow", async ({ page }) => {
	await loadMenu(page);
	const allowed = await page.evaluate(() => {
		const link = document.querySelector<HTMLAnchorElement>('a[href="/settings"]')!;
		// Observe the guard after it bubbles through document, without leaving the fixture.
		const result: boolean[] = [];
		window.addEventListener("click", event => {
			result.push(!event.defaultPrevented);
			event.preventDefault();
		});
		link.click(); link.click();
		window.dispatchEvent(new PageTransitionEvent("pageshow", { persisted: true }));
		link.click();
		return result;
	});
	// The rejected second click stops propagation before the window observer.
	expect(allowed).toEqual([true, true]);
});
