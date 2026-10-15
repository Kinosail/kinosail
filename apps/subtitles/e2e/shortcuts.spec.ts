import { fixtureTotp } from "../../../scripts/testing/fixture-totp";
import { expect, test, type Page } from "@playwright/test";

function totp(): string {
	return fixtureTotp();
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

test("Linear-style navigation shortcuts work on browse and settings pages", async ({ page }) => {
	test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
	await login(page);

	await page.keyboard.press("?");
	const commandMenu = page.getByRole("dialog", { name: "Browse library" });
	await expect(commandMenu).toBeVisible();
	await expect(commandMenu.getByRole("link", { name: /Movies/ })).toBeVisible();
	await expect(commandMenu.getByRole("button", { name: /Open command menu/ })).toBeVisible();
	await page.keyboard.press("Escape");

	await page.keyboard.press("g");
	await page.keyboard.press("m");
	await expect(page).toHaveURL(/\/?view=movies$/);

	await page.goto("/settings/configuration");
	await page.keyboard.press("Control+K");
	const pageMenu = page.getByRole("dialog", { name: "Keyboard shortcuts" });
	await expect(pageMenu).toBeVisible();
	await expect(pageMenu.getByRole("link", { name: /Settings/ })).toBeVisible();
	await page.keyboard.press("Escape");
	await page.keyboard.press("g");
	await page.keyboard.press("t");
	await expect(page).toHaveURL(/\/settings(?:#.*)?$/);
});
