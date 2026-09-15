import { createHash, createHmac } from "node:crypto";
import { expect, test, type Page } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";

test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");

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

async function signIn(page: Page) {
	await page.goto("/login");
	await page.getByLabel("Name").fill("Owner");
	await page.getByLabel("Password", { exact: true }).fill("test-instance-password");
	const secondsLeft = 30 - Math.floor(Date.now() / 1_000) % 30;
	if (secondsLeft < 15) await page.waitForTimeout((secondsLeft + 1) * 1_000);
	await page.getByLabel("6-digit code").fill(totp());
	await page.getByRole("button", { name: "Sign in", exact: true }).click();
	try {
		await page.waitForURL((url) => url.pathname !== "/login", { timeout: 2_000 });
	} catch {
		await page.getByLabel("6-digit code").fill(totp());
		await page.getByRole("button", { name: "Sign in", exact: true }).click();
	}
	if (await page.getByRole("link", { name: "Not now" }).isVisible()) await page.getByRole("link", { name: "Not now" }).click();
	await expect(page).toHaveURL("/");
}

async function openSettings(page: Page) {
	await page.goto("/settings#access");
	if (new URL(page.url()).pathname === "/login") {
		await signIn(page);
		await page.goto("/settings#access");
	}
}

test("Home Assistant stays dark until the Owner enables it in Settings and the wizard", async ({ page, request }, testInfo) => {
	test.setTimeout(300_000);
	await signIn(page);
	await openSettings(page);
	const setting = page.getByLabel("Home Assistant", { exact: true });
	if (await setting.isChecked()) {
		await setting.uncheck();
		await page.getByRole("button", { name: "Save Home Assistant", exact: true }).click();
	}
	await expect.poll(async () => (await request.get("/api/v1/home-assistant")).status()).toBe(404);
	await expect(page.getByRole("link", { name: "Add to Home Assistant" })).toHaveCount(0);
	await page.setViewportSize({ width: 390, height: 844 });
	await page.evaluate(() => (document.activeElement as HTMLElement)?.blur());
	await page.screenshot({ path: testInfo.outputPath("390-disabled-settings-home-assistant.png"), fullPage: true });

	await openSettings(page);
	await setting.check();
	await page.getByRole("button", { name: "Save Home Assistant", exact: true }).click();
	await expect.poll(async () => (await request.get("/api/v1/home-assistant")).status()).toBe(200);

	try {
		for (const viewport of [
			{ width: 1440, height: 900 },
			{ width: 1024, height: 768 },
			{ width: 720, height: 450 },
			{ width: 390, height: 844 },
			{ width: 320, height: 700 },
		]) {
			await page.setViewportSize(viewport);
			await openSettings(page);
			await expect(page.getByLabel("Home Assistant", { exact: true })).toBeChecked();
			await expect(page.getByRole("link", { name: "Add to Home Assistant" })).toBeVisible();
			await expect(page.getByText("Manual pairing", { exact: true })).toBeVisible();
			expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBeTruthy();
			await page.evaluate(() => (document.activeElement as HTMLElement)?.blur());
			await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-settings-home-assistant.png`), fullPage: true });

			await page.goto("/onboarding/connection");
			await expect(page.getByLabel("Connect Home Assistant", { exact: true })).toBeChecked();
			expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBeTruthy();
			await page.evaluate(() => (document.activeElement as HTMLElement)?.blur());
			await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-wizard-home-assistant.png`), fullPage: true });
		}

		await page.setViewportSize({ width: 390, height: 844 });
		await openSettings(page);
		if (testInfo.project.name === "chromium") {
			await page.getByLabel("Home Assistant", { exact: true }).focus();
			await page.keyboard.press("Tab");
			await expect(page.getByRole("button", { name: "Save Home Assistant", exact: true })).toBeFocused();
		}
		expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
		await page.getByText("Manual pairing", { exact: true }).click();
		await page.getByRole("button", { name: "Create one-time pairing code" }).click();
		await expect(page.getByRole("heading", { name: "Connect Home Assistant" })).toBeVisible();
		await expect(page.locator(".pairing-code code")).toHaveText(/^\d{8}$/);
		expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
		await page.screenshot({ path: testInfo.outputPath("390-home-assistant-pairing.png"), fullPage: true });

		const verifier = "v".repeat(64);
		const challenge = createHash("sha256").update(verifier).digest("base64url");
		const approval = new URLSearchParams({ response_type: "code", client_id: "home-assistant", redirect_uri: "https://my.home-assistant.io/redirect/oauth", state: "signed-state", code_challenge: challenge, code_challenge_method: "S256" });
		await page.goto(`/home-assistant/authorize?${approval}`);
		await expect(page.getByRole("heading", { name: "Allow Home Assistant to connect?" })).toBeVisible();
		await expect(page.getByRole("button", { name: "Allow connection" })).toBeVisible();
		expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
		await page.screenshot({ path: testInfo.outputPath("390-home-assistant-approval.png"), fullPage: true });

		await page.emulateMedia({ forcedColors: "active", reducedMotion: "reduce" });
		await page.goto("/onboarding/connection");
		await expect(page.getByLabel("Connect Home Assistant", { exact: true })).toBeChecked();
		expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
		await page.screenshot({ path: testInfo.outputPath("forced-colors-wizard-home-assistant.png"), fullPage: true });
	} finally {
		await page.emulateMedia({ forcedColors: "none", reducedMotion: "no-preference" });
		await openSettings(page);
		const enabled = page.getByLabel("Home Assistant", { exact: true });
		if (await enabled.isChecked()) {
			await enabled.uncheck();
			await page.getByRole("button", { name: "Save Home Assistant", exact: true }).click();
		}
	}
});
