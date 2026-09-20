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
		expect(headings.slice(firstChoice, firstChoice + 3)).toEqual(["Secure local access", "Jellyfin apps", "Trusted HTTPS for phones, TVs, and Jellyfin apps"]);
		await expect(page.getByText("On by default.", { exact: true })).toBeVisible();
		await expect(page.getByLabel("Allow compatible Jellyfin apps to connect")).not.toBeChecked();
		await expect(page.locator("#jellyfin")).toContainText("Trusted HTTPS is required.");
		await expect(page.locator("#jellyfin")).toContainText("Set up Trusted HTTPS first.");
		await expect(page.getByRole("heading", { name: "Trusted HTTPS for phones, TVs, and Jellyfin apps" })).toBeVisible();
		await expect(page.locator("#trusted-https p").first()).toContainText("Required for Jellyfin apps. Recommended for phones and TVs.");
		expect(await page.locator("main").evaluate((element) => element.scrollWidth <= element.clientWidth)).toBe(true);
		const accessibility = await new AxeBuilder({ page }).include("main").analyze();
		expect(accessibility.violations).toEqual([]);
		await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-secure-connection-choices.png`), fullPage: true });
	}
});

test("Mobile More menu puts library shortcuts at the bottom", async ({ page }, testInfo) => {
  await page.addInitScript(() => localStorage.setItem("kinosail-theme", "dark"));
  await login(page);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/");
  await page.getByRole("navigation", { name: "Main navigation" }).getByText("More", { exact: true }).click();
  const order = await page.locator(".nav-more-menu").evaluate((menu) => [...menu.children].filter((child) => child.classList.contains("nav-more-section")).map((section) => [...section.classList].find((name) => name !== "nav-more-section")));
  expect(order).toEqual(["nav-more-account", "nav-more-actions", "nav-more-library"]);
  await expect(page.locator(".nav-more-library")).toBeVisible();
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
  for (const viewport of [{ width: 1440, height: 900 }, { width: 1024, height: 768 }, { width: 720, height: 450 }, { width: 390, height: 844 }, { width: 320, height: 800 }]) {
    await page.setViewportSize(viewport);
    await page.goto("/");
    const navigation = page.getByRole("navigation", { name: "Main navigation" });
    if (viewport.width < 901) {
      await navigation.getByText("More", { exact: true }).click();
      await expect(page.locator(".title-jump-scrubbable .letter-jump")).toBeHidden();
    }
    const support = navigation.locator(viewport.width >= 901 ? ".nav-main-supporter:visible" : ".nav-more-actions .nav-supporter:visible");
    await expect(support).toBeVisible();
    const edit = navigation.locator(viewport.width >= 901 ? ".nav-main-edit" : ".nav-edit-menu");
    await expect(edit).toBeVisible();
    if (viewport.width >= 901) {
			await expect(navigation.locator(".nav-main-overflow:visible")).toHaveCount(8);
      await expect(navigation.getByText("More", { exact: true })).toBeHidden();
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

test("Community edition keeps a quiet permanent supporter signature", async ({ page }, testInfo) => {
  await login(page);
  await page.evaluate(() => localStorage.setItem("kinosail-supporter-reminder:free", "dismissed"));
  for (const viewport of [{ width: 1440, height: 900 }, { width: 1024, height: 768 }, { width: 720, height: 450 }, { width: 390, height: 844 }, { width: 320, height: 800 }]) {
    await page.setViewportSize(viewport);
    await page.goto("/?view=movies");
    const masthead = page.locator(".library-masthead");
    const signature = masthead.getByRole("complementary", { name: "Kinosail supporter status" });
    await expect(signature).toContainText("Community edition · Free and supporter-funded.");
    await expect(signature.getByRole("link", { name: "Support Kinosail" })).toHaveAttribute("href", "/supporter");
    await expect(signature.getByRole("button")).toHaveCount(0);
    expect(await signature.evaluate((element) => getComputedStyle(element).position)).toBe("static");
    expect(await signature.locator("p").evaluate((element) => parseFloat(getComputedStyle(element).fontSize))).toBeLessThanOrEqual(12);
    const mastheadBox = await masthead.boundingBox();
    const signatureBox = await signature.boundingBox();
    const copyBox = await signature.locator("p").boundingBox();
    const summaryBox = await masthead.locator(":scope>p").boundingBox();
    expect(mastheadBox).not.toBeNull();
    expect(signatureBox).not.toBeNull();
    expect(copyBox).not.toBeNull();
    expect(summaryBox).not.toBeNull();
    expect(signatureBox!.x).toBeGreaterThanOrEqual(mastheadBox!.x);
    expect(signatureBox!.x + signatureBox!.width).toBeLessThanOrEqual(mastheadBox!.x + mastheadBox!.width);
    if (viewport.width > 900) {
      expect(signatureBox!.x).toBeGreaterThan(mastheadBox!.x + mastheadBox!.width / 2);
      expect(signatureBox!.y).toBeLessThanOrEqual(summaryBox!.y);
    } else {
      expect(signatureBox!.x).toBeLessThan(mastheadBox!.x + 40);
      expect(copyBox!.x).toBeLessThan(mastheadBox!.x + 40);
      expect(signatureBox!.y).toBeGreaterThanOrEqual(summaryBox!.y + summaryBox!.height);
    }
    await page.screenshot({ path: testInfo.outputPath(`community-edition-signature-${viewport.width}.png`), fullPage: true });
  }
});

test("Community signature does not shift library content after navigation", async ({ page }) => {
  await page.route("**/api/v1/supporter", async (route) => {
    await new Promise((resolve) => setTimeout(resolve, 250));
    await route.continue();
  });
  await login(page);
  for (const viewport of [{ width: 1440, height: 900 }, { width: 1024, height: 768 }, { width: 390, height: 844 }, { width: 320, height: 800 }]) {
    await page.setViewportSize(viewport);
    await page.goto("/?view=movies", { waitUntil: "domcontentloaded" });
    const before = await page.locator("#library").evaluate((element) => element.getBoundingClientRect().top);
    await expect(page.getByRole("complementary", { name: "Kinosail supporter status" })).toBeVisible();
    const after = await page.locator("#library").evaluate((element) => element.getBoundingClientRect().top);
    expect(after, `supporter signature insertion shifted library content at ${viewport.width}px`).toBe(before);
  }
});

test.describe("active supporter signature", () => {
  test.use({ serviceWorkers: "block" });
  test("shows the title and level mark", async ({ page }, testInfo) => {
    await page.route("**/api/v1/supporter", async (route) => route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ active: true, livingStandard: { family: "living-standard", name: "Legacy", rank: 10, active: true } }),
    }));
    await login(page);
    for (const viewport of [{ width: 1440, height: 900 }, { width: 1024, height: 768 }, { width: 720, height: 450 }, { width: 390, height: 844 }, { width: 320, height: 800 }]) {
      await page.setViewportSize(viewport);
      await page.goto("/?view=movies");
      const signature = page.getByRole("complementary", { name: "Kinosail supporter status" });
      await expect(signature).toContainText("Legacy Living Standard");
      await expect(signature.locator('.supporter-level-logo[data-rank="10"][data-family="living-standard"]')).toBeVisible();
      await expect(signature.getByRole("link", { name: "Legacy Living Standard supporter passport" })).toHaveAttribute("href", "/supporter");
      await expect(page.locator(".brand-lockup .supporter-edition-mark")).toHaveCount(0);
      expect(await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBe(0);
      if (viewport.width <= 900) {
        const logoBox = await signature.locator(".supporter-level-logo").boundingBox();
        const copyBox = await signature.locator("p").boundingBox();
        const linkBox = await signature.getByRole("link").boundingBox();
        expect(logoBox).not.toBeNull();
        expect(copyBox).not.toBeNull();
        expect(linkBox).not.toBeNull();
        expect(linkBox!.x).toBeCloseTo(copyBox!.x, 0);
        // Firefox rounds adjacent box edges by fractions of a CSS pixel.
        expect(linkBox!.y).toBeGreaterThanOrEqual(copyBox!.y + copyBox!.height - 0.01);
        expect(logoBox!.x + logoBox!.width).toBeLessThanOrEqual(copyBox!.x);
      }
      await page.screenshot({ path: testInfo.outputPath(`active-supporter-signature-${viewport.width}.png`), fullPage: true });
    }
  });

	test("removes generated chrome after an active badge expires", async ({ page }) => {
		let status: Record<string, object | string | number | boolean | null> = { active: true, livingStandard: { family: "living-standard", name: "Legacy", rank: 10, active: true } };
		await page.route("**/api/v1/supporter", async (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(status) }));
		await login(page);
		await page.goto("/settings");
		await expect(page.locator(".supporter-edition-mark")).toHaveCount(1);
		await expect(page.locator("body")).toHaveAttribute("data-supporter-rank", "10");
		await page.goto("/?view=movies");
		const signature = page.getByRole("complementary", { name: "Kinosail supporter status" });
		await expect(signature).toHaveClass(/is-active/);
		status = { active: false, subscriptionActive: false, livingStandard: { family: "living-standard", name: "Legacy", rank: 10, active: false, expired: true } };
		await page.evaluate(() => (window as Window & { bindSupporterRecognition: () => Promise<void> }).bindSupporterRecognition());
		await expect(signature).not.toHaveClass(/is-active/);
		await expect(signature).toContainText("Community edition · Living Standard archived.");
		await expect(signature.locator(".supporter-level-logo")).toHaveCount(0);
		await expect(page.locator("body")).not.toHaveAttribute("data-supporter-rank", /.+/);
	});
});

test("Invalid supporter levels keep the community signature", async ({ page }) => {
  await page.route("**/api/v1/supporter", async (route) => route.fulfill({
    status: 200,
    contentType: "application/json",
    body: JSON.stringify({ active: true, livingStandard: { family: "living-standard", name: "Forged", rank: 11, active: true } }),
  }));
  await login(page);
  await page.goto("/?view=movies");
  const signature = page.getByRole("complementary", { name: "Kinosail supporter status" });
  await expect(signature).toContainText("Community edition · Free and supporter-funded.");
  await expect(signature.locator(".supporter-level-logo")).toHaveCount(0);
  await expect(signature).not.toContainText("Forged");
});
