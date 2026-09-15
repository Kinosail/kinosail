import { expect, test, type Page, type TestInfo } from "@playwright/test";
import { uiElementAttachment, uiElementInventory } from "../../../scripts/testing/ui-element-inventory";
import { createHmac } from "node:crypto";

export const viewports = [
	{ width: 1440, height: 900 },
	{ width: 1024, height: 768 },
	{ width: 900, height: 768 },
	{ width: 720, height: 450 },
	{ width: 390, height: 844 },
	{ width: 320, height: 800 },
];

export function configureLayoutAudit() {
	test.beforeEach(async ({ page }) => page.addInitScript(() => Object.defineProperty(PublicKeyCredential, "isConditionalMediationAvailable", { value: async () => false })));
}

export function totp(): string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567";
	const bits = [...(process.env.KINOSAIL_TEST_TOTP_SECRET ?? "")].map((character) => alphabet.indexOf(character).toString(2).padStart(5, "0")).join("");
	const secret = Buffer.from(bits.match(/.{8}/g)?.map((byte) => Number.parseInt(byte, 2)) ?? []);
	const counter = Buffer.alloc(8);
	counter.writeBigUInt64BE(BigInt(Math.floor(Date.now() / 30_000)));
	const digest = createHmac("sha1", secret).update(counter).digest();
	const offset = digest[19] & 15;
	return ((digest.readUInt32BE(offset) & 0x7fffffff) % 1_000_000).toString().padStart(6, "0");
}

export async function login(page: Page, name = "Owner", password = "test-instance-password") {
	await page.goto("/login");
	await page.getByLabel("Name").fill(name);
	await page.getByLabel("Password", { exact: true }).fill(password);
	const code = page.getByLabel("Authentication or recovery code");
	if (await code.count()) await code.fill(totp());
	await page.getByRole("button", { name: "Sign in", exact: true }).click();
	if (await page.getByRole("link", { name: "Not now" }).isVisible()) await page.getByRole("link", { name: "Not now" }).click();
}

export async function layoutProblems(page: Page) {
	return page.evaluate(() => {
		const viewportWidth = document.documentElement.clientWidth;
		const visible = (element: Element) => {
			if (element.closest("details:not([open])") && !element.matches("details > summary")) return false;
			const style = getComputedStyle(element);
			const box = element.getBoundingClientRect();
			return style.visibility !== "hidden" && style.display !== "none" && box.width > 0 && box.height > 0;
		};
		const label = (element: Element) => element.getAttribute("aria-label") || element.textContent?.trim().replace(/\s+/g, " ").slice(0, 60) || element.tagName.toLowerCase();
		const outside = [...document.querySelectorAll("main *, body > header *, body > nav *")]
			.filter((element) => visible(element) && !element.closest(".rail, .settings-nav, .wizard-steps, .app-header nav"))
			.map((element) => ({ element, box: element.getBoundingClientRect() }))
			.filter(({ box }) => box.left < -1 || box.right > viewportWidth + 1)
			.map(({ element, box }) => `${label(element)} [${Math.round(box.left)}, ${Math.round(box.right)}]`);
		const tinyControls = [...document.querySelectorAll("button, input:not([type=hidden]):not([type=checkbox]):not([type=radio]), select, textarea, .back")]
			.filter(visible)
			.map((element) => ({ element, box: element.getBoundingClientRect() }))
			.filter(({ box }) => box.width < 24 || box.height < 24)
			.map(({ element, box }) => `${label(element)} [${Math.round(box.width)}x${Math.round(box.height)}]`);
		const distortedChecks = [...document.querySelectorAll('input[type="checkbox"], input[type="radio"]')]
			.filter(visible)
			.map((element) => ({ element, box: element.getBoundingClientRect() }))
			.filter(({ element, box }) => {
				if (element.parentElement?.classList.contains("choice-option")) {
					const target = element.parentElement.getBoundingClientRect();
					return box.width < 44 || box.height < 44 || Math.abs(box.width - target.width) > 2 || Math.abs(box.height - target.height) > 2;
				}
				return Math.abs(box.width - box.height) > 2 || box.width > 24;
			})
			.map(({ element, box }) => `${label(element)} [${Math.round(box.width)}x${Math.round(box.height)}]`);
		const clippedControls = [...document.querySelectorAll("button, .button, .mode, .header-link")]
			.filter(visible)
			.filter((element) => element.scrollWidth > element.clientWidth + 1 || element.scrollHeight > element.clientHeight + 1)
			.map((element) => `${label(element)} [${element.clientWidth}x${element.clientHeight} of ${element.scrollWidth}x${element.scrollHeight}]`);
		const overlappingStatuses = [...document.querySelectorAll(".settings-flow strong.status")]
			.filter(visible)
			.filter((element) => {
				const next = element.parentElement?.nextElementSibling;
				if (!next || !visible(next)) return false;
				const status = element.getBoundingClientRect();
				const sibling = next.getBoundingClientRect();
				return status.bottom > sibling.top && status.top < sibling.bottom && status.right > sibling.left && status.left < sibling.right;
			})
			.map(label);
		return {
			documentOverflow: document.documentElement.scrollWidth - viewportWidth,
			outside: outside.slice(0, 10),
			tinyControls: tinyControls.slice(0, 10),
			distortedChecks: distortedChecks.slice(0, 10),
			clippedControls: clippedControls.slice(0, 10),
			overlappingStatuses: overlappingStatuses.slice(0, 10),
		};
	});
}

