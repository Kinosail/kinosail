import { createHmac } from "node:crypto";
import { expect, test } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";

test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");

test("Owner can switch Player checkout cadence without losing levels or accessibility", async ({ page }) => {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567";
	const bits = [...(process.env.KINOSAIL_TEST_TOTP_SECRET ?? "")].map((character) => alphabet.indexOf(character).toString(2).padStart(5, "0")).join("");
	const secret = Buffer.from(bits.match(/.{8}/g)?.map((byte) => Number.parseInt(byte, 2)) ?? []);
	const counter = Buffer.alloc(8);
	counter.writeBigUInt64BE(BigInt(Math.floor(Date.now() / 30_000)));
	const digest = createHmac("sha1", secret).update(counter).digest();
	const offset = digest[19] & 15;
	const code = ((digest.readUInt32BE(offset) & 0x7fffffff) % 1_000_000).toString().padStart(6, "0");
	await page.goto("/login");
	await page.getByLabel("Name").fill("Owner");
	await page.getByLabel("Password", { exact: true }).fill("test-instance-password");
	await page.getByLabel("Authentication or recovery code").fill(code);
	await page.getByRole("button", { name: "Sign in", exact: true }).click();
	if (await page.getByRole("link", { name: "Not now" }).isVisible()) await page.getByRole("link", { name: "Not now" }).click();
	await page.setViewportSize({ width: 390, height: 844 });
	await page.goto("/supporter");
	const checkout = page.locator("[data-supporter-checkout]");
	const billing = page.getByRole("group", { name: "Contribution frequency" });
	await expect(billing).toBeVisible();
	for (const [frequency, family, price, url] of [
		["monthly", "monthly", "$3/month", "https://buy.polar.sh/polar_cl_qnIK97BxBRJpKtqw0na3QocqgA3IyY0KlM6wZ36YiP8"],
		["annual", "yearly", "$12/year", "https://buy.polar.sh/polar_cl_WwzGYfc354pJm13xvL90r1qbP0bfSJo0FEDpI3xKiRn"],
		["once", "one-time", "$5 once", "https://buy.polar.sh/polar_cl_U5woq81CkwRbf0B8P9vpyo2mZkxgPD2qIkFsJ2685XU"],
	] as const) {
		await billing.locator(`input[value="${frequency}"]`).check();
		const gallery = page.locator(`[data-supporter-family="${family}"]`);
		await expect(gallery).toBeVisible();
		await expect(gallery.locator("li")).toHaveCount(10);
		await expect(gallery.locator("[data-supporter-price]").first()).toHaveText(price);
		await expect(checkout).toHaveAttribute("href", url);
	}
	expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true);
	expect((await new AxeBuilder({ page }).include(".supporter-contribute").analyze()).violations).toEqual([]);
});
