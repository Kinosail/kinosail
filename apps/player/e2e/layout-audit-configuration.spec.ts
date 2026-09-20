import { expect, test } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { configureLayoutAudit, layoutProblems, login, viewports } from "./layout-audit-helpers";

configureLayoutAudit();

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
		await expect(updates.getByLabel("Choose when to update")).not.toBeChecked();
		await expect(updates.getByLabel("Install updates automatically")).toBeChecked();
		const alignment = await page.evaluate(() => ({ navigation: document.querySelector("[data-settings-nav]")!.getBoundingClientRect().bottom, updates: document.querySelector("#updates h2")!.getBoundingClientRect().top }));
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

test("Owner settings separates everyday preferences from advanced tools", async ({ page }, testInfo) => {
	test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
	await login(page);
	for (const viewport of [{ width: 1440, height: 900 }, { width: 1024, height: 768 }, { width: 900, height: 900 }, { width: 768, height: 1024 }, { width: 390, height: 844 }, { width: 320, height: 800 }]) {
		await page.setViewportSize(viewport);
		await page.goto("/settings", { waitUntil: "domcontentloaded" });
		const nav = page.locator("[data-settings-nav]");
		const levels = page.locator("[data-settings-levels]");
		await expect(nav.locator('[aria-current="page"]')).toHaveText("Playback & subtitles");
		await expect(levels.locator('[aria-current="page"]')).toHaveText("Basic");
		await expect(page.locator("#playback")).toBeVisible();
		await nav.getByRole("link", { name: "General", exact: true }).click();
		await page.getByRole("textbox", { name: "Server name", exact: true }).fill("Unsaved household name");
		await levels.getByRole("link", { name: "Advanced", exact: true }).click();
		await levels.getByRole("link", { name: "Basic", exact: true }).click();
		await nav.getByRole("link", { name: "General", exact: true }).click();
		await expect(page.getByRole("textbox", { name: "Server name", exact: true })).toHaveValue("Unsaved household name");
		await expect(page.locator("#transcoder")).toBeHidden();
		await expect(page.locator("#security")).toBeHidden();
		if (viewport.width <= 900) {
			const geometry = await nav.evaluate((element) => {
				const links = [...element.querySelectorAll("a")].filter(link => !link.hidden);
				return { rows: new Set(links.map(link => link.offsetTop)).size, minHeight: Math.min(...links.map(link => link.getBoundingClientRect().height)), overflow: document.documentElement.scrollWidth > innerWidth };
			});
			expect(geometry).toEqual({ rows: 1, minHeight: 44, overflow: false });
		}
		for (const [level, category, anchor] of [["Basic", "playback", "playback"], ["Basic", "household", "profiles"], ["Basic", "library", "library"], ["Basic", "appearance", "appearance"], ["Advanced", "network", "access"], ["Advanced", "security", "security"], ["Advanced", "integrations", "settings-integrations"], ["Advanced", "system", "transcoder"], ["Advanced", "migration", "viewing-imports"]]) {
			await levels.getByRole("link", { name: level, exact: true }).click();
			await nav.locator(`[data-settings-group="${category}"]`).click();
			await expect(page.locator(`#${anchor}`)).toBeVisible();
			await expect(page.locator(`[data-settings-flow]>[data-settings-category]:visible:not([data-settings-category="${category}"])`)).toHaveCount(0);
			expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
			await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-settings-${category}.png`), fullPage: true });
		}
		await page.goto("/settings#transcoder");
		await expect(levels.locator('[aria-current="page"]')).toHaveText("Advanced");
		await levels.getByRole("link", { name: "Basic", exact: true }).click();
		await page.goBack();
		await expect(page.locator("#transcoder")).toBeVisible();
		await page.goto("/settings#unknown-%broken");
		await expect(page.locator("#playback")).toBeVisible();
	}
});
