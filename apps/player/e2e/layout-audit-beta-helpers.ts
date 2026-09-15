import { chromium, expect, firefox, webkit, type Page, type TestInfo } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { attachUIInventory, layoutProblems, login, presentationProblems, viewports } from "./layout-audit-helpers";

export const betaRouteGroups = [
	{
		name: "library",
		routes: ["/", "/?view=list", "/?view=movies", "/?view=shows", "/?view=collections", "/?view=playlists"],
	},
	{
		name: "library more",
		routes: ["/?view=music", "/?view=audiobooks", "/?view=books", "/?view=photos", "/?view=history", "/?view=unwatched"],
	},
	{
		name: "utilities",
		routes: ["/quick-connect", "/offline-downloads", "/supporter"],
	},
	{
		name: "settings",
		routes: ["/settings", "/settings#playback", "/settings#access", "/settings#system"],
	},
	{
		name: "system settings",
		routes: ["/settings/configuration", "/settings/backups", "/settings/system", "/settings/agent-connections"],
	},
	{
		name: "connection settings",
		routes: ["/settings/media-shares", "/settings/remote-readiness"],
	},
	{
		name: "onboarding",
		routes: ["/onboarding/connection", "/onboarding/household", "/onboarding/migrate", "/account"],
	},
] as const;

export async function auditBetaRoutes(page: Page, testInfo: TestInfo, viewport: (typeof viewports)[number], routes: string[]) {
	const violations: {
		route: string;
		viewport: number;
		problems: Awaited<ReturnType<typeof layoutProblems>>;
	}[] = [];
	const presentationViolations: {
		route: string;
		viewport: number;
		problems: Awaited<ReturnType<typeof presentationProblems>>;
	}[] = [];
	const accessibilityViolations: { route: string; rules: string[] }[] = [];
	const storageState = await page.context().storageState();
	const browserType = { chromium, firefox, webkit }[testInfo.project.name as "chromium" | "firefox" | "webkit"];
	const routeBrowser = await browserType.launch();
	try {
		for (const route of routes) {
			const routeContext = await routeBrowser.newContext({
				baseURL: process.env.KINOSAIL_E2E_URL ?? "https://127.0.0.1:38127",
				ignoreHTTPSErrors: true,
				storageState,
				viewport,
			});
			const routePage = await routeContext.newPage();
			try {
				const response = await routePage.goto(route, {
					waitUntil: "domcontentloaded",
				});
				expect(response?.ok(), route).toBeTruthy();
				await expect(routePage.locator("main")).toBeVisible();
				const stylesheet = /\/static\/app\.css\?v=\d+$/;
				await expect(routePage.locator('link[rel="stylesheet"][href^="/static/app.css"]')).toHaveAttribute("href", stylesheet);
				await expect(routePage.locator('link[rel="stylesheet"][href^="/static/app.css"]')).toHaveCount(1);
				if (route === "/settings/configuration") await expect(routePage.getByLabel("SCIM token expiration date")).toBeVisible();
				if (route === "/settings") {
					for (const key of ["integrations.tmdb", "integrations.oidc", "integrations.scim", "integrations.webhook.url", "dlna.url", "backup.key"]) {
						await expect(routePage.locator(`a[href="/settings/configuration#${key}"]`)).toHaveCount(1);
					}
				}
				const problems = await layoutProblems(routePage);
				await attachUIInventory(routePage, testInfo, `${viewport.width}-${route.replace(/[^a-z0-9]+/gi, "-")}`);
				await routePage.screenshot({
					path: testInfo.outputPath(`${viewport.width}-${route.replace(/[^a-z0-9]+/gi, "-") || "home"}.png`),
					fullPage: true,
				});
				if (problems.documentOverflow || problems.outside.length || problems.tinyControls.length || problems.distortedChecks.length || problems.clippedControls.length || problems.overlappingStatuses.length) violations.push({ route, viewport: viewport.width, problems });
				const presentation = await presentationProblems(routePage);
				if (presentation.blurred.length || presentation.shadowed.length || presentation.overRounded.length || presentation.settingsBackdrop || presentation.oversizedSettingsHeading)
					presentationViolations.push({
						route,
						viewport: viewport.width,
						problems: presentation,
					});
				const rules = (await new AxeBuilder({ page: routePage }).analyze()).violations.map(({ id }) => id);
				if (rules.length)
					accessibilityViolations.push({
						route: `${route} at ${viewport.width}px`,
						rules,
					});
			} finally {
				await routeContext.close();
			}
		}
	} finally {
		await routeBrowser.close();
	}
	expect(violations).toEqual([]);
	expect(presentationViolations).toEqual([]);
	expect(accessibilityViolations).toEqual([]);
}
