import { createHmac } from "node:crypto";
import { expect, test } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";

test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
test.beforeEach(async ({ page }) => page.addInitScript(() => Object.defineProperty(PublicKeyCredential, "isConditionalMediationAvailable", { value: async () => false })));

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

test("passwords can be shown and hidden without changing their value", async ({ page, browserName }) => {
	await page.goto("/login");
	const password = page.getByLabel("Password", { exact: true });
	await password.fill("test-instance-password");
	const reveal = page.getByRole("button", { name: "Show secret" });

	await expect(password).toHaveAttribute("type", "password");
	await expect(reveal).toHaveAttribute("aria-pressed", "false");
	await reveal.click();
	await expect(password).toHaveAttribute("type", "text");
	await expect(password).toHaveValue("test-instance-password");
	await expect(page.getByRole("button", { name: "Hide secret" })).toHaveAttribute("aria-pressed", "true");
	await page.getByRole("button", { name: "Hide secret" }).click();
	await expect(password).toHaveAttribute("type", "password");
	await password.focus();
	// macOS WebKit uses Option-Tab to include buttons in keyboard navigation.
	await page.keyboard.press(browserName === "webkit" && process.platform === "darwin" ? "Alt+Tab" : "Tab");
	await expect(reveal).toBeFocused();
	await page.keyboard.press("Space");
	await expect(password).toHaveAttribute("type", "text");
	expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);

	await page.getByLabel("Name").fill("Owner");
	await page.getByLabel("Authentication or recovery code").fill(totp());
	await page.getByRole("button", { name: "Sign in", exact: true }).click();
	await expect(page.getByRole("heading", { name: "Make the next sign-in easier" })).toBeVisible();
	await expect(page.getByText("Add one directly on another device")).toBeVisible();
	for (const viewport of [{ width: 390, height: 844 }, { width: 320, height: 800 }]) {
		await page.setViewportSize(viewport);
		for (const target of [page.getByText("Add one directly on another device", { exact: true }), page.getByRole("link", { name: "Not now" })]) expect((await target.boundingBox())!.height).toBeGreaterThanOrEqual(44);
	}
	await page.setViewportSize({ width: 1440, height: 900 });
	await page.getByRole("link", { name: "Not now" }).click();
	await expect(page).toHaveURL("/");
	await page.goto("/settings");
	await page.getByRole("link", { name: "Advanced", exact: true }).click();
	await page.getByRole("link", { name: "Connections", exact: true }).click();
	const token = page.getByLabel("Provider token", { exact: true });
	await expect(token).toBeVisible();
	const passwordInputs = page.locator("input[data-password-toggle]");
	await expect(page.locator(".password-control")).toHaveCount(await passwordInputs.count());
});

test("the password reveal control stays aligned across responsive settings", async ({ page, browserName }, testInfo) => {
	await page.goto("/login");
	await page.getByLabel("Name").fill("Owner");
	await page.getByLabel("Password", { exact: true }).fill("test-instance-password");
	await page.getByLabel("Authentication or recovery code").fill(totp());
	await page.getByRole("button", { name: "Sign in", exact: true }).click();
	if (await page.getByRole("link", { name: "Not now" }).isVisible()) await page.getByRole("link", { name: "Not now" }).click();
	await expect(page).toHaveURL("/");

	for (const viewport of [
		{ width: 1440, height: 900 },
		{ width: 1024, height: 768 },
		{ width: 720, height: 450 },
		{ width: 390, height: 844 },
		{ width: 320, height: 700 },
	]) {
		await page.setViewportSize(viewport);
		await page.goto("/settings#access");
		const token = page.getByLabel("Provider token", { exact: true });
		await expect(token).toBeVisible();
		const control = page.locator(".password-control").filter({ has: token });
		const geometry = await control.evaluate((element) => {
			const input = element.querySelector("input")!.getBoundingClientRect();
			const button = element.querySelector("button")!.getBoundingClientRect();
			return { buttonWidth: button.width, inputRight: input.right, buttonRight: button.right };
		});
		expect(geometry.buttonWidth).toBeGreaterThanOrEqual(40);
		expect(Math.abs(geometry.inputRight - geometry.buttonRight)).toBeLessThanOrEqual(1);
		expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBeTruthy();
		await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-settings-access.png`), fullPage: true });
	}
	await page.setViewportSize({ width: 390, height: 844 });
	await page.goto("/settings");
	await page.getByRole("link", { name: "Appearance & language", exact: true }).click();
	await page.getByRole("group", { name: "Theme" }).locator('input[value="dark"]').check();
	await page.getByRole("link", { name: "Advanced", exact: true }).click();
	await page.getByRole("link", { name: "Connections", exact: true }).click();
	const darkToken = page.getByLabel("Provider token", { exact: true });
	await expect(darkToken).toBeVisible();
	await darkToken.fill("test-duckdns-token");
	await page.locator(".password-control").filter({ has: darkToken }).getByRole("button", { name: "Show secret" }).click();
	await expect(darkToken).toHaveAttribute("type", "text");
	await expect(darkToken).toHaveValue("test-duckdns-token");
	await page.screenshot({ path: testInfo.outputPath("390-dark-settings-access.png"), fullPage: true });

	await page.emulateMedia({ forcedColors: "active", reducedMotion: "reduce" });
	await page.reload();
	await page.getByRole("link", { name: "Advanced", exact: true }).click();
	await page.getByRole("link", { name: "Connections", exact: true }).click();
	const forcedToken = page.getByLabel("Provider token", { exact: true });
	const forcedReveal = page.locator(".password-control").filter({ has: forcedToken }).getByRole("button", { name: "Show secret" });
	await forcedToken.focus();
	await page.keyboard.press(browserName === "webkit" && process.platform === "darwin" ? "Alt+Tab" : "Tab");
	await expect(forcedReveal).toBeFocused();
	expect(await forcedReveal.evaluate((button) => getComputedStyle(button).outlineStyle)).not.toBe("none");
	expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
	await page.screenshot({ path: testInfo.outputPath("forced-colors-settings-access.png"), fullPage: true });
});
