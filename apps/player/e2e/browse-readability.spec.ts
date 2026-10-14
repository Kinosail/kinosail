import { expect, test } from "@playwright/test";
import { configureTestInstance, login } from "./test-instance-helpers";
import { expectNavigationIconPaint } from "./navigation-icon-paint";
import { verifyTitleRail } from "./title-rail-geometry";

configureTestInstance();
test.use({ serviceWorkers: "block" });

test("mobile title rail clears Search and navigation across browse states", { tag: "@smoke" }, async ({ page }, info) => {
	await login(page);
	await verifyTitleRail(page, info);
});

test("browse counts and navigation icons remain readable", { tag: "@smoke" }, async ({ page }, info) => {
	await login(page);
	for (const viewport of [{ width: 390, height: 844 }, { width: 1440, height: 900 }, { width: 1920, height: 1080 }]) {
		await page.setViewportSize(viewport);
		for (const [view, label] of [["shows", "1 item"], ["movies", "4 items"]]) {
			await page.goto(`/?view=${view}`, { waitUntil: "domcontentloaded" });
			await expect(page.locator("#library .poster").first()).toBeVisible();
			expect.soft(await page.locator(".browse-toolbar > span").innerText(), `${view} count at ${viewport.width}px`).toBe(label);
			await page.screenshot({ path: info.outputPath(`${view}-count-${viewport.width}.png`) });
		}
	}
	const name = `QA readability ${info.project.name} ${Date.now()}`;
	const detailPath = `/playlist/${encodeURIComponent(name)}`;
	const listing = await page.context().newPage();
	const indexCount = async (label: string) => {
		await listing.goto("/?view=playlists");
		const card = listing.locator(".curation-card").filter({ has: listing.getByText(name, { exact: true }) });
		await expect(card).toBeVisible();
		expect.soft((await card.locator("small").first().innerText()).split(" · ").pop(), "playlist index count").toBe(label);
		await card.screenshot({ path: info.outputPath(`playlist-index-${label.replace(" ", "-")}.png`) });
	};
	let creationAttempted = false, primaryFailed = false;
	try {
		await page.goto("/?view=playlists");
		await page.getByLabel("New playlist name").fill(name);
		creationAttempted = true;
		await page.getByRole("button", { name: "Create", exact: true }).click();
		await indexCount("0 items");
		await page.getByRole("textbox", { name: "Find something", exact: true }).fill("Example");
		await page.getByRole("button", { name: "Find", exact: true }).click();
		for (const label of ["1 item", "2 items"]) {
			await page.locator(".collection-results").getByRole("button", { name: "Add to", exact: true }).first().click();
			await expect(page.locator("main.collection-detail > p").filter({ hasText: /^\d+ items?$/ })).toHaveText(label);
			await indexCount(label);
		}
	} catch (error) {
		primaryFailed = true;
		throw error;
	} finally {
		const cleanup = await Promise.allSettled([
			listing.close(),
			(async () => {
				if (!creationAttempted) return;
				await page.goto(detailPath);
				await page.getByText(`Delete ${name}?`, { exact: true }).click();
				await page.getByRole("button", { name: "Delete playlist", exact: true }).click();
			})(),
		]);
		const outcomes = cleanup.map((result, index) => ({ operation: ["close listing page", "delete owned playlist"][index], status: result.status }));
		try {
			await info.attach("playlist-cleanup.json", { body: JSON.stringify(outcomes), contentType: "application/json" });
		} catch (error) {
			if (!primaryFailed) throw error;
			info.annotations.push({ type: "cleanup-evidence", description: "Cleanup attachment failed; original test failure preserved." });
		}
		if (cleanup.some(result => result.status === "rejected") && !primaryFailed) throw new Error("Owned playlist cleanup failed; see playlist-cleanup.json");
	}
	await page.setViewportSize({ width: 720, height: 450 });
	await page.goto("/?view=all", { waitUntil: "domcontentloaded" });
	for (const theme of ["dark", "light"] as const) {
		await page.evaluate(value => { document.documentElement.dataset.theme = value; }, theme);
		for (const forcedColors of ["none", "active"] as const) {
			await page.emulateMedia({ colorScheme: theme, forcedColors, reducedMotion: "reduce" });
			await expect(page.locator(".header-compact-menu > summary")).toHaveAccessibleName("Actions");
			await expectNavigationIconPaint(page, info, `${theme}-${forcedColors}`);
		}
	}
});
