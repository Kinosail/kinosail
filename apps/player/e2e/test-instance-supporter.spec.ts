import { configureProviderProfile, providerRoute, isolateProvider } from "./provider-profile-fixture";
import { expect, test } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { readFile, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { configureTestInstance, login } from "./test-instance-helpers";

configureProviderProfile();
configureTestInstance();
test.use({ serviceWorkers: "block" });

test("Pending supporter recognition keeps navigation usable and then shows the collection", async ({ page }, testInfo) => {
  await login(page);
  let gate = Promise.resolve();
  let release = () => {};
  await providerRoute(page, "**/api/v1/supporter/collection", async (route) => {
    await gate;
    await route.fulfill({ json: { display: "automatic", badges: [{ family: "patron-order", edition: "one-time", name: "Lighthouse", rank: 6 }] } });
  });
  for (const viewport of [{ width: 1440, height: 900 }, { width: 1024, height: 768 }, { width: 390, height: 844 }]) {
    gate = new Promise<void>((resolve) => { release = resolve; });
    await page.setViewportSize(viewport);
    await page.goto("/?view=movies", { waitUntil: "domcontentloaded" });
    const support = page.locator(".header-supporter:visible");
    try {
      await expect(support).toHaveText("Support Kinosail");
      await expect(support).toHaveAttribute("href", "/supporter");
      await expect(support.locator("img")).toHaveCount(0);
    } finally { release(); }
    await expect(support).toHaveAttribute("aria-label", "Your supporter collection");
    await expect(support.locator("img")).toHaveAttribute("alt", "Lighthouse · one-time");
    await page.screenshot({ path: testInfo.outputPath(`pending-supporter-${viewport.width}.png`), fullPage: true });
  }
});

test("Supporter brand marks do not compress the server title", async ({ page }, testInfo) => {
  await login(page);
  for (const viewport of [{ width: 1440, height: 900 }, { width: 1024, height: 768 }, { width: 390, height: 844 }]) {
    await page.setViewportSize(viewport);
    await page.goto("/?view=movies");
    await page.locator(".brand-lockup").evaluate((brand) => {
      const mark = document.createElement("a");
      mark.className = "supporter-edition-mark";
      mark.href = "/supporter";
      mark.textContent = "LIGHTHOUSE · 0130ABCDEF";
      brand.append(mark);
    });
    const layout = await page.locator(".brand-lockup").evaluate((brand) => {
      const title = brand.querySelector("h1").getBoundingClientRect();
      const mark = brand.querySelector(".supporter-edition-mark");
      const markBox = mark.getBoundingClientRect();
      return { titleWidth: title.width, markDisplay: getComputedStyle(mark).display, markRight: markBox.right, brandRight: brand.getBoundingClientRect().right };
    });
    expect(layout.titleWidth).toBeGreaterThan(0);
    if (viewport.width >= 901) expect(layout.markRight).toBeLessThanOrEqual(layout.brandRight + 0.5);
    else expect(layout.markDisplay).toBe("none");
    await page.screenshot({ path: testInfo.outputPath(`supporter-brand-mark-${viewport.width}.png`) });
  }
});

test("Supporter chooser combines badges and prices with optional activation", async ({ page }, testInfo) => {
  await login(page);
  for (const viewport of [{ width: 1440, height: 900 }, { width: 1024, height: 768 }, { width: 720, height: 450 }, { width: 390, height: 844 }, { width: 320, height: 800 }]) {
    await page.setViewportSize(viewport);
    await page.goto("/supporter");
    await expect(page.getByRole("heading", { name: "A place in the story." })).toBeVisible();
    await expect(page.getByText("Monthly support · Gold radiant crest. Collect it alongside your other editions.")).toBeVisible();
    await expect(page.getByLabel("Supporter key", { exact: true })).toBeHidden();
    await page.getByText("Already supported? Activate your badge", { exact: true }).click();
    await page.getByText("About badges and privacy", { exact: true }).click();
    await expect(page.getByText("One-time, monthly, and yearly support each have their own collectible badge.", { exact: false })).toBeVisible();
    await expect(page.getByText("Existing recurring certificates stay in your archive until refreshed", { exact: false })).toBeVisible();
    await expect(page.getByText("Activation sends your key, the Kinosail Player app ID", { exact: false })).toBeVisible();
    await expect(page.getByRole("link", { name: "Continue to support", exact: true })).toHaveAttribute("rel", "external noreferrer");
    await expect(page.getByRole("heading", { name: "Add or refresh one badge" })).toBeVisible();
    await expect(page.getByLabel("Public certificate name")).toHaveAttribute("maxlength", "80");
    await expect(page.getByText("Kinosail does not store your key or payment details.", { exact: false })).toBeVisible();
    await expect(page.getByRole("link", { name: "Public supporters" })).toHaveAttribute("rel", "external noreferrer");
    for (const edition of ["monthly", "yearly", "one-time"]) await expect(page.locator(`.supporter-badge[data-family="${edition}"]`)).toHaveCount(10);
    await expect(page.locator("[data-supporter-family]:not([hidden]) .supporter-price:visible")).toHaveCount(10);
    await expect(page.locator(".supporter-contribute details")).toHaveCount(0);
    for (const badge of ["Commodore", "Admiral", "North Star", "Legacy"]) await expect(page.getByRole("heading", { name: badge, exact: true })).toBeVisible();
    expect(await page.locator("main").evaluate((element) => element.scrollWidth <= element.clientWidth)).toBe(true);
    expect((await new AxeBuilder({ page }).include("main").analyze()).violations, `${viewport.width}px supporter accessibility`).toEqual([]);
    await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-supporter-donation.png`), fullPage: true });
  }
  await page.setViewportSize({ width: 390, height: 844 });
  await page.emulateMedia({ colorScheme: "dark", reducedMotion: "reduce" });
  await page.evaluate(() => localStorage.removeItem("kinosail-theme"));
  await page.goto("/supporter");
  await page.screenshot({ path: testInfo.outputPath("390-supporter-donation-dark-reduced-motion.png"), fullPage: true });
  await page.emulateMedia({ forcedColors: "active" });
  await page.keyboard.press("Tab");
  const focus = await page.evaluate(() => {
    const element = document.activeElement as HTMLElement;
    const style = getComputedStyle(element);
    return { interactive: element.matches("a,button,input"), visible: element.getClientRects().length > 0, indicated: element.matches(":focus-visible") || style.outlineStyle !== "none" && Number.parseFloat(style.outlineWidth) > 0 };
  });
  expect(focus).toEqual({ interactive: true, visible: true, indicated: true });
  expect((await new AxeBuilder({ page }).include("main").analyze()).violations, "forced-colors supporter accessibility").toEqual([]);
  await page.screenshot({ path: testInfo.outputPath("390-supporter-donation-forced-colors.png"), fullPage: true });
});

test("Every supporter level shows its artwork and title", async ({ page }, testInfo) => {
  await login(page);
  await page.setViewportSize({ width: 1024, height: 768 });
  await page.goto("/supporter");
  await expect(page.locator(".supporter-contribute details")).toHaveCount(0);
  for (const [family, cadence] of [["monthly", "Monthly"], ["yearly", "Yearly"], ["one-time", "One-time"]] as const) {
    await page.getByRole("radio", { name: cadence, exact: true }).check();
    for (let rank = 1; rank <= 10; rank += 1) {
      const badge = page.locator(`.supporter-badge[data-family="${family}"][data-rank="${rank}"]`).first();
      await expect(badge).toBeVisible();
      await expect(badge.locator(".supporter-badge-art")).toHaveAttribute("src", new RegExp(`/static/supporter/badges/${family}-${rank}\\.svg`));
    }
  }
  await expect(page.locator(".supporter-badge-art")).toHaveCount(30);
  await expect.poll(() => page.locator(".supporter-badge-art").evaluateAll((images) => images.every((image) => (image as HTMLImageElement).naturalWidth > 0))).toBe(true);
  await expect(page.locator('[data-supporter-family="one-time"]').getByRole("heading", { name: "Legacy", exact: true })).toBeVisible();
  await page.screenshot({ path: testInfo.outputPath("supporter-badge-families.png"), fullPage: true });
  expect(await page.locator("main").evaluate((element) => element.scrollWidth <= element.clientWidth)).toBe(true);
});

test("Legacy supporter honors and masterwork remain visible beside current editions", async ({ browser }, testInfo) => {
  const fixtureDirectory = process.env.KINOSAIL_UI_FIXTURE_DIR;
  test.skip(!fixtureDirectory, "requires the production supporter fixture");
  const supporterPage = await readFile(join(fixtureDirectory!, "supporter-populated-both.html"), "utf8");
  const certificate = await readFile(join(fixtureDirectory!, "supporter-living-certificate.html"), "utf8");
  const context = await browser.newContext({ baseURL: process.env.KINOSAIL_E2E_URL, ignoreHTTPSErrors: false, serviceWorkers: "block" });
  if (process.env.KINOSAIL_PROVIDER_PROFILE) await isolateProvider(context, testInfo.project.name, testInfo.project.use.baseURL);
  const page = await context.newPage();
  await login(page);
  await providerRoute(page, (url) => url.pathname === "/supporter", (route) => route.fulfill({ status: 200, contentType: "text/html", body: supporterPage }));
  await providerRoute(page, "**/api/v1/supporter/certificates/living-standard.svg", (route) => route.fulfill({ status: 200, contentType: "image/svg+xml", body: certificate }));
  for (const viewport of [{ width: 1440, height: 900 }, { width: 1024, height: 768 }, { width: 390, height: 844 }, { width: 320, height: 800 }]) {
    await page.setViewportSize(viewport);
    await page.goto("/supporter");
    await expect(page.getByText("Full Sail joins your Patron Order and Living Standard.")).toBeVisible();
    await expect(page.getByText("Lighthouse Ascendant")).toBeVisible();
    await expect(page.locator('.supporter-owned-grid .owned-badge')).toHaveCount(2);
    await expect(page.locator('.supporter-gallery li.collected')).toHaveCount(5);
    await expect(page.locator('.supporter-gallery li.current')).toHaveCount(1);
    await expect(page.locator('.supporter-gallery .supporter-badge[data-family="monthly"]')).toHaveCount(10);
    await expect(page.locator('.supporter-gallery .supporter-badge[data-family="yearly"]')).toHaveCount(10);
    expect((await new AxeBuilder({ page }).include("main").analyze()).violations).toEqual([]);
    expect(await page.locator("main").evaluate((element) => element.scrollWidth <= element.clientWidth)).toBe(true);
    await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-supporter-both.png`), fullPage: true, animations: "disabled" });
  }
  await context.close();
});

