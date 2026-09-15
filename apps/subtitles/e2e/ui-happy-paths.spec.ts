import { createHmac } from "node:crypto";
import { expect, test, type Page } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";

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
	await page.getByLabel("6-digit code").fill(totp());
	await page.getByRole("button", { name: "Sign in", exact: true }).click();
	if (await page.getByRole("link", { name: "Not now" }).isVisible()) await page.getByRole("link", { name: "Not now" }).click();
	await expect(page).toHaveURL("/");
}

test("Owner can toggle the active title letter without a second scroll", async ({ page }) => {
	await login(page);
	await page.setViewportSize({ width: 1440, height: 900 });
	await page.goto("/?view=movies");

	const titleJump = page.getByRole("navigation", { name: "Jump to title" });
	await titleJump.getByRole("link", { name: "B, 1 title" }).click();
	await expect(page).toHaveURL(/letter=B/);
	await expect(page.getByRole("heading", { name: "Beta" })).toBeVisible();
	await expect(page.getByRole("heading", { name: "Arrival" })).toHaveCount(0);
	await expect(page.locator("#library a.card").first()).toBeFocused();
	await expect.poll(() => page.evaluate(() => window.scrollY)).toBe(0);

	await titleJump.getByRole("link", { name: "B, 1 title, selected; activate to show all titles" }).click();
	await expect(page).not.toHaveURL(/letter=/);
	await expect(page.getByRole("heading", { name: "Arrival" })).toBeVisible();
	await expect(page.getByRole("heading", { name: "Beta" })).toBeVisible();
	await expect.poll(() => page.evaluate(() => window.scrollY)).toBe(0);
});

test("Owner can use keyboard utilities and persist the chosen theme", async ({ page }) => {
	await login(page);
	await page.keyboard.press("Control+K");
	const commands = page.getByRole("dialog", { name: "Browse library" });
	await expect(commands).toBeVisible();
	await commands.getByLabel("Search library").fill("offline");
	await expect(commands.getByRole("link", { name: "Offline downloads" })).toBeVisible();
	await commands.getByLabel("Search library").press("Enter");
	await expect(page).toHaveURL("/offline-downloads");
	await expect(page.getByRole("heading", { name: "Offline downloads", exact: true })).toBeVisible();

	await page.goto("/settings#appearance");
	const theme = page.getByRole("group", { name: "Theme" });
	await theme.locator('input[value="light"]').check();
	await expect(page.locator("html")).toHaveAttribute("data-theme", "light");
	await expect.poll(() => page.evaluate(() => localStorage.getItem("kinosail-theme"))).toBe("light");
	await theme.locator('input[value="dark"]').check();
	await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
	await theme.locator('input[value="system"]').check();
	await expect(page.locator("html")).not.toHaveAttribute("data-theme", /.+/);
});

