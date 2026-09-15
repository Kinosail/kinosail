import { createHmac } from "node:crypto";
import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";

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

async function login(page: Page) {
	await page.goto("/login");
	await page.getByLabel("Name").fill("Owner");
	await page.getByLabel("Password", { exact: true }).fill("test-instance-password");
	await page.getByLabel("6-digit code").fill(totp());
	await page.getByRole("button", { name: "Sign in", exact: true }).click();
	if (await page.getByRole("link", { name: "Not now" }).isVisible()) await page.getByRole("link", { name: "Not now" }).click();
	await expect(page).toHaveURL("/");
}

test("Jellyfin setup stays blocked until trusted HTTPS is saved", async ({ page }, testInfo) => {
	test.setTimeout(120_000);
	await login(page);
	await page.goto("/onboarding/connection");
	const jellyfin = page.locator("#jellyfin");
	const choice = jellyfin.getByLabel("Allow compatible Jellyfin apps to connect");
	await expect(choice).toBeDisabled();
	await expect(jellyfin.getByText("Many Jellyfin apps reject private certificates", { exact: false })).toBeVisible();

	for (const viewport of [{ width: 1440, height: 900 }, { width: 1024, height: 768 }, { width: 390, height: 844 }, { width: 320, height: 700 }]) {
		await page.setViewportSize(viewport);
		await expect(choice).toBeDisabled();
		expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBeTruthy();
		await page.evaluate(() => (document.activeElement as HTMLElement)?.blur());
		await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-jellyfin-blocked.png`), fullPage: true });
	}
	expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);

	const trusted = page.locator('form[action="/onboarding/trusted-https"]');
	await trusted.getByRole("group", { name: "DNS provider" }).locator('input[value="duckdns"]').check();
	await trusted.getByLabel("Trusted hostname").fill("kinosail-e2e");
	await trusted.getByLabel("Provider token").fill("t".repeat(32));
	await trusted.getByLabel("Kinosail LAN address").fill("192.168.1.10");
	await trusted.getByLabel("Allow DNS validation and accept the Let's Encrypt subscriber agreement").check();
	await trusted.getByRole("button", { name: "Save trusted HTTPS" }).click();
	await expect(page).toHaveURL("/onboarding/connection");
	await expect(choice).toBeEnabled();
	await expect(jellyfin.getByText("You can now enable Jellyfin apps, then restart Kinosail once.")).toBeVisible();
	await choice.check();
	await choice.press("Tab");
	await expect(jellyfin.getByRole("button", { name: "Save Jellyfin choice" })).toBeFocused();
	await jellyfin.getByRole("button", { name: "Save Jellyfin choice" }).click();
	await expect(page).toHaveURL("/onboarding/connection#jellyfin");
	await expect(jellyfin.getByText("Connect address:")).toContainText("https://kinosail-e2e.duckdns.org:38128");
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