test("Supporter badge rendering and share conversion remain responsive", async ({ browser }, testInfo) => {
  const fixtureDirectory = process.env.KINOSAIL_UI_FIXTURE_DIR;
  test.skip(!fixtureDirectory, "requires the production certificate fixture");
  const certificate = await readFile(join(fixtureDirectory!, "supporter-certificate.html"), "utf8");
	const context = await browser.newContext({ baseURL: process.env.KINOSAIL_E2E_URL, ignoreHTTPSErrors: false, serviceWorkers: "block" });
	if (process.env.KINOSAIL_PROVIDER_PROFILE) await isolateProvider(context, testInfo.project.name, testInfo.project.use.baseURL);
	const page = await context.newPage();
  await login(page);
  await providerRoute(page, "**/api/v1/supporter/certificates/living-standard.svg", async (route) => route.fulfill({ status: 200, contentType: "image/svg+xml", body: certificate }));
  await page.goto("/supporter");
  const galleryMilliseconds = await page.evaluate(async () => {
    const start = performance.now();
    document.querySelectorAll(".supporter-badge").forEach((badge) => badge.getBoundingClientRect());
    await new Promise(requestAnimationFrame);
    return performance.now() - start;
  });
  expect(galleryMilliseconds).toBeLessThan(500);
  const conversion = await page.evaluate(async () => {
    const start = performance.now();
    const share = (window as Window & { supporterShareFile: (family: string) => Promise<File> }).supporterShareFile;
    const file = await share("living-standard");
    const dataURL = await new Promise<string>((resolve, reject) => {
      const reader = new FileReader();
      reader.onload = () => resolve(String(reader.result));
      reader.onerror = () => reject(reader.error);
      reader.readAsDataURL(file);
    });
    return { milliseconds: performance.now() - start, type: file.type, size: file.size, dataURL };
  });
  expect(conversion.milliseconds).toBeLessThan(3000);
  expect(conversion.type).toBe("image/png");
  expect(conversion.size).toBeGreaterThan(10_000);
  await writeFile(testInfo.outputPath("elite-living-share.png"), Buffer.from(conversion.dataURL.split(",")[1], "base64"));

	await page.evaluate(() => {
		Object.defineProperty(navigator, "canShare", { configurable: true, value: () => true });
		Object.defineProperty(navigator, "share", { configurable: true, value: async () => { throw new DOMException("cancelled", "AbortError"); } });
		const button = document.createElement("button");
		button.type = "button";
		button.dataset.supporterShare = "living-standard";
		button.textContent = "Share Living Standard";
		const output = document.createElement("output");
		output.dataset.supporterShareStatus = "living-standard";
		document.body.append(button, output);
		(window as Window & { bindSupporterShare: () => void }).bindSupporterShare();
	});
	const cancelButton = page.getByRole("button", { name: "Share Living Standard" });
	await cancelButton.click();
	await expect(cancelButton).toBeEnabled();
	await expect(page.locator('[data-supporter-share-status="living-standard"]')).toHaveText("");
	await page.evaluate(() => Object.defineProperty(navigator, "share", { configurable: true, value: async () => { throw new DOMException("blocked", "NotAllowedError"); } }));
	const fallback = page.waitForEvent("download");
	await cancelButton.click();
	expect((await fallback).suggestedFilename()).toBe("kinosail-player-living-standard.png");
	await expect(page.locator('[data-supporter-share-status="living-standard"]')).toHaveText("Share image downloaded.");
	await context.close();
});

test("Home keeps support reachable and respects hidden supporter recognition", async ({ page }) => {
  await login(page);
  await providerRoute(page, "**/api/v1/supporter/collection", route => route.fulfill({ json: { badges: [], display: "automatic" } }));
  for (const width of [1440, 1024, 768, 390, 320]) {
    await page.setViewportSize({ width, height: 900 });
    await page.goto("/");
    const notice = page.locator(".header-supporter:visible");
    await expect(notice).toBeVisible();
    await expect(notice).toHaveText("Support Kinosail");
    await expect(notice).toHaveAttribute("href", "/supporter");
    expect(await notice.evaluate(el => el.scrollWidth <= el.clientWidth)).toBe(true);
    expect(await notice.evaluate(el => el.getBoundingClientRect().height)).toBeLessThanOrEqual(56);
  }
  await providerRoute(page, "**/api/v1/supporter/collection", route => route.fulfill({ json: { badges: [], display: "hidden" } }));
  await page.reload();
  await expect(page.locator(".header-supporter:visible")).toHaveCount(0);
  await providerRoute(page, "**/api/v1/supporter/collection", route => route.fulfill({ status: 503 }));
  await page.reload();
  await expect(page.locator(".header-supporter:visible")).toHaveText("Support Kinosail");
});
