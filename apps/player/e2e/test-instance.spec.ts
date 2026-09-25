import { expect, test } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { configureTestInstance, login } from "./test-instance-helpers";

configureTestInstance();

test("Connection choices stay optional and secure by default", async ({ page }, testInfo) => {
	await login(page);
	for (const viewport of [{ width: 1440, height: 900 }, { width: 390, height: 844 }, { width: 320, height: 800 }]) {
		await page.setViewportSize(viewport);
		await page.goto("/settings#access");
		await page.getByRole("link", { name: "Connections", exact: true }).click();
		const headings = await page.locator("section:visible h2").allTextContents();
		const firstChoice = headings.indexOf("Secure local access");
		expect(headings.slice(firstChoice, firstChoice + 3)).toEqual(["Secure local access", "Jellyfin apps", "Trusted HTTPS (Required for Jellyfin apps)"]);
		await expect(page.getByText("On by default.", { exact: true })).toBeVisible();
		await expect(page.getByLabel("Allow compatible Jellyfin apps to connect")).not.toBeChecked();
		await expect(page.locator("#jellyfin")).toContainText("Trusted HTTPS is required.");
		await expect(page.locator("#jellyfin")).toContainText("Set up Trusted HTTPS first.");
		await expect(page.getByRole("heading", { name: "Trusted HTTPS (Required for Jellyfin apps)" })).toBeVisible();
		await expect(page.locator("#trusted-https p").first()).toContainText("Required for Jellyfin apps. Recommended for phones and TVs.");
		expect(await page.locator("main").evaluate((element) => element.scrollWidth <= element.clientWidth)).toBe(true);
		const accessibility = await new AxeBuilder({ page }).include("main").analyze();
		expect(accessibility.violations).toEqual([]);
		await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-secure-connection-choices.png`), fullPage: true });
	}
});

test("Mobile More menu prioritizes personal tabs", async ({ page }, testInfo) => {
  await page.addInitScript(() => localStorage.setItem("kinosail-theme", "dark"));
  await login(page);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/");
  await page.getByRole("navigation", { name: "Main navigation" }).getByText("More", { exact: true }).click();
  const order = await page.locator(".nav-more-menu").evaluate((menu) => [...menu.children].filter((child) => child.classList.contains("nav-more-section")).map((section) => [...section.classList].find((name) => name !== "nav-more-section")));
  expect(order).toEqual(["nav-more-account", "nav-more-actions", "nav-more-library", "nav-personal-more"]);
  await expect(page.locator(".nav-more-library")).toBeHidden();
  await expect(page.locator(".nav-personal-more")).toBeVisible();
  const surface = await page.locator(".nav-more-menu").evaluate((menu) => {
    const style = getComputedStyle(menu);
    return { backgroundColor: style.backgroundColor, animationName: style.animationName, opacity: style.opacity };
  });
  expect(surface.opacity).toBe("1");
  expect(surface.backgroundColor).not.toBe("rgba(0, 0, 0, 0)");
  await page.screenshot({ path: testInfo.outputPath("390-more-menu-library-at-bottom.png"), fullPage: true });
});

test("Owner can edit Main navigation from desktop and compact layouts", async ({ page }, testInfo) => {
  await login(page);
  for (const viewport of [{ width: 1440, height: 900 }, { width: 1101, height: 768 }, { width: 1024, height: 768 }, { width: 720, height: 450 }, { width: 390, height: 844 }, { width: 320, height: 800 }]) {
    await page.setViewportSize(viewport);
    await page.goto("/");
    const navigation = page.getByRole("navigation", { name: "Main navigation" });
    if (viewport.width > 1100) {
      await expect(page.locator(".desktop-sidebar")).toBeVisible();
      await expect(navigation.getByRole("heading")).toHaveText(["Library", "Your library", "Library views"]);
      await expect(navigation.getByRole("link", { name: "Home" })).toHaveAttribute("aria-current", "page");
      await expect(page.locator(".app-header .header-actions > .header-supporter")).toBeVisible();
      expect(await page.evaluate(() => document.documentElement.scrollWidth), `horizontal overflow at ${viewport.width}px`).toBeLessThanOrEqual(viewport.width);
      const edit = navigation.getByRole("link", { name: "Edit navigation" });
      await expect(edit).toBeVisible();
      await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-main-navigation.png`) });
      await edit.click();
      await expect(page).toHaveURL(/\/settings#navigation$/);
      await expect(page.getByRole("heading", { name: "Navigation", exact: true })).toBeVisible();
      await expect(page.getByLabel("Show Movies", { exact: true })).toBeChecked();
      await expect(page.getByRole("button", { name: "Move Movies down", exact: true })).toBeVisible();
      await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-navigation-editor.png`), fullPage: true });
      continue;
    }
    await navigation.getByText("More", { exact: true }).click();
    if (viewport.width < 901) {
      await expect(page.locator(".title-jump-scrubbable .letter-jump")).toBeHidden();
    }
    const support = navigation.locator(".nav-more-actions .nav-supporter:visible");
    await expect(support).toBeVisible();
    const edit = navigation.locator(".nav-edit-menu");
    await expect(edit).toBeVisible();
    if (viewport.width >= 901) {
			await expect(navigation.locator(".nav-main-overflow:visible")).toHaveCount(0);
    }
    await edit.scrollIntoViewIfNeeded();
    await expect(edit).toBeInViewport();
    await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-main-navigation.png`) });
    await edit.click();
    await expect(page).toHaveURL(/\/settings#navigation$/);
    await expect(page.getByRole("heading", { name: "Navigation", exact: true })).toBeVisible();
    await expect(page.getByLabel("Show Movies", { exact: true })).toBeChecked();
    await expect(page.getByRole("button", { name: "Move Movies down", exact: true })).toBeVisible();
    await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-navigation-editor.png`), fullPage: true });
  }
});

test("Community edition keeps Support Kinosail reachable", async ({ page }, testInfo) => {
  await login(page);
  for (const viewport of [{ width: 1440, height: 900 }, { width: 1024, height: 768 }, { width: 390, height: 844 }, { width: 320, height: 800 }]) {
    await page.setViewportSize(viewport);
    await page.goto("/");
    const support = page.locator(".header-supporter:visible");
    const beta = page.locator(".app-header .web-beta-badge");
    const name = page.locator(".app-header .brand-lockup h1");
    await expect(beta).toBeVisible();
    await expect(beta).toHaveText("Beta");
    await expect(beta).toBeInViewport();
    await expect(name).toBeVisible();
    await expect(support).toHaveCount(1);
    await expect(support).toHaveText("Support Kinosail");
    await expect(support).toHaveAttribute("href", "/supporter");
    await expect(support.locator("img")).toHaveCount(0);
    await expect(support).not.toHaveClass(/supporter-trio/);
    if (viewport.width <= 900) {
      const betaBox = await beta.boundingBox();
      const searchBox = await page.locator('.app-header .search input[type="search"]').boundingBox();
      const supportBox = await support.boundingBox();
      expect(betaBox && supportBox && betaBox.x + betaBox.width <= supportBox.x).toBe(true);
      if (viewport.width <= 430) {
        const nameBox = await name.boundingBox();
        expect(searchBox && nameBox && searchBox.y >= nameBox.y + nameBox.height).toBe(true);
      } else {
        expect(searchBox && supportBox && searchBox.x + searchBox.width <= supportBox.x).toBe(true);
      }
    }
    await page.screenshot({ path: testInfo.outputPath(`community-header-${viewport.width}.png`) });
    await support.focus();
    await expect(support).toBeFocused();
  }
});

test("supporter collection shows valid badges in the header", async ({ page }, testInfo) => {
  await page.route("**/api/v1/supporter/collection", (route) => route.fulfill({
    status: 200, contentType: "application/json",
    body: JSON.stringify({ display: "automatic", badges: [{ family: "living-standard", name: "Legacy", rank: 10 }] }),
  }));
  await login(page);
  for (const viewport of [{ width: 1440, height: 900 }, { width: 390, height: 844 }]) {
    await page.setViewportSize(viewport);
    await page.goto("/");
    const support = page.locator(".header-supporter:visible");
    await expect(support).toHaveAttribute("aria-label", "Your supporter collection");
    await expect(support).toHaveClass(/supporter-trio/);
    await expect(support.locator("img")).toHaveAttribute("alt", "Legacy · Legacy recurring");
    await expect(support).toHaveAttribute("href", "/supporter");
    expect((await new AxeBuilder({ page }).include(".app-header").analyze()).violations).toEqual([]);
    await page.screenshot({ path: testInfo.outputPath(`supporter-header-${viewport.width}.png`) });
  }
});

test("clearing a supporter collection removes badge styling", async ({ page }) => {
  let badges: object[] = [{ family: "living-standard", name: "Legacy", rank: 10 }];
  await page.route("**/api/v1/supporter/collection", (route) => route.fulfill({
    status: 200, contentType: "application/json", body: JSON.stringify({ display: "automatic", badges }),
  }));
  await login(page);
  await page.goto("/");
  const support = page.locator(".header-supporter:visible");
  await expect(support).toHaveClass(/supporter-trio/);
  badges = [];
  await page.evaluate(() => (window as Window & { bindSupporterRecognition: () => Promise<void> }).bindSupporterRecognition());
  await expect(support).toHaveText("Support Kinosail");
  await expect(support).not.toHaveClass(/supporter-trio/);
  await expect(support.locator("img")).toHaveCount(0);
});

test("invalid supporter levels do not render a badge", async ({ page }) => {
  await page.route("**/api/v1/supporter/collection", (route) => route.fulfill({
    status: 200, contentType: "application/json",
    body: JSON.stringify({ display: "automatic", badges: [{ family: "living-standard", name: "Forged", rank: 11 }] }),
  }));
  await login(page);
  await page.goto("/");
  const support = page.locator(".header-supporter:visible");
  await expect(support).toHaveText("Support Kinosail");
  await expect(support.locator("img")).toHaveCount(0);
  await expect(support).not.toHaveClass(/supporter-trio/);
});
