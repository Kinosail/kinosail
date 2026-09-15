import AxeBuilder from "@axe-core/playwright";
import { createHmac } from "node:crypto";
import { expect, type Page } from "@playwright/test";

export type HappyPathState = {
  capture: (enabled: boolean) => void;
  errors: string[];
  passkeyCreated: boolean;
};

export function totp(secret: string) {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567";
	let bits = "";
	for (const character of secret) bits += alphabet.indexOf(character).toString(2).padStart(5, "0");
	const key = Buffer.from(bits.match(/.{8}/g)!.map((byte) => Number.parseInt(byte, 2)));
	const counter = Buffer.alloc(8);
	counter.writeBigUInt64BE(BigInt(Math.floor(Date.now() / 30_000)));
	const digest = createHmac("sha1", key).update(counter).digest();
	const offset = digest.at(-1)! & 15;
	return ((digest.readUInt32BE(offset) & 0x7fffffff) % 1_000_000).toString().padStart(6, "0");
}

export async function expectAccessible(page: Page, capture: (enabled: boolean) => void) {
	capture(false);
	try {
		expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
	} finally {
		await page.waitForTimeout(50);
		capture(true);
	}
}

export async function openSettings(page: Page) {
	const settings = page.getByRole("link", { name: "Settings" });
	if (!await settings.isVisible()) await page.locator(".nav-more > summary").click();
	await settings.click();
}

export async function signOut(page: Page) {
	const button = page.getByRole("button", { name: "Sign out" });
	if (!await button.isVisible()) await page.locator(".nav-more > summary").click();
	await button.click();
}

export async function openQuickConnect(page: Page) {
	const quickConnect = page.getByRole("link", { name: "Quick Connect", exact: true });
	if (!await quickConnect.isVisible()) await page.locator(".nav-more > summary").click();
	await quickConnect.click();
}
