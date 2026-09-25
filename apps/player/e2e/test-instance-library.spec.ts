import { expect, test } from "@playwright/test";
import { configureTestInstance, login, openLibrarySection } from "./test-instance-helpers";

configureTestInstance();

test("a stale login page redirects passkey sign-in to the canonical origin", async ({ page }) => {
  await page.addInitScript(() => Object.defineProperty(navigator, "credentials", { value: { get: async () => { throw new DOMException("no test passkey", "NotAllowedError"); } } }));
  await page.goto("/login");
  await expect(page.locator('meta[name="kinosail-csrf"]')).toHaveCount(0);
  const login = new URL(page.url());
  await page.context().addCookies([{ name: login.protocol === "https:" ? "__Host-kinosail_session" : "kinosail_session", value: "stale-test-session", url: new URL("/", login).toString(), httpOnly: true, secure: login.protocol === "https:", sameSite: "Strict" }]);
  const response = page.waitForResponse((candidate) => candidate.url().endsWith("/api/v1/passkeys/login/begin") && candidate.request().method() === "POST");
  await page.getByRole("button", { name: "Sign in with passkey" }).click();
  const begin = await response;
  expect(begin.status()).toBe(421);
  expect(begin.headers().location).toBe("https://localhost:38127/login");
});

test("public test instance exercises every media section and local TMDB metadata", async ({ page }) => {
  await login(page);
	for (const group of ["Watch & view", "Listen & read", "Organize"]) await expect(page.getByRole("heading", { name: group })).toBeVisible();

  const sections = [
    ["Movies", "Example Movie"],
    ["Shows", "Example Show"],
    ["Music", "Example Album"],
    ["Audiobooks", "Example Audiobook"],
    ["Books", "Example PDF Book"],
    ["Photos", "Example Photo One"],
  ] as const;
  for (const [section, title] of sections) {
    await openLibrarySection(page, section);
    await expect(page.getByRole("heading", { name: title }).first()).toBeVisible();
  }

  await page.getByRole("link", { name: "Movies", exact: true }).click();
  await page.getByRole("link", { name: /Example Movie/ }).click();
  await expect(page.locator("video")).toBeVisible();
	await expect(page.locator('video track[label="EN"]')).toHaveAttribute("default", "");
  await expect(page.getByRole("paragraph").filter({ hasText: "Example metadata from the local generated TMDB fixture." })).toBeVisible();
	await page.goto("/settings#playback");
	const subtitles = page.locator('form[action="/settings/subtitles"]').locator("..");
	await expect(subtitles).toContainText("Stored here. Streamed directly.");
	await expect(subtitles).toContainText("Find local subtitle files with");
	await expect(subtitles.getByRole("link", { name: "Kino Subtitles on GitHub" })).toHaveAttribute("href", "https://github.com/Kinosail/kinosail/tree/main/apps/subtitles");
	await expect(subtitles).not.toContainText("SubDL");
	await expect(subtitles.getByLabel("Preferred language")).toHaveValue("en");

  await page.getByRole("link", { name: "Browse library", exact: true }).click();
  await openLibrarySection(page, "Music");
  await page.getByRole("link", { name: /Example Album/ }).click();
  await page.getByRole("link", { name: "Example Track One" }).click();
  await expect(page.locator("audio")).toBeVisible();

  await page.getByRole("link", { name: "Library" }).click();
  await openLibrarySection(page, "Books");
  await page.getByRole("link", { name: "Example PDF Book" }).click();
  await page.getByRole("link", { name: /Read now/ }).click();
  const reader = page.locator("iframe.book-reader");
  await expect(reader).toBeVisible();
  const document = await page.request.get((await reader.getAttribute("src"))!);
  expect(document.headers()["x-frame-options"]).toBe("SAMEORIGIN");
  expect(document.headers()["content-security-policy"]).toContain("frame-ancestors 'self'");

  await page.goto("/?view=books");
  await page.getByRole("link", { name: "Example EPUB Book" }).click();
  await page.getByRole("link", { name: /Read now/ }).click();
  await expect(page.locator("iframe.book-reader")).toBeVisible();
  await page.goto("/?view=books");
  await page.getByRole("link", { name: "Example Comic" }).click();
  await page.getByRole("link", { name: /Read now/ }).click();
  await expect(page.locator(".reader-pages img")).toHaveCount(2);
  const library = await page.request.get("/api/v1/library");
  expect(library.ok()).toBeTruthy();
  expect(await library.text()).toContain("Example metadata from the local generated TMDB fixture.");
});

