import { createHmac } from "node:crypto";
import { expect, test, type Page } from "@playwright/test";
import { finishRootSignIn } from "./test-instance-helpers";

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

async function login(page: Page) {
	await page.goto("/login");
	await page.getByLabel("Name").fill("Owner");
	await page.getByLabel("Password", { exact: true }).fill("test-instance-password");
	await page.getByLabel("Authentication or recovery code").fill(totp());
	const origin = new URL(page.url()).origin;
	await page.getByRole("button", { name: "Sign in", exact: true }).click();
	await finishRootSignIn(page, origin);
	await expect(page).toHaveURL("/");
}

const routedTest = test.extend({serviceWorkers: "block"});
routedTest("repeating the active Movies link does not reload the document", async ({ page }, testInfo) => {
	const origin = new URL(test.info().project.use.baseURL!).origin;
	await login(page);
	let offerObserved = false;
	const facts = {handlerEntered: false, fetchStarted: false, fetchReturned: false, fetchFailed: false,
		responseStatus: null as number | null, ownedResponse: false, delayFinished: false,
		fulfillStarted: false, fulfillFinished: false, fulfillFailed: false};
	const target = new URL("/account?passkey=offer&next=%2F", origin);
	const offer = (url: URL) => url.href === target.href;
	await page.route(offer, async route => {
		facts.handlerEntered = true;
		facts.fetchStarted = true;
		let response;
		try {response = await route.fetch({ maxRedirects: 0 }); facts.fetchReturned = true;}
		catch (error) {facts.fetchFailed = true; throw error;}
		const status = response.status();
		facts.responseStatus = Number.isInteger(status) && status >= 100 && status <= 599 ? status : null;
		facts.ownedResponse = response.url() === target.href;
		if (status !== 200 || !facts.ownedResponse) {
			await route.abort();
			throw new Error("the owned passkey offer response was not accepted");
		}
		await new Promise(resolve => setTimeout(resolve, 200));
		facts.delayFinished = true;
		facts.fulfillStarted = true;
		try {await route.fulfill({ response }); facts.fulfillFinished = true;}
		catch (error) {facts.fulfillFailed = true; throw error;}
		offerObserved = true;
	});
	try {
		await page.goto(target.href, { waitUntil: "commit" });
		await finishRootSignIn(page, origin);
		expect(offerObserved, "the real authenticated offer response must exercise delayed completion").toBe(true);
	} finally {
		// Snapshot before cleanup so it cannot change the original failure stage.
		let timer: ReturnType<typeof setTimeout> | undefined;
		try {await Promise.race([testInfo.attach("owned-offer-delay-stages", {contentType: "application/json", body: JSON.stringify({...facts})}),
			new Promise(resolve => {timer = setTimeout(resolve, 500);})]);}
		catch { /* Observation failure cannot replace the original assertion. */ }
		finally {clearTimeout(timer); await page.unroute(offer);}
	}
	await page.goto("/?view=movies");
	const movies = page.getByRole("navigation", { name: "Main navigation" }).getByRole("link", { name: "Movies", exact: true });
	await expect(movies).toHaveAttribute("aria-current", "page");

	const documentRequests: string[] = [];
	page.on("request", (request) => {
		if (request.isNavigationRequest() && request.resourceType() === "document") documentRequests.push(new URL(request.url()).pathname + new URL(request.url()).search);
	});
	await page.evaluate(() => {
		const link = [...document.querySelectorAll('nav[aria-label="Main navigation"] a')].find((candidate) => candidate.textContent?.trim() === "Movies");
		if (!link) throw new Error("Movies navigation link is missing");
		for (let index = 0; index < 6; index++) link.click();
	});
	await page.waitForTimeout(500);

	expect(documentRequests, "repeating the active link must not start document navigations").toEqual([]);
	expect(page.url()).toContain("/?view=movies");
});

test("refresh keeps compact navigation and sign out reachable", async ({ page }) => {
	await login(page);
	await page.setViewportSize({ width: 390, height: 844 });
	await page.goto("/?view=list");

	for (let refresh = 0; refresh < 3; refresh++) {
		if (refresh > 0) await page.reload();
		const navigation = page.getByRole("navigation", { name: "Main navigation" });
		await expect(navigation).toBeVisible();
		await expect.poll(() => navigation.evaluate((element) => {
			const bounds = element.getBoundingClientRect();
			return Math.round(bounds.bottom) >= window.innerHeight - 1;
		})).toBe(true);

		const more = navigation.locator("details.nav-more");
		await more.locator("summary").click();
		await expect(more.getByRole("button", { name: "Sign out", exact: true })).toBeVisible();
		await more.locator("summary").click();
	}
});

test("double-clicking a compact menu shortcut starts one navigation", async ({ page }) => {
	await login(page);
	await page.setViewportSize({ width: 390, height: 844 });
	await page.goto("/?view=list");

	const actions = page.locator("details.nav-more");
	await actions.locator("summary").click();
	const shortcut = actions.locator(".nav-more-menu a[href^='/?view=']:not([href='/?view=list']):visible").first();
	await expect(shortcut).toBeVisible();
	const targetHref = await shortcut.getAttribute("href");
	if (!targetHref) throw new Error("Library shortcut target is missing");

	const documentRequests: string[] = [];
	page.on("request", (request) => {
		if (request.isNavigationRequest() && request.resourceType() === "document") documentRequests.push(new URL(request.url()).pathname + new URL(request.url()).search);
	});
	await page.evaluate(() => {
		const link = [...document.querySelectorAll(".nav-more-menu a[href^='/?view=']:not([href='/?view=list'])")].find(link => link.getClientRects().length && getComputedStyle(link).display !== "none");
		if (!(link instanceof HTMLAnchorElement)) throw new Error("Library shortcut is missing");
		link.click();
		link.click();
	});
	await expect(page).toHaveURL(new RegExp(`${targetHref.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}$`));

	expect(documentRequests, "a repeated shortcut must not start duplicate document navigations").toEqual([targetHref]);
	await expect(page.getByRole("navigation", { name: "Main navigation" })).toBeVisible();
});