test("Dark is the default and every theme choice persists", async ({ page }, testInfo) => {
	await page.emulateMedia({ colorScheme: "light" });
	await login(page);
	await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
	await expect.poll(() => page.evaluate(() => localStorage.getItem("kinosail-theme"))).toBeNull();
	await page.goto("/settings#appearance");
	const theme = page.getByRole("group", { name: "Theme" });
	await expect(theme.locator('input[value="dark"]')).toBeChecked();
	expect(await theme.locator("label").allTextContents()).toEqual(["Dark", "Light", "System"]);
	for (const viewport of [{ width: 1440, height: 900 }, { width: 1024, height: 768 }, { width: 720, height: 450 }, { width: 390, height: 844 }, { width: 320, height: 640 }]) {
		await page.setViewportSize(viewport);
		await page.goto("/settings#appearance");
		await theme.locator("input:checked").focus();
		await page.keyboard.press("ArrowRight");
		await theme.locator('input[value="dark"]').check();
		expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true);
		expect((await new AxeBuilder({ page }).include("#appearance").analyze()).violations).toEqual([]);
		await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-dark-default-theme.png`), fullPage: true });
	}
	await page.emulateMedia({ forcedColors: "active", reducedMotion: "reduce" });
	await page.setViewportSize({ width: 390, height: 844 });
	await expect(theme).toBeVisible();
	await page.screenshot({ path: testInfo.outputPath("390-dark-default-theme-forced-colors.png"), fullPage: true });
	for (const choice of ["light", "dark", "system"]) {
		await theme.locator(`input[value="${choice}"]`).check();
		await expect.poll(() => page.evaluate(() => localStorage.getItem("kinosail-theme"))).toBe(choice);
	}
	await expect(page.locator("html")).not.toHaveAttribute("data-theme", /.+/);
});

test("Owner can create, fill, empty, and delete a playlist and Collection", async ({ page }, testInfo) => {
	await login(page);
	const suffix = `${testInfo.project.name}-${Date.now()}`;
	const playlistName = `E2E playlist ${suffix}`;
	await page.goto("/?view=playlists");
	const playlistForm = page.locator('form[action="/playlists"]');
	await playlistForm.getByLabel("New playlist name").fill(playlistName);
	await playlistForm.getByRole("button", { name: "Create" }).click();
	await expect(page.getByRole("heading", { name: playlistName, exact: true })).toBeVisible();

	const collectionName = `E2E Collection ${suffix}`;
	await page.goto("/?view=collections");
	const defaultCollections = page.locator('[aria-labelledby="default-collections-heading"]');
	await expect(defaultCollections.getByText("Example Collection", { exact: true })).toBeVisible();
	await expect(defaultCollections.getByText("1 item", { exact: true })).toBeVisible();
	const populatedPoster = defaultCollections.locator(".curation-poster:not(:empty)");
	await expect(populatedPoster).toHaveCount(1);
	expect(await populatedPoster.evaluate((element) => getComputedStyle(element, "::before").content)).not.toBe("none");
	await populatedPoster.evaluate((element) => {
		element.append(document.createElement("span"));
		element.append(document.createElement("span"));
	});
	expect(await populatedPoster.evaluate((element) => getComputedStyle(element, "::before").gridRowStart)).toBe("2");
	expect(await populatedPoster.evaluate((element) => getComputedStyle(element, "::before").gridColumnStart)).toBe("2");
	expect(await populatedPoster.evaluate((element) => getComputedStyle(element, "::before").content)).not.toBe("none");
	await populatedPoster.evaluate((element) => element.append(document.createElement("span")));
	expect(await populatedPoster.evaluate((element) => getComputedStyle(element, "::before").content)).toBe("none");
	await page.getByLabel("New Collection name").fill(collectionName);
	await page.getByRole("button", { name: "Create" }).click();
	await expect(page.getByRole("heading", { name: collectionName, exact: true })).toBeVisible();
	await page.goto("/?view=collections");
	const customCollections = page.locator('[aria-labelledby="custom-collections-heading"]');
	await expect(customCollections.getByText(collectionName, { exact: true })).toBeVisible();
	await expect(defaultCollections.getByText("Example Collection", { exact: true })).toBeVisible();

	await page.goto("/?view=movies");
	await page.getByRole("link", { name: /Example Movie/ }).click();
	await page.getByText("Add to playlist or collection", { exact: true }).click();
	await expect(page.getByRole("heading", { name: "Playlists", exact: true })).toBeVisible();
	await expect(page.getByRole("heading", { name: "Collections", exact: true })).toBeVisible();
	const destinationSearch = page.getByLabel("Search playlists and collections");
	await destinationSearch.fill(playlistName);
	await expect(page.getByRole("button", { name: `Add to playlist · ${playlistName}` })).toBeVisible();
	await expect(page.getByRole("heading", { name: "Collections", exact: true })).toBeHidden();
	await destinationSearch.fill(collectionName);
	await expect(page.getByRole("button", { name: `Add to Collection · ${collectionName}` })).toBeVisible();
	await expect(page.getByRole("heading", { name: "Playlists", exact: true })).toBeHidden();
	await destinationSearch.fill("No matching destination");
	await expect(page.getByText("No playlists or collections found.", { exact: true })).toBeVisible();
	await destinationSearch.fill(playlistName);
	await page.getByRole("button", { name: `Add to playlist · ${playlistName}` }).click();
	await page.getByText("Add to playlist or collection", { exact: true }).click();
	await page.getByRole("button", { name: `Add to Collection · ${collectionName}` }).click();

	await page.goto(`/playlist/${encodeURIComponent(playlistName)}`);
	const playlistItems = page.locator(".collection-items");
	await expect(playlistItems.getByRole("heading", { name: "Example Movie", exact: true })).toBeVisible();
	await playlistItems.getByRole("button", { name: "Remove" }).click();
	await expect(page.getByRole("heading", { name: "This playlist is empty." })).toBeVisible();
	await page.getByText(`Delete ${playlistName}?`, { exact: true }).click();
	await page.getByRole("button", { name: "Confirm delete" }).click();
	await expect(page).toHaveURL(/view=playlists/);

	await page.goto(`/collection/${encodeURIComponent(collectionName)}`);
	const collectionItems = page.locator(".collection-items");
	await expect(collectionItems.getByRole("heading", { name: "Example Movie", exact: true })).toBeVisible();
	await collectionItems.getByRole("button", { name: "Remove" }).click();
	await expect(page.getByRole("heading", { name: "This Collection is empty." })).toBeVisible();
	await page.getByText(`Delete ${collectionName}?`, { exact: true }).click();
	await page.getByRole("button", { name: "Confirm delete" }).click();
	await expect(page).toHaveURL("/");
	await page.goto("/?view=collections");
	await expect(page.getByRole("heading", { name: collectionName, exact: true })).toHaveCount(0);
});

test("Owner can create and revoke an API key and manage a Viewer Profile", async ({ page }, testInfo) => {
	await login(page);
	const suffix = `${testInfo.project.name}-${Date.now()}`;
	const keyName = `E2E key ${suffix}`;
	await page.goto("/settings#system");
	const keyForm = page.locator('form[action="/settings/api-keys"]');
	await keyForm.getByLabel("API key name").fill(keyName);
	await keyForm.getByLabel("Stream media").check();
	await keyForm.getByRole("button", { name: "Create API key" }).click();
	await expect(page.getByRole("heading", { name: keyName, exact: true })).toBeVisible();
	await expect(page.locator("code.grant-link")).toHaveText(/^ks_/);
	await page.getByRole("link", { name: "Return to settings" }).click();
	await page.getByRole("link", { name: "System", exact: true }).click();
	await page.getByRole("button", { name: `Revoke ${keyName}` }).click();
	await expect(page.getByRole("button", { name: `Revoke ${keyName}` })).toHaveCount(0);

	const profileName = `E2E Viewer ${suffix}`;
	await page.getByRole("link", { name: "Access", exact: true }).click();
	const addProfile = page.locator('form[action="/settings/profiles"]');
	await addProfile.getByLabel("New profile name").fill(profileName);
	await addProfile.getByLabel("New profile password").fill("viewer-password");
	await addProfile.getByLabel("Libraries").fill("all");
	await addProfile.getByLabel("Allow downloads").check();
	await addProfile.getByRole("button", { name: "Add Profile" }).click();
	await page.getByRole("link", { name: "Access", exact: true }).click();
	let row = page.locator(".profile-row").filter({ hasText: profileName });
	await expect(row).toBeVisible();
	await row.getByRole("group", { name: "Content" }).locator('input[value="teen"]').check();
	await row.getByLabel("Managed remote access").check();
	await row.getByRole("button", { name: "Save" }).click();
	await page.getByRole("link", { name: "Access", exact: true }).click();
	row = page.locator(".profile-row").filter({ hasText: profileName });
	await expect(row.getByRole("group", { name: "Content" }).locator('input[value="teen"]')).toBeChecked();
	await expect(row.getByLabel("Managed remote access")).toBeChecked();

	const reset = page.locator('form[action="/settings/profiles/password"]');
	await reset.getByLabel("Profile to reset").selectOption({ label: profileName });
	await reset.getByLabel("New profile password").fill("replacement-password");
	await reset.getByRole("button", { name: "Reset password" }).click();
	await page.getByRole("link", { name: "Access", exact: true }).click();
	row = page.locator(".profile-row").filter({ hasText: profileName });
	await row.getByText(`Remove ${profileName}?`, { exact: true }).click();
	await row.getByRole("button", { name: `Confirm remove ${profileName}` }).click();
	await expect(page.locator(".profile-row").filter({ hasText: profileName })).toHaveCount(0);
});

test("Owner can share Library Content, open the claim, and revoke it", async ({ browser, page }, testInfo) => {
	await login(page);
	await page.setViewportSize({ width: 390, height: 844 });
	await page.goto("/settings/media-shares");
	const form = page.locator('form[action="/settings/media-shares"]');
	await form.getByLabel(/Example Movie/).check();
	const expiry = form.getByRole("group", { name: "Expires" });
	const devices = form.getByRole("group", { name: "Devices" });
	await expect(expiry.getByRole("radio", { name: "One hour" })).toBeChecked();
	await page.screenshot({ path: testInfo.outputPath("390-media-share-choices.png"), fullPage: true });
	await devices.getByRole("radio", { name: "Two" }).check();
	await form.getByLabel(/I confirm I have the right/).check();
	await form.getByRole("button", { name: "Create secure Media Share" }).click();
	const claimURL = await page.getByRole("status").locator("code").textContent();
	expect(claimURL).toMatch(/^\/share#.+/);

	const authenticatedClaim = await page.context().newPage();
	await authenticatedClaim.goto(new URL(claimURL!, page.url()).toString());
	await expect(authenticatedClaim).toHaveURL(/\/share\/items$/);
	await expect(authenticatedClaim.getByRole("heading", { name: "Shared with you" })).toBeVisible();
	await authenticatedClaim.close();

	const claimContext = await browser.newContext({ ignoreHTTPSErrors: true });
	const claim = await claimContext.newPage();
	await claim.goto(new URL(claimURL!, page.url()).toString());
	await expect(claim).toHaveURL(/\/share\/items$/);
	await expect(claim.getByRole("heading", { name: "Shared with you" })).toBeVisible();
	await expect(claim.getByRole("heading", { name: "Example Movie", exact: true })).toBeVisible();
	await expect(claim.locator(".media-stage video")).toBeVisible();
	await claimContext.close();

	await page.getByRole("heading", { name: "Active" }).locator("..").getByRole("button", { name: "Revoke" }).click();
	await expect(page.getByText("No active Media Shares.")).toBeVisible();
});

test("Owner can authorize a waiting device with Quick Connect", async ({ page, request }) => {
	await login(page);
	const started = await request.post("/api/v1/quick-connect", { data: { device: "E2E living room" } });
	expect(started.status()).toBe(201);
	const pending = await started.json() as { code: string; secret: string };
	await page.goto("/quick-connect");
	await page.getByLabel("Code").fill(pending.code);
	await page.getByRole("button", { name: "Authorize device" }).click();
	await expect(page).toHaveURL("/");
	const completed = await request.post("/api/v1/quick-connect/token", { data: { secret: pending.secret } });
	expect(completed.status()).toBe(201);
	expect((await completed.json()).token).toEqual(expect.any(String));
});
