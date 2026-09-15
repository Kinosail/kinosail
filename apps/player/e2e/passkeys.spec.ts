import { readFile } from "node:fs/promises";
import { expect, test, type Page } from "@playwright/test";

const passkeys = await readFile(new URL("../../../packages/webassets/static/passkeys.js", import.meta.url), "utf8");
const mockOrigin = process.env.KINOSAIL_E2E_URL ?? "http://localhost:38127";

type BeginResponse = { status: number; contentType?: string; body?: string; headers?: Record<string, string> };

async function mockPasskeyPage(page: Page, origin: string, begin: () => BeginResponse, body = `<body data-passkey-waiting="Waiting for your passkey…" data-passkey-added="Passkey added." data-passkey-failed="Could not use your passkey. Try again or use another sign-in method." data-passkey-insecure="Passkeys need the configured Server address and trusted HTTPS. Open that address, trust the Server certificate, or use an authenticator."><button data-passkey-add>Create passkey</button><output data-passkey-status></output><script src="/static/passkeys.js"></script></body>`): Promise<string> {
	if (origin === mockOrigin) {
		origin = "http://localhost:38128";
	}
	await page.route(`${origin}/**`, async (route) => {
		const url = new URL(route.request().url());
		if (url.pathname === "/static/passkeys.js") return route.fulfill({ contentType: "text/javascript", body: passkeys });
		if (url.pathname === "/api/v1/me") return route.fulfill({ json: { csrf: "test-session-csrf" } });
		if (url.pathname.startsWith("/api/v1/passkeys/register/")) expect(route.request().headers()["x-kinosail-csrf"]).toBe("test-session-csrf");
		if (url.pathname.endsWith("/begin")) return route.fulfill(begin());
		if (url.pathname.endsWith("/finish")) return route.fulfill({ status: 204 });
		if (url.pathname === "/") return route.fulfill({ contentType: "text/html", body: "<h1>Library</h1>" });
		return route.fulfill({ contentType: "text/html", body });
	});
	await page.addInitScript(() => {
		(window as typeof window & { passkeyMediations: string[]; passkeySucceeds?: boolean }).passkeyMediations = [];
		Object.defineProperty(window, "PublicKeyCredential", { value: { parseCreationOptionsFromJSON: (options: object) => options, parseRequestOptionsFromJSON: (options: object) => options, isConditionalMediationAvailable: async () => true } });
		Object.defineProperty(navigator, "credentials", { value: { create: async () => ({ id: "credential", type: "public-key" }), get: async ({ mediation }: { mediation?: string }) => {
			const state = window as typeof window & { passkeyMediations: string[]; passkeySucceeds?: boolean };
			state.passkeyMediations.push(mediation ?? "modal");
			if (state.passkeySucceeds && !mediation) return { id: "credential", type: "public-key" };
			throw new DOMException("no passkey selected", "NotAllowedError");
		} } });
	});
	return origin;
}

test("a declined conditional passkey offer stays silent", async ({ page }) => {
	const origin = await mockPasskeyPage(page, mockOrigin, () => ({ status: 200, contentType: "application/json", body: '{"publicKey":{}}' }), `<body data-passkey-failed="Could not use your passkey. Try again or use another sign-in method."><input autocomplete="username webauthn"><output data-passkey-status></output><script src="/static/passkeys.js"></script></body>`);
	await page.goto(`${origin}/login`, {waitUntil: "commit"});
	await expect(page.locator("input[autocomplete='username webauthn']")).toBeVisible();
	await expect(page.getByRole("status")).toBeEmpty();
});

test("a returning passkey browser opens the chooser and falls back silently when dismissed", async ({ page }) => {
	const origin = await mockPasskeyPage(page, mockOrigin, () => ({ status: 200, contentType: "application/json", body: '{"publicKey":{}}' }), `<body data-passkey-waiting="Waiting for your passkey…"><input autocomplete="username webauthn"><button data-passkey-login>Sign in with passkey</button><output data-passkey-status></output><script src="/static/passkeys.js"></script></body>`);
	await page.addInitScript(() => localStorage.setItem("kinosail-passkey", "1"));
	await page.goto(`${origin}/login`, {waitUntil: "commit"});
	await expect.poll(() => page.evaluate(() => (window as typeof window & { passkeyMediations: string[] }).passkeyMediations)).toEqual(["modal", "conditional"]);
	await expect(page.getByRole("status")).toBeEmpty();
});

