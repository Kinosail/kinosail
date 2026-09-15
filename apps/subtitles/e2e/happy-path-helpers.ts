import AxeBuilder from "@axe-core/playwright";
import { createHmac } from "node:crypto";
import { expect, type Page } from "@playwright/test";

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

export async function openConnect(page: Page) {
	const connect = page.getByRole("link", { name: "Connect", exact: true });
	if (!await connect.isVisible()) await page.locator(".nav-more > summary").click();
	await connect.click();
}

export async function finishOfflineInstall(page: Page, projectName: string, errors: string[]) {
	await page.locator(".nav-more > summary").click();
	await page.evaluate(() => {
		const event = new Event("beforeinstallprompt");
		Object.assign(event, { prompt: async () => {}, userChoice: Promise.resolve({ outcome: "accepted" }) });
		window.dispatchEvent(event);
	});
	const install = page.getByRole("button", { name: "Install" });
	await expect(install).toBeVisible();
	await install.click();
	await expect(install).toBeHidden();
	await page.evaluate(() => navigator.serviceWorker.ready);
	if (!await page.evaluate(() => Boolean(navigator.serviceWorker.controller))) await page.reload();
	const cached = await page.evaluate(async () => (await Promise.all((await caches.keys()).map(async (name) => (await caches.open(name)).keys()))).flat().map((request) => new URL(request.url).pathname));
	expect(cached.some((path) => path.startsWith("/api/") || path.startsWith("/media/") || path.startsWith("/hls/"))).toBe(false);
	if (projectName === "chromium") {
		await page.context().setOffline(true);
		try {
			await page.goto("/?offline-check=1");
			await expect(page.getByRole("heading", { name: "Server unavailable" })).toBeVisible();
		} finally {
			await page.context().setOffline(false);
		}
	}
	expect(errors.filter((error) => !error.includes("blob:http://") && !(projectName === "firefox" && error.includes("NS_BINDING_ABORTED") && error.includes("WorkerMain.js")))).toEqual([]);
}