test("media artwork keeps its intended ratio in the populated library", async ({ page }) => {
  await page.setViewportSize({ width: 992, height: 964 });
  await login(page);
	await page.goto("/settings");
	await page.getByRole("link", { name: "System", exact: true }).click();
	await page.locator('form[action="/settings/tasks/metadata"] button').click();
	await page.goto("/?view=shows");
	await page.getByRole("link", { name: "Example Show", exact: true }).click();
	const episode = await page.locator('a.episode[href^="/watch/"]').first().getAttribute("href");
	expect(episode).toBeTruthy();
	const show = new URL(page.url()).pathname;
	await page.goto("/");
	const recent = page.locator(".home-shelf").filter({ hasText: "Recently added" });
	await expect(recent.locator(`a[href="${show}"]`)).toContainText("2 episodes stacked");
	await expect(recent.locator(`a[href="${show}"] .recent-stack-count`)).toHaveText("2");
  await page.goto("/?view=movies");
  const poster = page.locator('a.card[href^="/watch/"] .poster').first();
  const box = await poster.boundingBox();
  expect(box).not.toBeNull();
  expect(box!.height / box!.width).toBeCloseTo(1.5, 2);
  await expect(page.getByRole("heading", { name: /Example Movie/ })).toBeInViewport();
	await page.goto("/?view=photos");
	const photo = await page.locator(".poster.photo").first().boundingBox();
	expect(photo).not.toBeNull();
	expect(photo!.height / photo!.width).toBeCloseTo(.75, 2);
});

test("mobile media detail heroes stack artwork for movies, shows, albums, and books", async ({ page }) => {
	await page.setViewportSize({ width: 390, height: 844 });
	await login(page);

	const routes: string[] = [];
	for (const [view, selector] of [
		["movies", 'a.card[href^="/item/"]'],
		["shows", 'a.show-details[href^="/show/"]'],
		["music", 'a.card[href^="/album/"]'],
		["books", 'a.card[href^="/book/"]'],
	] as const) {
		await page.goto(`/?view=${view}`, { waitUntil: "domcontentloaded" });
		const href = await page.locator(selector).first().getAttribute("href");
		expect(href, `${view} detail link`).toBeTruthy();
		routes.push(href!);
	}

	for (const route of routes) {
		await page.goto(route, { waitUntil: "domcontentloaded" });
		const hero = page.locator(".media-hero");
		await expect(hero, route).toBeVisible();
		const layout = await hero.evaluate((element) => {
			const visibleChildren = [...element.children].filter((child) => getComputedStyle(child).display !== "none");
			const artwork = visibleChildren.find((child) => child.matches(".hero-poster, .media-backdrop")) as HTMLElement;
			const copy = visibleChildren.at(-1) as HTMLElement;
			const artworkBox = artwork.getBoundingClientRect();
			const copyBox = copy.getBoundingClientRect();
			const heroBox = element.getBoundingClientRect();
			return {
				artworkBottom: artworkBox.bottom,
				artworkWidth: artworkBox.width,
				copyTop: copyBox.top,
				heroWidth: heroBox.width,
				isBackdrop: artwork.matches(".media-backdrop"),
				overflow: document.documentElement.scrollWidth - innerWidth,
			};
		});
		expect(layout.copyTop, `${route} copy follows artwork`).toBeGreaterThanOrEqual(layout.artworkBottom - 1);
		if (!layout.isBackdrop) expect(layout.artworkWidth, `${route} poster fits the phone`).toBeLessThanOrEqual(Math.min(layout.heroWidth, 288) + 1);
		expect(layout.overflow, `${route} has no page overflow`).toBe(0);
	}
});

