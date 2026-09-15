import AxeBuilder from "@axe-core/playwright";
import { chromium, expect, firefox, test, webkit, type Page, type TestInfo } from "@playwright/test";
import { layoutProblems, login, presentationProblems, totp, viewports } from "./layout-audit-helpers";

export function registerLayoutSettingsTests() {
test("SCIM configuration stays usable at every supported width", async ({ page }, testInfo) => {
	test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
	test.setTimeout(120_000);
	await login(page);
	for (const viewport of viewports) {
		await page.setViewportSize(viewport);
		await page.goto("/settings/configuration#integrations.scim");
		const section = page.locator('section[id="integrations.scim"]');
		const endpoint = section.getByLabel("SCIM base URL");
		const token = section.getByLabel("SCIM bearer token");
		const expiration = section.getByLabel("SCIM token expiration date");
		await expect(section.getByRole("heading", { name: "SCIM provisioning" })).toBeVisible();
		await expect(endpoint).toHaveValue(/\/scim\/v2$/);
		await expect(endpoint).toHaveAttribute("readonly", "");
		await expect(token).toHaveAttribute("type", "password");
		expect(await token.inputValue()).toMatch(/^[A-Za-z0-9_-]{43}$/);
		await expect(expiration).toHaveAttribute("type", "date");
		if (viewport.width === 1440) {
			await section.getByRole("button", { name: "Copy URL" }).click();
			await expect(section.locator(".scim-endpoint .copy-status")).toHaveText(/Copied|Select Copy/);
			await section.getByRole("button", { name: "Copy token" }).click();
			await expect(section.locator(".scim-fields .copy-status")).toHaveText(/Copied|Select Copy/);
		}
		const columns = await section.locator(".scim-fields").evaluate((element) => getComputedStyle(element).gridTemplateColumns.split(" ").filter(Boolean).length);
		expect(columns, `${viewport.width}px field reflow`).toBe(viewport.width > 700 ? 2 : 1);
		const problems = await layoutProblems(page);
		expect(Boolean(problems.documentOverflow), `${viewport.width}px overflow`).toBe(false);
		expect({ ...problems, documentOverflow: false }, `${viewport.width}px layout`).toEqual({ documentOverflow: false, outside: [], tinyControls: [], distortedChecks: [], clippedControls: [], overlappingStatuses: [] });
		expect((await new AxeBuilder({ page }).analyze()).violations.map(({ id }) => id), `${viewport.width}px accessibility`).toEqual([]);
		await section.screenshot({ path: testInfo.outputPath(`scim-configuration-${viewport.width}.png`) });
	}
});

test("single sign-on configuration stays usable at every supported width", async ({ page }, testInfo) => {
	test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
	test.setTimeout(120_000);
	await login(page);
	for (const viewport of viewports) {
		await page.setViewportSize(viewport);
		await page.goto("/settings/configuration#integrations.oidc");
		const section = page.locator('section[id="integrations.oidc"]');
		const redirect = section.getByLabel("Single sign-on return address");
		const identity = section.getByLabel("Single sign-on identity claim");
		await expect(section.getByRole("heading", { name: "Single sign-on" })).toBeVisible();
		await expect(section.getByLabel("Single sign-on client secret")).toHaveAttribute("type", "password");
		await expect(redirect).toHaveAttribute("placeholder", "https://media.example.com/login/oidc/callback");
		await expect(identity).toHaveValue(/^(sub|oid)$/);
		await expect(identity).toHaveAttribute("required", "");
		const columns = await section.locator(".oidc-fields").evaluate((element) => getComputedStyle(element).gridTemplateColumns.split(" ").filter(Boolean).length);
		expect(columns, `${viewport.width}px field reflow`).toBe(viewport.width > 700 ? 2 : 1);
		const problems = await layoutProblems(page);
		expect(Boolean(problems.documentOverflow), `${viewport.width}px overflow`).toBe(false);
		expect({ ...problems, documentOverflow: false }, `${viewport.width}px layout`).toEqual({ documentOverflow: false, outside: [], tinyControls: [], distortedChecks: [], clippedControls: [], overlappingStatuses: [] });
		expect((await new AxeBuilder({ page }).analyze()).violations.map(({ id }) => id), `${viewport.width}px accessibility`).toEqual([]);
		await section.screenshot({ path: testInfo.outputPath(`oidc-configuration-${viewport.width}.png`) });
	}
	await page.setViewportSize({ width: 1440, height: 900 });
	await page.goto("/settings/configuration#integrations.oidc");
	const section = page.locator('section[id="integrations.oidc"]');
	await section.getByLabel("Single sign-on return address").fill("https://media.example.com/login/oidc/callback");
	await section.getByLabel("Single sign-on issuer").fill("https://identity.example.com/realms/family");
	await section.getByLabel("Single sign-on client ID").fill("kinosail");
	await section.getByLabel("Single sign-on client secret").fill("test-secret");
	await section.getByLabel("Single sign-on identity claim").fill("oid");
	await section.getByRole("button", { name: "Save single sign-on" }).click();
	await expect(page.locator('section[id="integrations.oidc"]')).toContainText("Single sign-on is configured.");
	await expect(page.getByLabel("Single sign-on client secret")).toHaveAttribute("placeholder", "Leave blank to keep the configured secret");
	await expect(page.getByRole("button", { name: "Disable single sign-on" })).toBeVisible();
	await page.locator('section[id="integrations.oidc"]').screenshot({ path: testInfo.outputPath("oidc-configuration-configured-1440.png") });
});

test("SAML configuration stays usable at every supported width", async ({ page }, testInfo) => {
	test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
	test.setTimeout(120_000);
	await login(page);
	for (const viewport of viewports) {
		await page.setViewportSize(viewport);
		await page.goto("/settings/configuration#integrations.saml");
		const section = page.locator('section[id="integrations.saml"]');
		await expect(section.getByRole("heading", { name: "SAML single sign-on" })).toBeVisible();
		await expect(section.getByLabel("SAML provider metadata URL")).toHaveAttribute("type", "url");
		await expect(section.getByLabel("SAML provider metadata XML")).toHaveAttribute("maxlength", "262144");
		await expect(section.getByLabel("SAML identity attribute")).toHaveValue("NameID");
		const problems = await layoutProblems(page);
		expect({ ...problems, documentOverflow: false }, `${viewport.width}px layout`).toEqual({ documentOverflow: false, outside: [], tinyControls: [], distortedChecks: [], clippedControls: [], overlappingStatuses: [] });
		expect((await new AxeBuilder({ page }).analyze()).violations.map(({ id }) => id), `${viewport.width}px accessibility`).toEqual([]);
		await section.screenshot({ path: testInfo.outputPath(`saml-configuration-${viewport.width}.png`) });
	}
});

test("software update choices stay clear at supported widths", async ({ page }, testInfo) => {
	test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
	test.setTimeout(120_000);
	await login(page);
	for (const viewport of [...viewports, { width: 320, height: 800 }]) {
		await page.setViewportSize(viewport);
		await page.goto("/settings#updates");
		const updates = page.locator("#updates");
		await expect(updates.getByRole("heading", { name: "Software updates" })).toBeVisible();
		await expect(updates.getByLabel("Choose when to update")).toBeChecked();
		await expect(updates.getByLabel("Install updates automatically")).not.toBeChecked();
		const alignment = await page.evaluate(() => ({ navigation: document.querySelector("[data-settings-nav]")!.getBoundingClientRect().bottom, updates: document.querySelector("#updates")!.getBoundingClientRect().top }));
		expect(alignment.updates, `${viewport.width}px update heading below navigation`).toBeGreaterThanOrEqual(alignment.navigation);
		expect(await layoutProblems(page), `${viewport.width}px update layout`).toEqual({ documentOverflow: 0, outside: [], tinyControls: [], distortedChecks: [], clippedControls: [], overlappingStatuses: [] });
		await page.evaluate(() => (document.activeElement as HTMLElement)?.blur());
		await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-settings-updates.png`), fullPage: true });
		expect((await new AxeBuilder({ page }).analyze()).violations, `${viewport.width}px update accessibility`).toEqual([]);
	}
});

test("Media Shares keeps its heading and controls in a readable composition", async ({ page }) => {
	test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
	await login(page);
	for (const viewport of [...viewports, { width: 320, height: 800 }]) {
		await page.setViewportSize(viewport);
		await page.goto("/settings/media-shares", { waitUntil: "domcontentloaded" });
		const composition = await page.evaluate(() => {
			const heading = document.querySelector(".settings-intro h1")!;
			const form = document.querySelector(".media-share-form")!;
			const fieldset = form.querySelector("fieldset")!;
			const headingStyle = getComputedStyle(heading);
			const headingBox = heading.getBoundingClientRect();
			return {
				headingLines: Math.round(headingBox.height / Number.parseFloat(headingStyle.lineHeight)),
				headingWidth: headingBox.width,
				formWidth: form.getBoundingClientRect().width,
				fieldsetWidth: fieldset.getBoundingClientRect().width,
			};
		});
		expect(composition.headingLines, `${viewport.width}px heading lines`).toBeLessThanOrEqual(2);
		expect(composition.headingWidth, `${viewport.width}px heading width`).toBeGreaterThan(250);
		expect(composition.formWidth, `${viewport.width}px form width`).toBeGreaterThan(280);
		expect(composition.fieldsetWidth, `${viewport.width}px content width`).toBeGreaterThan(280);
	}
});

test("Owner settings keeps every task family in one list", async ({ page }, testInfo) => {
	test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
	await login(page);
	for (const viewport of [{ width: 1440, height: 900 }, { width: 1024, height: 768 }, { width: 390, height: 844 }, { width: 320, height: 800 }]) {
		await page.setViewportSize(viewport);
		await page.goto("/settings", { waitUntil: "domcontentloaded" });
		const settingsNav = page.locator("[data-settings-nav]");
		await expect(settingsNav.locator('[aria-current="page"]')).toHaveText("Playback");
		const settingsSections = page.locator('[data-settings-flow]>[data-settings-group]');
		expect(await page.locator('[data-settings-flow]>[data-settings-group]:visible').count()).toBe(await settingsSections.count());
		expect((await page.locator("[data-settings-flow]").evaluate((element) => getComputedStyle(element).gridTemplateColumns)).split(" ")).toHaveLength(1);
		await expect(page.locator("#onboarding")).toBeVisible();
		for (const [label, group] of [["Playback", "playback"], ["Access", "access"], ["Library", "library"], ["General", "general"], ["Appearance", "appearance"], ["System", "system"], ["Migration", "migration"]] as const) {
			const link = settingsNav.getByRole("link", { name: label, exact: true });
			await link.click();
			const anchor = group === "migration" ? "viewing-imports" : group === "general" ? "security" : group;
			await expect(page).toHaveURL(new RegExp(`#${anchor}$`));
			await expect(settingsNav.locator('[aria-current="page"]')).toHaveText(label);
			expect(await page.locator('[data-settings-flow]>[data-settings-group]:visible').count()).toBe(await settingsSections.count());
			await expect(page.locator(`#${anchor}`)).toBeVisible();
			await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-settings-${group}.png`), fullPage: true });
		}
	}
});

test("signed-in application pages retain the navigation shell", async ({ page }, testInfo) => {
	test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
	await login(page);
	for (const viewport of viewports) {
		await page.setViewportSize(viewport);
		await page.goto("/settings", { waitUntil: "domcontentloaded" });
		await expect(page.getByRole("navigation", { name: "Main navigation" })).toBeVisible();
		await expect(page.locator(".brand-lockup")).toBeVisible();
		await expect(page.locator("body")).toHaveClass(/library-page/);
		await expect(page.locator("main#main")).toHaveCount(1);
		await expect(page.locator(".skip")).toHaveCount(1);
		await expect(page.locator("[data-command-open]")).toHaveCount(0);
		await expect(page.locator('a[href="/"]').filter({ hasText: "Browse library" }).first()).toHaveAttribute("href", "/");
		expect((await new AxeBuilder({ page }).analyze()).violations, `settings accessibility at ${viewport.width}px`).toEqual([]);
		expect(await layoutProblems(page), `settings shell at ${viewport.width}px`).toEqual({ documentOverflow: 0, outside: [], tinyControls: [], distortedChecks: [], clippedControls: [], overlappingStatuses: [] });
		const shell = await page.evaluate(() => {
			const navigation = document.querySelector(".app-header")!.getBoundingClientRect();
			const main = document.querySelector("main")!.getBoundingClientRect();
			return { navigation: { left: Math.round(navigation.left), right: Math.round(navigation.right), bottom: Math.round(navigation.bottom) }, main: { left: Math.round(main.left) }, viewport: { width: innerWidth, height: innerHeight } };
		});
		if (viewport.width > 900) {
			expect(shell.navigation).toMatchObject({ left: 0, right: 256, bottom: viewport.height });
			expect(shell.main.left).toBeGreaterThanOrEqual(256);
			expect(await page.locator(".app-header").evaluate((element) => element.scrollTop)).toBe(0);
		} else {
			const dock = await page.locator('.app-header nav').evaluate((element) => {
				const box = element.getBoundingClientRect();
				const header = getComputedStyle(element.closest('.app-header')!);
				return { bottom: Math.round(box.bottom), position: getComputedStyle(element).position, headerBackdrop: header.backdropFilter, headerTransform: header.transform };
			});
			expect(dock).toMatchObject({ bottom: viewport.height, position: "fixed", headerBackdrop: "none", headerTransform: "none" });
			expect(await page.locator('.app-header nav > a, .app-header nav > details').count()).toBeGreaterThanOrEqual(5);
		}
		if (viewport.width === 390) {
			await page.locator(".nav-more > summary").click();
			await expect(page.locator(".nav-more-menu")).toBeVisible();
		}
		await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-settings-navigation-shell.png`) });
	}

	await page.setViewportSize({ width: 1440, height: 900 });
	await page.goto("/");
	const show = await page.locator('a[href^="/show/"]').first().getAttribute("href");
	expect(show).toBeTruthy();
	await page.goto(show!);
	await expect(page.getByRole("navigation", { name: "Main navigation" }).getByRole("link", { name: "Shows", exact: true })).toHaveAttribute("aria-current", "page");
	await page.screenshot({ path: testInfo.outputPath("1440-show-navigation-shell.png") });

	await page.goto("/");
	const player = await page.locator('a[href^="/watch/"]').first().getAttribute("href");
	expect(player).toBeTruthy();
	await page.goto(player!);
	await expect(page.getByRole("navigation", { name: "Main navigation" })).toHaveCount(0);
});
}
