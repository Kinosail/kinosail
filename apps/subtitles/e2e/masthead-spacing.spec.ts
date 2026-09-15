import { expect, test } from "@playwright/test";
import { createHmac } from "node:crypto";

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

async function login(page: import("@playwright/test").Page) {
	await page.goto("/login");
	await page.getByLabel("Name").fill("Owner");
	await page.getByLabel("Password", { exact: true }).fill("test-instance-password");
	await page.getByLabel("6-digit code").fill(totp());
	await page.getByRole("button", { name: "Sign in", exact: true }).click();
	if (await page.getByRole("link", { name: "Not now" }).isVisible()) await page.getByRole("link", { name: "Not now" }).click();
}

test("home masthead keeps the first shelf in the useful viewport", async ({ page }, testInfo) => {
	await login(page);
	for (const viewport of [{ width: 1440, height: 900 }, { width: 1024, height: 768 }, { width: 720, height: 450 }, { width: 390, height: 844 }, { width: 320, height: 800 }]) {
		await page.setViewportSize(viewport);
		await page.goto("/", { waitUntil: "domcontentloaded" });
		await page.screenshot({ path: testInfo.outputPath(`home-${viewport.width}.png`), fullPage: false });
		const composition = await page.evaluate(() => {
			const masthead = document.querySelector(".library-masthead:not(.browse-masthead)")!.getBoundingClientRect();
			const firstShelf = document.querySelector(".home-shelf, .destination-browser")!.getBoundingClientRect();
			return { masthead: masthead.height, firstShelfTop: firstShelf.top };
		});
		expect(composition.masthead, `Home masthead at ${viewport.width}px`).toBeLessThan(viewport.width <= 700 ? 220 : 320);
		expect(composition.firstShelfTop, `Home content at ${viewport.width}px`).toBeLessThan(viewport.height * 0.8);
	}
});
