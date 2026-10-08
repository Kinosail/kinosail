import { configureProviderProfile, providerRoute } from "./provider-profile-fixture";
import { createHmac } from "node:crypto";
import { finishRootSignIn } from "./test-instance-helpers";
import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";

configureProviderProfile();
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
	const origin = new URL(page.url()).origin;
	await page.getByRole("button", { name: "Sign in", exact: true }).click();
	await finishRootSignIn(page, origin);
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
	await providerRoute(page, "**/api/v1/settings/trusted-https/validate", async (route) => {
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
	await providerRoute(page, "**/api/v1/settings/trusted-https/test", async (route) => {
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
	await providerRoute(page, "**/api/v1/settings/trusted-https", async (route) => {
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
	const expectedOrigin = new URL(testInfo.project.use.baseURL!).origin;
	const put = {requests: 0, responses: 0, failures: 0, status: null as number | null};
	const nativePost = {requests: 0, responses: 0, failures: 0, status: null as number | null};
	const facts = {documentStarted: false, documentCommitted: false};
	const ownedURL = (raw: string) => {
		if (typeof raw !== "string" || raw.length > 4096) return null;
		try {const url = new URL(raw); return url.origin === expectedOrigin && !url.username && !url.password ? url : null;}
		catch {return null;}
	};
	const entry = (raw: string, method: string) => {
		const url = ownedURL(raw);
		if (!url || url.search || url.hash) return null;
		if (url.pathname === "/api/v1/settings/trusted-https" && method === "PUT") return put;
		if (url.pathname === "/onboarding/trusted-https" && method === "POST") return nativePost;
		return null;
	};
	const requested = (request: {url(): string; method(): string; isNavigationRequest(): boolean; frame(): unknown}) => {
		const row = entry(request.url(), request.method());
		if (row) row.requests = Math.min(8, row.requests + 1);
		const url = ownedURL(request.url());
		if (url?.pathname === "/onboarding/connection" && !url.search && request.isNavigationRequest() && request.frame() === page.mainFrame()) facts.documentStarted = true;
	};
	const responded = (response: {url(): string; status(): number; request(): {method(): string}}) => {
		const row = entry(response.url(), response.request().method());
		if (!row) return;
		row.responses = Math.min(8, row.responses + 1);
		const status = response.status();
		row.status = Number.isInteger(status) && status >= 100 && status <= 599 ? status : null;
	};
	const failed = (request: {url(): string; method(): string}) => {
		const row = entry(request.url(), request.method());
		if (row) row.failures = Math.min(8, row.failures + 1);
	};
	const committed = (frame: {url(): string}) => {
		const url = ownedURL(frame.url());
		if (frame === page.mainFrame() && url?.pathname === "/onboarding/connection" && !url.search) facts.documentCommitted = true;
	};
	page.on("request", requested); page.on("response", responded); page.on("requestfailed", failed); page.on("framenavigated", committed);
	try {
		await trusted.getByRole("button", { name: "Save trusted HTTPS" }).click();
		await expect(page).toHaveURL("/onboarding/connection");
	} finally {
		const current = ownedURL(page.url());
		const snapshot = {putRequests: put.requests, putResponses: put.responses, putFailures: put.failures, putStatus: put.status,
			nativePostRequests: nativePost.requests, nativePostResponses: nativePost.responses, nativePostFailures: nativePost.failures, nativePostStatus: nativePost.status,
			...facts, currentOriginOwned: Boolean(current), currentPath: current ? (current.pathname === "/onboarding/connection" ? "connection" : "other") : "unavailable",
			currentFragment: current ? (current.hash === "" ? "none" : current.hash === "#trusted-https-configuration" ? "trusted-https" : current.hash === "#jellyfin" ? "jellyfin" : "other") : "unavailable"};
		let timer: ReturnType<typeof setTimeout> | undefined;
		try {await Promise.race([testInfo.attach("jellyfin-save-stages", {contentType: "application/json", body: JSON.stringify(snapshot)}),
			new Promise(resolve => {timer = setTimeout(resolve, 500);})]);}
		catch { /* Diagnostic failure cannot replace the Save or destination assertion. */ }
		finally {clearTimeout(timer); page.off("request", requested); page.off("response", responded); page.off("requestfailed", failed); page.off("framenavigated", committed);}
	}
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
