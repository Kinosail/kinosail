import { createHmac } from "node:crypto";
import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";

test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
test.use({ serviceWorkers: "block" });

test.afterEach(async ({ page }) => {
	if ((await page.request.get("/api/v1/me")).status() !== 200) return;
	const csrf = await page.locator('meta[name="kinosail-csrf"]').getAttribute("content");
	expect(csrf).toBeTruthy();
	const headers = { "X-Kinosail-CSRF": csrf!, Origin: new URL(page.url()).origin };
	expect((await page.request.put("/api/v1/settings/jellyfin", { headers, data: { enabled: false } })).ok()).toBeTruthy();
	expect((await page.request.delete("/api/v1/settings/trusted-https", { headers })).ok()).toBeTruthy();
});

function totp(): string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567";
	const bits = [...(process.env.KINOSAIL_TEST_TOTP_SECRET ?? "")].map((character) => alphabet.indexOf(character).toString(2).padStart(5, "0")).join("");
	const secret = Buffer.from(bits.match(/.{8}/g)?.map((byte) => Number.parseInt(byte, 2)) ?? []);
	const counter = Buffer.alloc(8);
	counter.writeBigUInt64BE(BigInt(Math.floor(Date.now() / 30_000)));
	const digest = createHmac("sha1", secret).update(counter).digest();
	const offset = digest[19] & 15;
	return ((digest.readUInt32BE(offset) & 0x7fffffff) % 1_000_000).toString().padStart(6, "0");
}

async function login(page: Page) {
	await page.goto("/login");
	await page.getByLabel("Name").fill("Owner");
	await page.getByLabel("Password", { exact: true }).fill("test-instance-password");
	await page.getByLabel("Authentication or recovery code").fill(totp());
	await page.getByRole("button", { name: "Sign in", exact: true }).click();
	if (await page.getByRole("link", { name: "Not now" }).isVisible()) await page.getByRole("link", { name: "Not now" }).click();
	await expect(page).toHaveURL("/");
}

