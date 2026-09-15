import { expect, test, type Request } from "@playwright/test";
import { expectAccessible, finishOfflineInstall, openConnect, openSettings, signOut, totp } from "./happy-path-helpers";
import { holdNextLibraryPage, holdNextMainRequest } from "./request-holds";

test("Owner can set up, create a passkey, find, play, resume, curate, install, and sign back in", async ({ page }, testInfo) => {
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
    await page.getByLabel("Name").fill("Owner");
    await page.locator("#new-password").fill("test-password");
    await page.getByRole("button", { name: "Create Owner & continue" }).click();
    await expect(page.getByRole("heading", { name: "Protect the Owner account" })).toBeVisible();
    await expectAccessible(page, capture);
		if (testInfo.project.name === "chromium") {
			await page.getByRole("button", { name: "Create passkey" }).click();
			await expect(page).toHaveURL("/onboarding/connection");
			await expect(page.getByRole("heading", { name: "Choose how devices connect." })).toBeVisible();
			await expectAccessible(page, capture);
			await expect(page.getByText("Each choice is optional.", { exact: false })).toBeVisible();
			await expect(page.getByRole("heading", { name: "Secure local access" })).toBeVisible();
			await expect(page.getByRole("heading", { name: "Jellyfin apps" })).toBeVisible();
			await expect(page.getByLabel("Allow compatible Jellyfin apps to connect")).not.toBeChecked();
			await expect(page.getByRole("heading", { name: "Trusted HTTPS (Required for Jellyfin apps)" })).toBeVisible();
			await expect(page.getByRole("heading", { name: "Remote access comes later" })).toBeVisible();
			await expect(page.getByRole("group", { name: "DNS provider" }).locator('input[value="duckdns"]')).toBeChecked();
			await expect(page.getByLabel("Trusted hostname")).toBeVisible();
			await expect(page.getByLabel("Provider token")).toBeVisible();
			await expect(page.getByLabel("Kinosail LAN address")).toBeVisible();
			await expect(page.getByLabel(/Allow DNS validation/)).toBeVisible();
			await page.getByRole("link", { name: "Continue to household setup" }).click();
			await expect(page.getByRole("heading", { name: "Set up Viewer Profiles." })).toBeVisible();
			await page.getByRole("link", { name: "Continue to viewing history" }).click();
			await expect(page.getByRole("heading", { name: "Import your viewing history." })).toBeVisible();
			await page.getByRole("link", { name: "Finish and open Library" }).click();
			passkeyCreated = true;
		} else {
			await page.getByRole("button", { name: "Use an authenticator instead" }).click();
			authenticatorSecret = (await page.locator("code").first().textContent())!;
			await page.getByLabel("6-digit code").fill(totp(authenticatorSecret));
			await page.getByRole("button", { name: "Turn on extra sign-in protection" }).click();
			await expect(page).toHaveURL("/onboarding/connection");
			await page.getByRole("link", { name: "Continue to household setup" }).click();
			await page.getByRole("link", { name: "Continue to viewing history" }).click();
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
  await expect(page.getByRole("heading", { name: "Arrival" }).first()).toBeVisible();
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
	}, { timeout: 30_000 }).toMatchObject({ next: 0, status: "All titles are loaded." });
	const loadedLibrary = await page.evaluate(() => ({
		cards: document.querySelectorAll("#library .card").length,
		next: document.querySelectorAll("[data-library-next]").length,
		status: document.querySelector("[data-library-status]")?.textContent,
	}));
	expect(loadedLibrary.next).toBe(0);
	expect(loadedLibrary.status).toBe("All titles are loaded.");
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
	await page.getByText("More", { exact: true }).click();
	await expectAccessible(page, capture);
	await page.getByRole("link", { name: "Home", exact: true }).first().click();
	await openSettings(page);
	await expect(page.getByRole("heading", { name: "Server settings" })).toBeVisible();
	await expectAccessible(page, capture);
	if (testInfo.project.name === "chromium") {
		await page.getByRole("link", { name: "Access", exact: true }).click();
		if (await page.locator("#profiles").getByText("Partner", { exact: true }).count() === 0) {
			const profile = page.locator('form[action="/settings/profiles"]');
			await profile.getByLabel("New profile name").fill("Partner");
			await profile.getByLabel("New profile password").fill("partner-password");
			await profile.getByRole("group", { name: "Profile type" }).locator('input[value="true"]').check();
			await profile.getByRole("button", { name: "Add Profile" }).click();
		}
		await page.goto("/");
		await signOut(page);
		await page.getByLabel("Name").fill("Partner");
		await page.getByLabel("Password", { exact: true }).fill("partner-password");
		await page.getByRole("button", { name: "Sign in", exact: true }).click();
		await expect(page.getByRole("heading", { name: "Extra sign-in protection is required" })).toBeVisible();
		await page.getByRole("button", { name: "Set up an authenticator app" }).click();
		await expect(page.getByRole("heading", { name: "Add extra sign-in protection" })).toBeVisible();
		const secret = (await page.locator("code").first().textContent())!;
		await page.getByLabel("6-digit code").fill(totp(secret));
		await page.getByRole("button", { name: "Turn on extra sign-in protection" }).click();
		await expect(page.getByRole("heading", { name: "Ways to sign in" })).toBeVisible();
		await page.goto("/");
		await openSettings(page);
		await expect(page.getByRole("heading", { name: "Server settings" })).toBeVisible();
		await page.goto("/");
		await signOut(page);
		await page.getByLabel("Name").fill("Owner");
		await page.getByLabel("Password", { exact: true }).fill("test-password");
		await page.getByRole("button", { name: "Sign in", exact: true }).click();
		if (await page.getByRole("link", { name: "Not now" }).isVisible()) await page.getByRole("link", { name: "Not now" }).click();
		await openSettings(page);
	}
	await page.getByRole("link", { name: "System", exact: true }).click();
	await page.getByRole("link", { name: "Backup and recovery" }).click();
	await expect(page.getByRole("heading", { name: "Automatic backups" })).toBeVisible();
	if (testInfo.project.name === "chromium") {
		await page.getByRole("button", { name: "Back up now" }).click();
		await expect(page).toHaveURL("/login?stepup=1&next=%2Fsettings%2Fbackups");
		await page.getByRole("button", { name: "Sign in with passkey" }).click();
		await expect(page).toHaveURL("/settings/backups");
		await page.getByRole("button", { name: "Back up now" }).click();
		await expect(page).toHaveURL("/settings/backups");
		const lastBackup = page.getByText(/^Last successful backup:/);
		await expect(lastBackup).toBeVisible();
		await expect(lastBackup).not.toContainText("None");
		await page.getByRole("button", { name: "Verify latest backup" }).click();
		const lastVerified = page.getByText(/^Last verified:/);
		await expect(lastVerified).toBeVisible();
		await expect(lastVerified).not.toContainText("Never");
	}
	await openSettings(page);
	await page.locator('a.back[href="/"]').click();
	await openConnect(page);
	await expect(page.getByRole("heading", { name: "Connect a device" })).toBeVisible();
	await expectAccessible(page, capture);
	await page.getByRole("link", { name: "Cancel" }).click();
	await page.goto("/?q=missing-title");
	await expect(page.getByRole("heading", { name: "No matching titles." })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Arrival" })).toHaveCount(0);
	await page.goto("/?q=Arrival");
  await expect(page.getByRole("heading", { name: "Arrival" }).first()).toBeVisible();
  await page.locator('a.card[href^="/watch/"]').first().click();

  await expect(page.getByRole("heading", { name: "Arrival" })).toBeVisible();
	await expectAccessible(page, capture);
  const removeFromList = page.getByRole("button", { name: "Remove from My List" });
  if (await removeFromList.isVisible()) await removeFromList.click();
  const video = page.locator("video");
  await expect(video).toBeVisible();
  await video.evaluate(async (element: HTMLVideoElement) => {
    if (element.readyState < HTMLMediaElement.HAVE_METADATA) {
      await new Promise<void>((resolve, reject) => {
        element.addEventListener("loadedmetadata", () => resolve(), { once: true });
        element.addEventListener("error", () => reject(new Error("media failed to load")), { once: true });
      });
    }
    element.muted = true;
    await element.play();
  });
  await expect.poll(() => video.evaluate((element: HTMLVideoElement) => element.currentTime)).toBeGreaterThan(0.1);
  const progress = page.waitForResponse((response) => response.url().includes("/progress/") && response.request().method() === "POST");
  await video.evaluate((element: HTMLVideoElement) => element.pause());
  await progress;
  await page.getByRole("button", { name: "Add to My List" }).click();
  await page.getByRole("link", { name: "Library", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Continue watching" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "My List" })).toBeVisible();

  if (passkeyCreated) {
    await signOut(page);
    await expect(page.getByRole("heading", { name: "My List" })).toBeVisible();
  }
  await page.locator('a.card[href^="/watch/"]').first().click();
  const markUnwatched = page.getByRole("button", { name: "Mark unwatched" });
  if (await markUnwatched.isVisible()) await markUnwatched.click();
  await page.getByRole("button", { name: "Mark watched" }).click();
  await expect(page.getByRole("button", { name: "Mark unwatched" })).toBeVisible();
  await page.getByRole("link", { name: "Library", exact: true }).click();
  await expect(page.getByRole("heading", { name: "My List" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Continue watching" })).toHaveCount(0);
  await page.setViewportSize({ width: 390, height: 844 });
  await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
	await expectAccessible(page, capture);
	await finishOfflineInstall(page, testInfo.project.name, errors);
});
