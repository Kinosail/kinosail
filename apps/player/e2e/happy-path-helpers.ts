import { fixtureTotp } from "../../../scripts/testing/fixture-totp";
import AxeBuilder from "@axe-core/playwright";
import { expect, type Page } from "@playwright/test";
import type {captureHappyPathErrors} from "./happy-path-errors";

export type HappyPathState = ReturnType<typeof captureHappyPathErrors> & {
  passkeyCreated: boolean;
};

export function totp(secret: string): string {
	return fixtureTotp(secret);
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