test("a successful passkey sign-in enables the return-visit chooser", async ({ page }) => {
	const origin = await mockPasskeyPage(page, mockOrigin, () => ({ status: 200, contentType: "application/json", body: '{"publicKey":{}}' }), `<body data-passkey-waiting="Waiting for your passkey…"><input autocomplete="username webauthn"><button data-passkey-login>Sign in with passkey</button><output data-passkey-status></output><script src="/static/passkeys.js"></script></body>`);
	await page.addInitScript(() => { (window as typeof window & { passkeySucceeds?: boolean }).passkeySucceeds = true; });
	await page.goto(`${origin}/login`, {waitUntil: "commit"});
	await page.getByRole("button", { name: "Sign in with passkey" }).click();
	await expect(page).toHaveURL(`${origin}/`);
	await expect.poll(() => page.evaluate(() => localStorage.getItem("kinosail-passkey"))).toBe("1");
});

test("successful required passkey enrollment leaves the requirement page", async ({ page }) => {
	const origin = await mockPasskeyPage(page, mockOrigin, () => ({ status: 200, contentType: "application/json", body: '{"publicKey":{}}' }));
	await page.goto(`${origin}/account?mfa=required`, {waitUntil: "commit"});
	await page.getByRole("button", { name: "Create passkey" }).click();
	await expect(page).toHaveURL(`${origin}/`);
});

test("successful offered enrollment continues to the requested page", async ({ page }) => {
	const origin = await mockPasskeyPage(page, mockOrigin, () => ({ status: 200, contentType: "application/json", body: '{"publicKey":{}}' }), `<body data-login-next="/?view=movies" data-passkey-waiting="Waiting for your passkey…" data-passkey-added="Passkey added." data-passkey-failed="Could not use your passkey. Try again or use another sign-in method."><button data-passkey-add>Add a passkey</button><output data-passkey-status></output><script src="/static/passkeys.js"></script></body>`);
	await page.goto(`${origin}/account?passkey=offer&next=%2F%3Fview%3Dmovies`, {waitUntil: "commit"});
	await page.getByRole("button", { name: "Add a passkey" }).click();
	await expect(page).toHaveURL(`${origin}/?view=movies`);
	await expect.poll(() => page.evaluate(() => localStorage.getItem("kinosail-passkey"))).toBe("1");
});

	test("an insecure passkey page explains how to recover", async ({ page }) => {
	if (test.info().project.name === "webkit") {
		await mockPasskeyPage(page, "http://passkey.test", () => ({ status: 200, contentType: "application/json", body: '{"publicKey":{}}' }));
		await page.goto("http://passkey.test/account?mfa=required", {waitUntil: "commit"});
		await page.getByRole("button", { name: "Create passkey" }).click();
		await expect(page.getByRole("status")).toHaveText("Passkeys need the configured Server address and trusted HTTPS. Open that address, trust the Server certificate, or use an authenticator.");
		return;
	}
	await mockPasskeyPage(page, "http://passkey.test", () => ({ status: 200, contentType: "application/json", body: '{"publicKey":{}}' }));
	await page.goto("http://passkey.test/account?mfa=required", {waitUntil: "commit"});
	await page.getByRole("button", { name: "Create passkey" }).click();
	await expect(page.getByRole("status")).toHaveText("Passkeys need the configured Server address and trusted HTTPS. Open that address, trust the Server certificate, or use an authenticator.");
});

test("a passkey origin response sends the browser to the configured address", async ({ page }) => {
	const origin = await mockPasskeyPage(page, mockOrigin, () => ({ status: 421, headers: { location: "https://media.example:38127/account" } }));
	await page.route("https://media.example:38127/**", (route) => route.fulfill({ contentType: "text/html", body: "<h1>Ways to sign in</h1>" }));
	await page.goto(`${origin}/account?mfa=required`, {waitUntil: "commit"});
	await page.getByRole("button", { name: "Create passkey" }).click();
	await expect(page).toHaveURL("https://media.example:38127/account");
});
