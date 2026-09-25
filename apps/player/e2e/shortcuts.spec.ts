import { createHmac } from "node:crypto";
import { expect, test, type Page } from "@playwright/test";

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
	await page.getByLabel("Authentication or recovery code").fill(totp());
	await page.getByRole("button", { name: "Sign in", exact: true }).click();
	if (await page.getByRole("link", { name: "Not now" }).isVisible()) await page.getByRole("link", { name: "Not now" }).click();
	await expect(page).toHaveURL("/");
}

async function startGoChordFromStaleMenuInput(page: Page) {
	await page.evaluate(() => {
		const input = document.querySelector("[data-command-menu] [data-command-search]");
		if (!input || input.closest("dialog")?.open) throw new Error("command menu input is not inside a closed dialog");
		input.dispatchEvent(new KeyboardEvent("keydown", { key: "g", bubbles: true }));
	});
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
	await expect(commandMenu).toBeHidden();

	await page.keyboard.press("g");
	await page.keyboard.press("m");
	await expect(page).toHaveURL(/\/?view=movies$/);

	await page.goto("/settings/configuration");
	await page.keyboard.press("Control+K");
	const pageMenu = page.getByRole("dialog", { name: "Keyboard shortcuts" });
	await expect(pageMenu).toBeVisible();
	await expect(pageMenu.getByRole("link", { name: /Settings/ })).toBeVisible();
	await page.keyboard.press("Escape");
	await expect(pageMenu).toBeHidden();
	await page.keyboard.press("g");
	await page.keyboard.press("t");
	await expect(page).toHaveURL(/\/settings(?:#.*)?$/);
});

test("closed command inputs and active editable fields handle go chords safely", async ({ page }) => {
	test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
	await login(page);
	await page.keyboard.press("?");
	const commandMenu = page.getByRole("dialog", { name: "Browse library" });
	await commandMenu.locator("[data-command-search]").focus();
	await page.keyboard.press("Escape");
	await expect(commandMenu).toBeHidden();
	await startGoChordFromStaleMenuInput(page);
	await page.keyboard.press("m");
	await expect(page).toHaveURL(/\/?view=movies$/);

	await page.goto("/");
	const librarySearch = page.getByRole("searchbox", { name: "Search all libraries" });
	await librarySearch.focus();
	await page.keyboard.type("gm");
	await expect(librarySearch).toHaveValue("gm");
	await page.waitForTimeout(1_100);
	await expect(page).toHaveURL((url) => url.pathname === "/" && url.searchParams.get("q") === "gm" && url.searchParams.get("view") !== "movies");
});
