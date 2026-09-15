import { expect, test } from "@playwright/test";
import { auditBetaRoutes, betaRouteGroups } from "./layout-audit-beta-helpers";
import { configureLayoutAudit, login, viewports } from "./layout-audit-helpers";

configureLayoutAudit();

for (const viewport of viewports) {
	for (const group of betaRouteGroups) {
		test(`beta ${group.name} pages fit the ${viewport.width}px viewport`, async ({ page }, testInfo) => {
			test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
			test.setTimeout(180_000);
			await login(page);
			await auditBetaRoutes(page, testInfo, viewport, [...group.routes]);
		});
	}

	test(`beta media pages fit the ${viewport.width}px viewport`, async ({ page }, testInfo) => {
		test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
		test.setTimeout(180_000);
		await login(page);
		const href = async (route: string, selector: string) => { await page.goto(route); const value = await page.locator(selector).first().getAttribute("href"); expect(value, selector).toBeTruthy(); return value!; };
		const show = await href("/?view=shows", 'a.show-details[href^="/show/"]');
		const album = await href("/?view=music", 'a.card[href^="/album/"]');
		const book = await href("/?view=books", 'a.card[href^="/book/"]');
		const watch = await href("/?view=movies", 'a.card[href^="/watch/"]');
		const reader = await href(book, 'a[href^="/read/"]');
		await auditBetaRoutes(page, testInfo, viewport, [watch, show, album, book, reader]);
	});

	test(`beta created pages fit the ${viewport.width}px viewport`, async ({ page }, testInfo) => {
		test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
		test.setTimeout(120_000);
		await login(page);
		await page.goto("/?view=playlists");
		const playlistForm = page.locator('form[action="/playlists"]');
		await playlistForm.getByLabel("New playlist name").fill(`Layout-${testInfo.project.name}-${viewport.width}-${Date.now()}`);
		await playlistForm.getByRole("button", { name: "Create" }).click();
		const playlist = new URL(page.url()).pathname;
		await page.goto("/?view=collections");
		await page.getByLabel("New Collection name").fill(`Layout collection-${testInfo.project.name}-${viewport.width}-${Date.now()}`);
		await page.getByLabel("New Collection name").press("Enter");
		await auditBetaRoutes(page, testInfo, viewport, [playlist, new URL(page.url()).pathname]);
	});
}