test("Jellyfin setup stays blocked until trusted HTTPS is saved", async ({ page, browserName }, testInfo) => {
	test.setTimeout(120_000);
	await login(page);
	await page.goto("/onboarding/connection");
	const jellyfin = page.locator("#jellyfin");
	const choice = jellyfin.getByLabel("Allow compatible Jellyfin apps to connect");
	const disclosure = page.locator("#trusted-https-configuration");
	const trusted = disclosure.locator('form[action="/onboarding/trusted-https"]');
	await expect(choice).toBeDisabled();
	await expect(jellyfin.getByText("Many Jellyfin apps reject private certificates", { exact: false })).toBeVisible();

	for (const viewport of [{ width: 1440, height: 900 }, { width: 1024, height: 768 }, { width: 390, height: 844 }, { width: 320, height: 700 }]) {
		await page.setViewportSize(viewport);
		await expect(choice).toBeDisabled();
		await expect(disclosure.locator(":scope > summary")).toHaveText(/Set up trusted HTTPS/);
		await expect(trusted).toBeVisible();
		expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBeTruthy();
		await page.evaluate(() => (document.activeElement as HTMLElement)?.blur());
		await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-jellyfin-blocked.png`), fullPage: true });
	}
	expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
	await page.evaluate(() => { location.hash = "#trusted-https-configuration"; });
	await expect(trusted).toBeVisible();
	await page.reload({ waitUntil: "domcontentloaded" });
	await expect(trusted).toBeVisible();
	await disclosure.locator(":scope > summary").press("Enter");
	await expect(trusted).toBeHidden();

	const prerequisite = jellyfin.getByRole("link", { name: "Trusted HTTPS" });
	await prerequisite.focus();
	await page.keyboard.press("Enter");
	await expect(page).toHaveURL(/#trusted-https-configuration$/);
	await expect(trusted).toBeVisible();
	await expect(disclosure.locator(":scope > summary")).toBeFocused();
	await disclosure.locator(":scope > summary").press("Enter");
	await expect(trusted).toBeHidden();
	await prerequisite.click();
	await expect(trusted).toBeVisible();
	await expect(disclosure.locator(":scope > summary")).toBeFocused();
	await expect(page.getByText("myhome.duckdns.org", { exact: true })).toBeVisible();
	await expect(page.getByText("myhome-subtitles.duckdns.org", { exact: true })).toBeVisible();
	await expect(trusted).toContainText("no router port forwarding is needed");
	await expect(trusted).toContainText("default 38127");
	await expect(trusted).toContainText("Public remote access is a separate feature");
	await expect(trusted).toContainText("TCP 443");
	await expect(trusted).toContainText("a DuckDNS or deSEC account");
	await page.screenshot({ path: testInfo.outputPath("320-jellyfin-expanded.png"), fullPage: true });
	const explanation = trusted.getByText("Kinosail points the hostname at the private LAN address.");
	await expect(explanation).not.toBeVisible();
	await trusted.getByText("How trusted HTTPS works").click();
	await expect(explanation).toBeVisible();
	await expect(trusted).toContainText("renews the certificate automatically");
	await page.setViewportSize({ width: 390, height: 844 });
	expect(await page.locator("html").evaluate((element) => element.scrollWidth <= element.clientWidth)).toBe(true);
	expect((await new AxeBuilder({ page }).include("#trusted-https").analyze()).violations).toEqual([]);
	await page.screenshot({ path: testInfo.outputPath("390-jellyfin-explained.png"), fullPage: true });
	let validationFails = true;
	let validationRequest: Record<string, string | boolean> | undefined;
	await page.route("**/api/v1/settings/trusted-https/validate", async (route) => {
		validationRequest = route.request().postDataJSON();
		if (validationFails) {
			await route.fulfill({ status: 409, contentType: "application/json", body: JSON.stringify({ error: "trusted HTTPS conflicts with the current deployment\nthe configured sign-in address does not match this trusted HTTPS address" }) });
			return;
		}
		await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ status: "valid", trustedHttps: { hostname: "kinosail-e2e.duckdns.org" } }) });
	});
	await trusted.getByRole("group", { name: "DNS provider" }).locator('input[value="duckdns"]').check();
	await trusted.getByLabel("Trusted hostname").fill("kinosail-e2e");
	await trusted.getByLabel("Provider token").fill("t".repeat(32));
	await trusted.getByLabel("Kinosail LAN address").fill("server.nox");
	await trusted.getByLabel("Allow DNS validation and accept the Let's Encrypt subscriber agreement").check();
	const testStatus = trusted.locator("[data-trusted-https-test-status]");
	await expect(testStatus).toContainText("configured sign-in address does not match this trusted HTTPS address");
	await page.screenshot({ path: testInfo.outputPath("390-trusted-https-conflict.png"), fullPage: true });
	expect(validationRequest).toMatchObject({ provider: "duckdns", domain: "kinosail-e2e", token: "t".repeat(32), address: "server.nox", termsAccepted: true });
	validationFails = false;
	await trusted.getByLabel("Trusted hostname").fill("kinosail-e2e-ready");
	await trusted.getByLabel("Trusted hostname").fill("kinosail-e2e");
	await expect(testStatus).toHaveText("Details match this deployment for kinosail-e2e.duckdns.org. DNS has not been tested.");
	let testRequest: Record<string, string | boolean> | undefined;
	await page.route("**/api/v1/settings/trusted-https/test", async (route) => {
		testRequest = route.request().postDataJSON();
		await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ status: "passed", trustedHttps: { hostname: "kinosail-e2e.duckdns.org" } }) });
	});
	await trusted.getByRole("button", { name: "Test DNS connection" }).click();
	await expect(testStatus).toHaveText("Connection verified for kinosail-e2e.duckdns.org. Nothing was saved.");
	await page.screenshot({ path: testInfo.outputPath("390-trusted-https-test-passed.png"), fullPage: true });
	expect(testRequest).toMatchObject({ provider: "duckdns", domain: "kinosail-e2e", token: "t".repeat(32), address: "server.nox", termsAccepted: true });
	await trusted.getByLabel("Trusted hostname").fill("kinosail-e2e-changed");
	await expect(testStatus).toHaveText("Details changed. Test again.");
	await trusted.getByLabel("Trusted hostname").fill("kinosail-e2e");
	let saveFails = true;
	await page.route("**/api/v1/settings/trusted-https", async (route) => {
		if (saveFails && route.request().method() === "PUT") {
			await route.fulfill({ status: 409, contentType: "application/json", body: JSON.stringify({ error: "trusted HTTPS conflicts with the current deployment\nthe configured sign-in address does not match this trusted HTTPS address" }) });
			return;
		}
		await route.continue();
	});
	await trusted.getByRole("button", { name: "Save trusted HTTPS" }).click();
	await expect(page).toHaveURL(/\/onboarding\/connection/);
	await expect(testStatus).toContainText("configured sign-in address does not match this trusted HTTPS address");
	saveFails = false;
	await trusted.getByRole("button", { name: "Save trusted HTTPS" }).click();
	await expect(page).toHaveURL("/onboarding/connection");
	await expect(disclosure.locator(":scope > summary")).toHaveText(/Review trusted HTTPS setup/);
	await expect(trusted).toBeHidden();
	await expect(choice).toBeEnabled();
	await expect(jellyfin.getByText("You can now enable Jellyfin apps, then restart Kinosail once.")).toBeVisible();
	await choice.check();
	// macOS WebKit uses Option-Tab to include buttons in keyboard navigation.
	await choice.press(browserName === "webkit" && process.platform === "darwin" ? "Alt+Tab" : "Tab");
	await expect(jellyfin.getByRole("button", { name: "Save Jellyfin choice" })).toBeFocused();
	await jellyfin.getByRole("button", { name: "Save Jellyfin choice" }).click();
	await expect(page).toHaveURL("/onboarding/connection#jellyfin");
	await expect(jellyfin.getByText("Connect address:")).toContainText("https://kinosail-e2e.duckdns.org:38127");
	await expect.poll(async () => (await page.request.get("/System/Info/Public")).status()).toBe(200);

	await page.setViewportSize({ width: 390, height: 844 });
	expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBeTruthy();
	expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
	await page.evaluate(() => (document.activeElement as HTMLElement)?.blur());
	await page.screenshot({ path: testInfo.outputPath("390-jellyfin-enabled.png"), fullPage: true });
	await page.emulateMedia({ forcedColors: "active", reducedMotion: "reduce" });
	expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
	await page.screenshot({ path: testInfo.outputPath("forced-colors-jellyfin-enabled.png"), fullPage: true });

	const activity = await (await page.request.get("/api/v1/activity?limit=100")).text();
	expect(activity).toContain('"action":"settings.trusted-https.updated"');
	expect(activity).toContain('"action":"settings.jellyfin.updated"');
	expect(activity).not.toContain("t".repeat(32));

	await page.emulateMedia({ forcedColors: "none", reducedMotion: "no-preference" });
	await choice.uncheck();
	await jellyfin.getByRole("button", { name: "Save Jellyfin choice" }).click();
	await page.goto("/settings#trusted-https");
	await page.getByRole("button", { name: "Disable trusted HTTPS after restart" }).click();
	await expect(page).toHaveURL("/settings#trusted-https");
});
