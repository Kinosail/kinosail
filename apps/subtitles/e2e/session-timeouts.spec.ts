import { expect, test } from "@playwright/test";
import { createHmac } from "node:crypto";
import AxeBuilder from "@axe-core/playwright";

test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated instance authentication fixture");

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

test("Owner can understand and customize automatic sign-out", async ({ page }, testInfo) => {
	let loginLoaded = false;
	for (let attempt = 0; attempt < 3 && !loginLoaded; attempt++) {
		try {
			await page.goto("/login", { waitUntil: "domcontentloaded", timeout: 15_000 });
			loginLoaded = true;
		} catch (error) {
			if (attempt === 2) throw error;
			await page.waitForTimeout(250);
		}
	}
	await page.getByLabel("Name").fill("Owner");
	const password = process.env.KINOSAIL_TEST_INSTANCE === "1" ? "test-instance-password" : "test-password";
	await page.getByLabel("Password").fill(password);
	await page.getByLabel("6-digit code").fill(totp());
	await page.getByRole("button", { name: "Sign in", exact: true }).click();
	if (await page.getByRole("link", { name: "Not now" }).isVisible()) await page.getByRole("link", { name: "Not now" }).click();

	for (const viewport of [{ width: 1440, height: 900 }, { width: 1024, height: 768 }, { width: 390, height: 844 }, { width: 320, height: 800 }]) {
		await page.setViewportSize(viewport);
		await page.goto("/settings", { waitUntil: "domcontentloaded" });
		const section = page.locator("#session-timeouts");
		await expect(section).toBeVisible();
		await expect(section.getByLabel("After inactivity")).toHaveValue("0.25");
		await expect(section.getByLabel("Always after")).toHaveValue("8");
		await expect(section.getByLabel("After inactivity").locator("option")).toHaveText(["15 minutes (recommended)", "1 hour", "8 hours", "1 day", "3 days", "7 days", "30 days", "90 days", "1 year"]);
		await expect(section.getByLabel("Always after").locator("option")).toHaveText(["4 hours", "8 hours (recommended)", "1 day", "7 days", "30 days", "90 days", "1 year"]);
		expect((await new AxeBuilder({ page }).include("#session-timeouts").analyze()).violations).toEqual([]);
		expect(await section.evaluate((element) => ({ fitsViewport: element.getBoundingClientRect().right <= document.documentElement.clientWidth + 1, noOverflow: element.scrollWidth <= element.clientWidth + 1 }))).toEqual({ fitsViewport: true, noOverflow: true });
		if (viewport.width === 1440 || viewport.width === 390) await page.screenshot({ path: testInfo.outputPath(`session-timeouts-${viewport.width}.png`), fullPage: true });
	}
});