export async function attachUIInventory(page: Page, testInfo: TestInfo, name: string) {
	await testInfo.attach(`${name}-elements.json`, await uiElementAttachment(testInfo.outputPath(`${name}-elements.json`), await page.evaluate(uiElementInventory)));
}

export async function applicationShellProblems(page: Page) {
	return page.evaluate(() => {
		const header = document.querySelector(".app-header")!;
		const bounds = header.getBoundingClientRect();
		const visible = (element: Element) => {
			const box = element.getBoundingClientRect();
			const style = getComputedStyle(element);
			return style.display !== "none" && style.visibility !== "hidden" && box.width > 0 && box.height > 0;
		};
		const outside = [...header.querySelectorAll(":scope > .brand-lockup, :scope > .search, :scope > nav, :scope > .header-actions, :scope > nav > a")]
			.filter(visible)
			.filter((element) => {
				const box = element.getBoundingClientRect();
				return box.left < bounds.left - 1 || box.right > bounds.right + 1;
			})
			.map((element) => element.getAttribute("aria-label") || element.textContent?.trim().replace(/\s+/g, " ").slice(0, 40) || element.tagName);
		return {
			display: getComputedStyle(header).display,
			horizontalOverflow: header.scrollWidth - header.clientWidth,
			outside,
		};
	});
}

export async function searchPlaceholderFits(page: Page, label: string) {
	return page.getByRole("searchbox", { name: label }).evaluate((element) => {
		const input = element as HTMLInputElement;
		const style = getComputedStyle(input);
		const canvas = document.createElement("canvas");
		const context = canvas.getContext("2d")!;
		context.font = style.font;
		// Firefox search inputs report clientWidth without padding; measure the border box consistently.
		const available = input.getBoundingClientRect().width - Number.parseFloat(style.borderLeftWidth) - Number.parseFloat(style.borderRightWidth) - Number.parseFloat(style.paddingLeft) - Number.parseFloat(style.paddingRight);
		return {
			available,
			required: context.measureText(input.placeholder).width,
		};
	});
}

export async function expectSearchControl(page: Page, viewport: number, label = "Search library") {
	const search = page.getByRole("searchbox", { name: label });
	await expect(search).toHaveAttribute("placeholder", label);
	await expect.poll(async () => {
		const fit = await searchPlaceholderFits(page, label);
		return fit.required - fit.available;
	}, { message: `${viewport}px search placeholder width` }).toBeLessThanOrEqual(0);
	await search.focus();
	await search.fill("Arrival");
	await expect(search).toHaveValue("Arrival");
	expect(await search.evaluate((element) => element === document.activeElement), `${viewport}px search focus`).toBeTruthy();
	await search.fill("");
}

export async function presentationProblems(page: Page) {
	return page.evaluate(() => {
		const visible = (element: Element) => {
			const style = getComputedStyle(element);
			const box = element.getBoundingClientRect();
			return style.visibility !== "hidden" && style.display !== "none" && box.width > 0 && box.height > 0;
		};
		const label = (element: Element) => element.id || element.className || element.tagName.toLowerCase();
		const elements = (selector: string) => [...document.querySelectorAll(selector)].filter(visible);
		const ordinarySurfaces = ".settings-flow>section,.settings-shell>section,.destination-card,.episode,.download-job,.connection-test,.primary-player-actions,.playback-tools>.player-actions";
		const blurred = elements(ordinarySurfaces)
			.filter((element) => {
				const style = getComputedStyle(element);
				return [style.backdropFilter, style.getPropertyValue("-webkit-backdrop-filter")].some((value) => value && value !== "none");
			})
			.map(label);
		const shadowed = elements(ordinarySurfaces)
			.filter((element) => getComputedStyle(element).boxShadow !== "none")
			.map(label);
		const overRounded = elements(".settings-flow>section,.settings-shell>section,.primary-player-actions button,.primary-player-actions .mode")
			.filter((element) => Number.parseFloat(getComputedStyle(element).borderTopLeftRadius) > 14)
			.map(label);
		const intro = document.querySelector(".settings-intro");
		const heading = document.querySelector(".settings-intro h1");
		return {
			blurred: blurred.slice(0, 10),
			shadowed: shadowed.slice(0, 10),
			overRounded: overRounded.slice(0, 10),
			settingsBackdrop: intro && getComputedStyle(intro, "::after").display !== "none",
			oversizedSettingsHeading: heading ? Number.parseFloat(getComputedStyle(heading).fontSize) > 64 : false,
		};
	});
}