test("show episodes expose their 16:9 still artwork", async ({ page }, testInfo) => {
	await login(page);
	await page.goto("/?view=shows");
	await page.getByRole("link", { name: "Example Show", exact: true }).click();
	const show = page.locator("[data-season-reel]").first();
	const episodes = show.locator("[data-episode-row]");
	await expect(episodes).toHaveCount(2);
	await expect(show.locator('[data-episode-row][data-episode-art*="variant=episode"]')).toHaveCount(2);
	await expect(show.locator('[data-preview-image][src*="variant=episode"]')).toBeVisible();
	for (const viewport of [{ width: 1440, height: 900 }, { width: 390, height: 844 }, { width: 320, height: 800 }]) {
		await page.setViewportSize(viewport);
		await page.reload();
		const preview = show.locator("[data-preview-art]");
		if (viewport.width > 700) {
			const still = await preview.boundingBox();
			expect(still).not.toBeNull();
			expect(still!.height / still!.width).toBeCloseTo(9 / 16, 2);
		} else {
			expect(await show.locator("[data-preview-link]").isVisible()).toBeFalsy();
		}
		await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-episode-stills.png`), fullPage: true });
	}
});

test("beta UI surfaces stay reachable and expose only working controls", async ({ page }, testInfo) => {
  const errors: string[] = [];
  let offline = false;
  await page.setViewportSize({ width: 390, height: 844 });
  await login(page);
  page.on("console", (message) => {
    if (!offline && message.type() === "error") errors.push(`${page.url()}: ${message.text()}`);
  });
  page.on("pageerror", (error) => { if (!offline) errors.push(`${page.url()}: ${error.message}`); });

  const owner = page.getByRole("link", { name: "Owner", exact: true });
  if (!await owner.isVisible()) await page.locator(".nav-more > summary").click();
  await expect(owner).toBeVisible();
  await expect(owner).toHaveAttribute("href", "/account");
  await page.getByRole("link", { name: "Movies", exact: true }).click();
  const watch = await page.locator('a.card[href^="/watch/"]').filter({ hasText: "Example Movie" }).first().getAttribute("href");
  await page.getByRole("link", { name: "Shows", exact: true }).click();
  const show = await page.locator('a.show-details[href^="/show/"]').first().getAttribute("href");
  await openLibrarySection(page, "Music");
  const album = await page.locator('a.card[href^="/album/"]').first().getAttribute("href");
  await openLibrarySection(page, "Books");
  const book = await page.locator('a.card[href^="/book/"]').first().getAttribute("href");
  expect(watch && show && album && book).toBeTruthy();

  await openLibrarySection(page, "Playlists");
  const smartName = `Beta ${testInfo.project.name} ${Date.now()}`;
  await page.getByText("New smart playlist", { exact: true }).click();
  await page.getByLabel("Smart playlist name").fill(smartName);
  await page.getByLabel("Match text").fill("Example");
  await page.getByLabel("Media").selectOption("video");
  await page.getByRole("button", { name: "Create smart playlist", exact: true }).click();
  await expect(page.getByRole("heading", { name: smartName, exact: true })).toBeVisible();
  await page.goto("/");
  const collectionName = `Beta Collection ${testInfo.project.name} ${Date.now()}`;
  await openLibrarySection(page, "Collections");
  await page.getByLabel("New Collection name").fill(collectionName);
  await page.getByLabel("New Collection name").press("Enter");
  await expect(page.getByRole("heading", { name: collectionName, exact: true })).toBeVisible();
  const routes = [
    "/", "/?view=list", "/?view=movies", "/?view=shows", "/?view=collections", "/?view=playlists", "/?view=music", "/?view=audiobooks", "/?view=books", "/?view=photos",
    "/?view=history", "/?view=unwatched", "/quick-connect", "/offline-downloads", "/settings", "/settings/configuration",
    "/settings/backups", "/settings/agent-connections", "/account", watch!, show!, album!, book!, `/collection/${encodeURIComponent(collectionName)}`, `/playlist/${encodeURIComponent(smartName)}`,
  ];
  for (const route of routes) {
    const response = await page.goto(route);
    expect(response?.ok(), route).toBeTruthy();
    await expect(page.locator("main")).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), route).toBeTruthy();
  }

  await page.goto(watch!);
  await expect(page.getByRole("button", { name: new RegExp(`(?:Add to|Remove from) ${smartName}`) })).toHaveCount(0);
  await page.getByText("Playback & downloads", { exact: true }).click();
  await page.getByRole("button", { name: "Prepare 720p offline", exact: true }).click();
  await expect(page.getByText("720p · Ready to download", { exact: true })).toBeVisible({ timeout: 30_000 });
  await expect(page.getByRole("link", { name: "Save file", exact: true })).toBeVisible();
  await page.getByRole("button", { name: "Download to this device", exact: true }).click();
  await expect(page.getByText("Ready offline on this device", { exact: true })).toBeVisible({ timeout: 30_000 });
  await page.goto("/offline");
  await page.getByRole("link", { name: /Example Movie · 720p/ }).click();
  await expect(page.locator("video")).toHaveAttribute("src", /\/offline-media\//);
  offline = true;
  await page.context().setOffline(true);
  try {
    await page.reload({ waitUntil: "domcontentloaded" });
    await expect(page.locator("video")).toHaveAttribute("src", /\/offline-media\//, { timeout: 15_000 });
  } finally {
    await page.context().setOffline(false);
  }
  expect(errors.filter((error) => !error.includes("blob:http://") && !error.includes("Applying inline style violates") && !(testInfo.project.name === "firefox" && error.includes("NS_BINDING_ABORTED")))).toEqual([]);
});
