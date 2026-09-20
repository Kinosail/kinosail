import { expect, type Page, type Request, type TestInfo } from "@playwright/test";
import { holdNextLibraryPage, holdNextMainRequest } from "./request-holds";
import { expectAccessible, openSettings, signOut, totp, type HappyPathState } from "./happy-path-helpers";

export async function startHappyPath(page: Page, testInfo: TestInfo): Promise<HappyPathState> {
	const errors: string[] = [];
	let passkeyCreated = false;
	let authenticatorSecret = "";
	if (testInfo.project.name === "chromium") {
		await page.addInitScript(() => Object.defineProperty(PublicKeyCredential, "isConditionalMediationAvailable", { value: async () => false }));
		const client = await page.context().newCDPSession(page);
		await client.send("WebAuthn.enable");
		await client.send("WebAuthn.addVirtualAuthenticator", { options: { protocol: "ctap2", transport: "internal", hasResidentKey: true, hasUserVerification: true, isUserVerified: true, automaticPresenceSimulation: true } });
	}
	let captureErrors = true;
	page.on("console", (message) => {
		if (captureErrors && message.type() === "error") errors.push(message.text());
	});
	page.on("pageerror", (error) => {
		if (captureErrors) errors.push(error.message);
	});
	const capture = (enabled: boolean) => {
		captureErrors = enabled;
	};

  await page.goto("/setup");
  if (await page.getByRole("heading", { name: "Set up your Server." }).isVisible()) {
    await expectAccessible(page, capture);
    await expect(page.getByLabel("Setup code")).toHaveCount(0);
    await page.getByLabel("Name").fill("Owner");
    await page.locator("#new-password").fill("test-password");
    await page.getByLabel(/Add extra sign-in protection now/).uncheck();
    await page.getByRole("button", { name: "Create Owner & continue" }).click();
    await expect(page.getByRole("heading", { name: "Protect the Owner account" })).toBeVisible();
    await expectAccessible(page, capture);
		if (testInfo.project.name === "chromium") {
			await page.getByRole("button", { name: "Create passkey" }).click();
			await expect(page).toHaveURL("/onboarding/connection");
			await expect(page.getByRole("heading", { name: "Connect the devices you already own." })).toBeVisible();
			await expectAccessible(page, capture);
			await expect(page.getByText("Local access is ready now. Add trusted HTTPS only if a phone, TV, or app needs it; you can change these choices later in Settings.", { exact: true })).toBeVisible();
			await expect(page.getByRole("heading", { name: "Secure local access" })).toBeVisible();
			await expect(page.getByRole("heading", { name: "Jellyfin apps", exact: true })).toBeVisible();
			await expect(page.getByLabel("Allow compatible Jellyfin apps to connect")).not.toBeChecked();
			await expect(page.getByRole("heading", { name: "Trusted HTTPS for phones, TVs, and Jellyfin apps" })).toBeVisible();
			await expect(page.getByRole("heading", { name: "Remote access comes later" })).toBeVisible();
			const trustedDisclosure = page.locator("#trusted-https-configuration");
			await expect(trustedDisclosure.locator(":scope > summary")).toHaveText(/Set up trusted HTTPS/);
			await expect(page.getByRole("group", { name: "DNS provider" })).toBeVisible();
			await trustedDisclosure.locator(":scope > summary").focus();
			await page.keyboard.press("Enter");
			await expect(page.getByRole("group", { name: "DNS provider" })).toBeHidden();
			await page.keyboard.press("Enter");
			await expect(page.getByRole("group", { name: "DNS provider" }).locator('input[value="duckdns"]')).toBeChecked();
			await expect(page.getByLabel("Trusted hostname")).toBeVisible();
			await expect(page.getByLabel("Provider token")).toBeVisible();
			await expect(page.getByLabel("Kinosail LAN address")).toBeVisible();
			await expect(page.getByLabel(/Allow DNS validation/)).toBeVisible();
			await page.getByRole("link", { name: "Continue to household setup" }).click();
			await expect(page.getByRole("heading", { name: "Set up Viewer Profiles." })).toBeVisible();
			await page.getByRole("link", { name: "Continue to optional viewing history" }).click();
			await expect(page.getByRole("heading", { name: "Import your viewing history." })).toBeVisible();
			await page.getByRole("link", { name: "Finish and open Library" }).click();
			passkeyCreated = true;
		} else {
			await page.getByRole("button", { name: "Use an authenticator instead" }).click();
			authenticatorSecret = (await page.locator("code").first().textContent())!;
			await page.getByLabel("Authentication code").fill(totp(authenticatorSecret));
			await page.getByRole("button", { name: "Turn on extra sign-in protection" }).click();
			await expect(page).toHaveURL("/onboarding/connection");
			await page.getByRole("link", { name: "Continue to household setup" }).click();
			await page.getByRole("link", { name: "Continue to optional viewing history" }).click();
			await page.getByRole("link", { name: "Finish and open Library" }).click();
		}
  } else {
    await expect(page.getByRole("heading", { name: "Kinosail" })).toBeVisible();
		await expectAccessible(page, capture);
    await page.getByLabel("Name").fill("Owner");
    await page.getByLabel("Password", { exact: true }).fill("test-password");
    await page.getByRole("button", { name: "Sign in", exact: true }).click();
		if (await page.getByRole("link", { name: "Not now" }).isVisible()) await page.getByRole("link", { name: "Not now" }).click();
  }

  await expect(page.getByRole("heading", { name: "Kinosail" })).toBeVisible();
  await expect(page.getByRole("main")).toBeVisible();
  await expect(page.locator("#library .card").first()).toBeVisible();
	const infiniteRequests: string[] = [];
	const captureInfinite = (request: Request) => {
		if (request.headers()["x-kinosail-library-page"] === "1") infiniteRequests.push(new URL(request.url()).searchParams.get("offset") ?? "");
	};
	page.on("request", captureInfinite);
	await page.goto("/?view=movies&limit=1");
	await expect.poll(async () => {
		await page.evaluate(() => window.scrollTo(0, document.body.scrollHeight));
		return {
		cards: await page.locator("#library .card").count(),
		next: await page.locator("[data-library-next]").count(),
		requests: [...infiniteRequests],
		status: await page.locator("[data-library-status]").textContent(),
		};
	}, { timeout: 30_000 }).toMatchObject({ next: 0, status: "Everything is loaded." });
	const loadedLibrary = await page.evaluate(() => ({
		cards: document.querySelectorAll("#library .card").length,
		next: document.querySelectorAll("[data-library-next]").length,
		status: document.querySelector("[data-library-status]")?.textContent,
	}));
	expect(loadedLibrary.next).toBe(0);
	expect(loadedLibrary.status).toBe("Everything is loaded.");
	expect(loadedLibrary.cards).toBe(infiniteRequests.length + 1);
	expect(infiniteRequests).toEqual(Array.from({ length: infiniteRequests.length }, (_, index) => String(index + 1)));
	page.off("request", captureInfinite);

	const libraryLoad = await holdNextLibraryPage(page, "1");
	await page.goto("/?view=movies&limit=1");
	await libraryLoad.waitUntilStarted();
	await page.getByRole("searchbox", { name: "Search library" }).fill("Arrival");
	await expect(page).toHaveURL(/q=Arrival/);
	await libraryLoad.release();
	await page.waitForTimeout(300);
	await expect(page.locator("#library .card")).toHaveCount(1);
	await expect(page.getByRole("heading", { name: /Beta|Gamma/ })).toHaveCount(0);

	await page.goto("/?view=movies");
	await page.setViewportSize({ width: 1024, height: 768 });
	const desktopTitleIndex = page.locator("[data-title-jump-index]");
	await expect(desktopTitleIndex).toBeVisible();
	await desktopTitleIndex.getByRole("link", { name: "B, 1 title" }).click();
	await expect(page).toHaveURL(/letter=B/);
	await expect(page.getByRole("heading", { name: "Beta" })).toBeVisible();
	await expect(page.getByRole("heading", { name: "Arrival" })).toHaveCount(0);

	await page.goto("/?view=movies");
	await page.setViewportSize({ width: 390, height: 650 });
	const jumpButton = page.getByRole("button", { name: "Jump to title" });
	await expect(jumpButton).toBeVisible();
	await expect(page.locator("[data-title-jump-index]")).toBeHidden();
	await jumpButton.click();
	const jumpDialog = page.getByRole("dialog", { name: "Jump to title" });
	await expect(jumpDialog).toBeVisible();
	await jumpDialog.getByRole("link", { name: "B, 1 title" }).click();
	await expect(page).toHaveURL(/letter=B/);
	await expect(page.getByRole("heading", { name: "Beta" })).toBeVisible();
	await expect(page.getByRole("heading", { name: "Arrival" })).toHaveCount(0);
	await expect(page.locator("#library a.card").first()).toBeFocused();
	await jumpButton.click();
	await page.getByRole("dialog", { name: "Jump to title" }).getByRole("link", { name: "B, 1 title, selected; activate to show all titles" }).click();
	await expect(page).not.toHaveURL(/letter=/);
	await expect(page.getByRole("heading", { name: "Arrival" }).first()).toBeVisible();
	await jumpButton.click();
	await page.getByRole("dialog", { name: "Jump to title" }).getByRole("link", { name: "B, 1 title" }).click();
	await expect(page).toHaveURL(/letter=B/);
	await page.goBack();
	await expect(page).not.toHaveURL(/letter=/);
	await expect(page.getByRole("heading", { name: "Arrival" }).first()).toBeVisible();
	const jumpLoad = await holdNextMainRequest(page, "letter", "B");
	await jumpButton.click();
	await page.getByRole("dialog", { name: "Jump to title" }).getByRole("link", { name: "B, 1 title" }).click();
	await jumpLoad.waitUntilStarted();
	await page.getByRole("searchbox", { name: "Search library" }).fill("Gamma");
	await expect(page).toHaveURL(/q=Gamma/);
	await jumpLoad.release();
	await page.waitForTimeout(300);
	await expect(page.getByRole("heading", { name: "Gamma" })).toBeVisible();
	await expect(page.getByRole("heading", { name: "Beta" })).toHaveCount(0);
	await page.goto("/?view=movies");

	await page.setViewportSize({ width: 390, height: 844 });
	const titleIndex = page.locator("[data-title-jump-index]");
	await expect(titleIndex).toBeVisible();
	await expect(jumpButton).toBeHidden();
	const firstLetter = await titleIndex.getByRole("link", { name: "A, 1 title" }).boundingBox();
	const lastLetter = await titleIndex.getByRole("link", { name: "G, 1 title" }).boundingBox();
	expect(firstLetter && lastLetter).toBeTruthy();
	await page.mouse.move(firstLetter!.x + firstLetter!.width / 2, firstLetter!.y + firstLetter!.height / 2);
	await page.mouse.down();
	await page.mouse.move(lastLetter!.x + lastLetter!.width / 2, lastLetter!.y + lastLetter!.height / 2, { steps: 4 });
	await expect(page.locator("[data-title-jump-preview]")).toHaveText("G, 1 title");
	await page.mouse.up();
	await expect(page).toHaveURL(/letter=G/);
	await expect(page.getByRole("heading", { name: "Gamma" })).toBeVisible();
	await expect(page.getByRole("heading", { name: "Arrival" })).toHaveCount(0);
	await expectAccessible(page, capture);
  return { capture, errors, passkeyCreated };
}
