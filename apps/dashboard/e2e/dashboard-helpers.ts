import { expect, type Page, type TestInfo } from "@playwright/test";
import { uiElementAttachment, uiElementInventory } from "../../../scripts/testing/ui-element-inventory";

export const owner = "Dashboard Owner",
	password = "correct horse battery staple";

export async function login(page: Page, passwordValue = password, dismissOffer = true) {
	await page.goto("/login");
	await page.getByLabel("Name", { exact: true }).fill(owner);
	await page.getByLabel("Password", { exact: true }).fill(passwordValue);
	await page.getByRole("button", { name: "Sign in", exact: true }).click();
	await expect(page).toHaveURL(/\?passkey=offer$/);
	await expect(page.getByRole("dialog", { name: "Make the next sign-in easier" })).toBeVisible();
	if (dismissOffer) await page.getByRole("button", { name: "Not now" }).click();
	if (dismissOffer) await expect(page).toHaveURL(/\/$/);
	await expect(page.getByRole("heading", { name: "Home" })).toBeVisible();
}

export async function addFixtureApp(page: Page, name: string, port: number, accent: string) {
	const boardResponse = await page.request.get("/api/v1/board");
	const board = await boardResponse.json();
	const meResponse = await page.request.get("/api/v1/me");
	const me = await meResponse.json();
	const response = await page.request.post("/api/v1/apps", {
		headers: { "X-Kinosail-CSRF": me.csrf },
		data: {
			name,
			url: `http://127.0.0.1:${port}`,
			category: "Household service",
			description: `${name} on the household network`,
			icon: "app",
			accent,
			checkEnabled: false,
			expectedVersion: board.version,
		},
	});
	expect(response.ok(), await response.text()).toBeTruthy();
}

export async function resetBoard(page: Page) {
	const board = await (await page.request.get("/api/v1/board")).json();
	const me = await (await page.request.get("/api/v1/me")).json();
	const response = await page.request.put("/api/v1/import", {
		headers: { "X-Kinosail-CSRF": me.csrf },
		data: { title: "Home", apps: [], expectedVersion: board.version },
	});
	expect(response.ok(), await response.text()).toBeTruthy();
	await page.reload();
}

export async function expectNoOverflow(page: Page) {
	const result = await page.evaluate(() => {
		const viewportWidth = document.documentElement.clientWidth;
		const outside = [...document.querySelectorAll("body *")]
			.filter((element) => {
				if (element.closest(".filter-row")) return false;
				const style = getComputedStyle(element);
				if (style.display === "none" || style.position === "fixed") return false;
				const box = element.getBoundingClientRect();
				return box.width > 0 && (box.left < -1 || box.right > viewportWidth + 1);
			})
			.slice(0, 10)
			.map((element) => `${element.tagName.toLowerCase()}#${element.id}.${element.className}`);
		return {
			overflow: document.documentElement.scrollWidth - viewportWidth,
			outside,
		};
	});
	expect(result).toEqual({ overflow: 0, outside: [] });
}

export async function screenshot(page: Page, testInfo: TestInfo, name: string) {
	await testInfo.attach(`${name}-elements.json`, await uiElementAttachment(testInfo.outputPath(`${name}-elements.json`), await page.evaluate(uiElementInventory)));
	await page.screenshot({
		path: testInfo.outputPath(name),
		fullPage: true,
		animations: "disabled",
	});
}
