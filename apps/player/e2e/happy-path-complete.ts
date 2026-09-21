import { expect, type Page, type TestInfo } from "@playwright/test";
import { expectAccessible, openQuickConnect, openSettings, signOut, totp, type HappyPathState } from "./happy-path-helpers";

export async function completeHappyPath(page: Page, testInfo: TestInfo, { capture, errors, passkeyCreated }: HappyPathState) {
	await page.getByText("More", { exact: true }).click();
	await expectAccessible(page, capture);
	await page.getByRole("link", { name: "Home", exact: true }).first().click();
	await openSettings(page);
	await expect(page.getByRole("heading", { name: "Server settings" })).toBeVisible();
	await expectAccessible(page, capture);
	if (testInfo.project.name === "chromium") {
		await page.getByRole("link", { name: "Viewer Profiles", exact: true }).click();
		if (await page.locator("#profiles").getByText("Partner", { exact: true }).count() === 0) {
			const profile = page.locator('form[action="/settings/profiles"]');
			await profile.getByLabel("New profile name").fill("Partner");
			await profile.getByLabel("New profile password").fill("partner-password");
			await profile.getByRole("group", { name: "Profile type" }).locator('input[value="true"]').check();
			await profile.getByRole("button", { name: "Add Profile" }).click();
		}
		await page.goto("/");
		await page.evaluate(() => localStorage.removeItem("kinosail-passkey"));
		await signOut(page);
		await page.getByLabel("Name").fill("Partner");
		await page.getByLabel("Password", { exact: true }).fill("partner-password");
		await page.getByRole("button", { name: "Sign in", exact: true }).click();
		await expect(page.getByRole("heading", { name: "Extra sign-in protection is required" })).toBeVisible();
		await page.getByRole("button", { name: "Set up an authenticator app" }).click();
		await expect(page.getByRole("heading", { name: "Add extra sign-in protection" })).toBeVisible();
		const secret = (await page.locator("code").first().textContent())!;
		await page.getByLabel("Authentication code").fill(totp(secret));
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
	await page.getByRole("link", { name: "Advanced", exact: true }).click();
	await page.getByRole("link", { name: "Server tools", exact: true }).click();
	await page.getByRole("link", { name: "Backup and recovery" }).click();
	await expect(page.getByRole("heading", { name: "Automatic backups" })).toBeVisible();
	if (testInfo.project.name === "chromium") {
		await page.getByRole("button", { name: "Back up now" }).click();
		await expect(page).toHaveURL("/settings/backups");
		// Fresh password authentication permits the backup; also exercise passkey reauthentication.
		await page.goto("/login?stepup=1&next=%2Fsettings%2Fbackups");
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
	await openQuickConnect(page);
	await expect(page.getByRole("heading", { name: "Connect a device" })).toBeVisible();
	await expectAccessible(page, capture);
	await page.getByRole("link", { name: "Cancel" }).click();
	await page.waitForURL("/", { waitUntil: "load" });
	await expect(page.getByRole("heading", { name: "Kinosail", exact: true })).toBeVisible();
	await page.goto("/?q=missing-title");
	await expect(page.getByRole("heading", { name: "No matching titles." })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Arrival" })).toHaveCount(0);
	await page.goto("/?q=Arrival");
  await expect(page.getByRole("heading", { name: "Arrival" }).first()).toBeVisible();
  await page.locator('a.card[href^="/item/"]').first().click();
  await page.getByRole("link", { name: /^(Play|Resume|Play again)$/ }).click();

  await expect(page.getByRole("heading", { name: "Arrival" })).toBeVisible();
	await expectAccessible(page, capture);
  const removeFromList = page.getByRole("button", { name: "Remove from My List" });
  if (await removeFromList.isVisible()) await removeFromList.click();
  const video = page.locator("video");
  await expect(video).toBeVisible();
  await page.getByRole("button", { name: "Start video transcode", exact: true }).click();
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
  await page.locator('a[href^="/watch/"]').first().click();
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
	await expect.poll(() => page.evaluate(async () => {
		const registration = await navigator.serviceWorker.getRegistration();
		return Boolean(registration?.active?.state === "activated" && !registration.installing && !registration.waiting && navigator.serviceWorker.controller === registration.active);
	}), { message: "latest service worker controls the page" }).toBeTruthy();
	const missingShell = await page.evaluate(async () => {
		const shell = ["/offline", "/static/app.css", "/static/main.kinosail.bundle.js", "/static/theme.js", "/static/pwa.js", "/static/player.js", "/static/downloads.js", "/static/icon.svg", "/static/icon-192.png", "/static/icon-512.png", "/static/icon-maskable-512.png", "/static/apple-touch-icon.png", "/static/cinema-backdrop.jpg"];
		const names = (await caches.keys()).filter((name) => name.startsWith("kinosail-shell-"));
		if (names.length !== 1) return ["Expected one active shell cache"];
		const cache = await caches.open(names[0]);
		return (await Promise.all(shell.map(async (path) => ({ path, response: await cache.match(path) })))).filter(({ response }) => !response?.ok).map(({ path }) => path);
	});
	expect(missingShell).toEqual([]);
	const cached = await page.evaluate(async () => (await Promise.all((await caches.keys()).map(async (name) => (await caches.open(name)).keys()))).flat().map((request) => new URL(request.url).pathname));
	expect(cached.some((path) => path.startsWith("/api/") || path.startsWith("/media/") || path.startsWith("/hls/"))).toBe(false);
	if (testInfo.project.name === "chromium") {
		const cacheClient = await page.context().newCDPSession(page);
		await cacheClient.send("Network.clearBrowserCache");
		await page.context().setOffline(true);
		try {
			await page.goto("/?offline-check=1");
			await expect(page.getByRole("heading", { name: "Offline downloads" })).toBeVisible();
			await expect(page.getByRole("link", { name: "Open Server downloads" })).toBeVisible();
			await expect(page.locator('link[rel="stylesheet"]')).toHaveAttribute("href", /\/static\/app\.css\?v=[a-z0-9-]+$/);
			await expect(page.locator("body")).toHaveCSS("color", "rgb(246, 248, 242)");
		} finally {
			await page.context().setOffline(false);
		}
	}
	expect(errors.filter((error) => !error.includes("blob:http://") && !(testInfo.project.name === "firefox" && error.includes("NS_BINDING_ABORTED") && error.includes("WorkerMain.js")))).toEqual([]);
}
