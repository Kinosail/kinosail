import AxeBuilder from "@axe-core/playwright";
import { createHash } from "node:crypto";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { expect, test } from "@playwright/test";
import { createViewer, firstPlayable, login, loginViewer, newViewerPage, openLibrarySection, removeViewer, type OfflineClient } from "./test-instance-helpers";

export function registerTestInstanceShellTests() {
test("Connection choices stay optional and secure by default", async ({ page }, testInfo) => {
	await login(page);
	for (const viewport of [{ width: 1440, height: 900 }, { width: 390, height: 844 }, { width: 320, height: 800 }]) {
		await page.setViewportSize(viewport);
		await page.goto("/settings#access");
		await page.getByRole("link", { name: "Access", exact: true }).click();
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
  expect(surface).toMatchObject({ animationName: "kinosail-panel-enter", opacity: "1" });
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
    await new Promise((resolve) => setTimeout(resolve, 750));
    await route.continue();
  });
  await login(page);
  for (const viewport of [{ width: 1440, height: 900 }, { width: 1024, height: 768 }, { width: 390, height: 844 }, { width: 320, height: 800 }]) {
    await page.setViewportSize(viewport);
    await page.goto("/?view=movies", { waitUntil: "domcontentloaded" });
    const pending = page.locator(".supporter-signature");
    await expect(pending).toHaveCount(1);
    await expect(pending).toBeHidden();
    const before = await page.locator("#library").evaluate((element) => element.getBoundingClientRect().top);
    await expect(page.getByRole("complementary", { name: "Kinosail supporter status" })).toBeVisible();
    const after = await page.locator("#library").evaluate((element) => element.getBoundingClientRect().top);
    expect(after, `supporter signature insertion shifted library content at ${viewport.width}px`).toBe(before);
  }
});

test("Supporter recognition clears stale active chrome after status changes", async ({ page }) => {
  let status = {
    livingStandard: { active: true, archived: false, rank: 10, title: "Living Standard", name: "Legacy" },
    patronOrder: { active: false, archived: false },
  };
  await page.route("**/api/v1/supporter", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(status) }));
  await login(page);
  await expect(page.locator("body")).toHaveAttribute("data-supporter-rank", "10");
  await expect(page.locator(".supporter-edition-mark")).toHaveText("Living Standard · Legacy");

  status = {
    livingStandard: { active: false, archived: true, rank: 10, title: "Living Standard", name: "Legacy" },
    patronOrder: { active: false, archived: false },
  };
  await page.evaluate(() => document.dispatchEvent(new Event("htmx:afterSwap")));
  await expect(page.locator("body")).not.toHaveAttribute("data-supporter-rank", /.+/);
  await expect(page.locator(".supporter-edition-mark")).toHaveCount(0);
  await expect(page.getByRole("complementary", { name: "Kinosail supporter status" })).toContainText("Living Standard archived.");

  status = {
    livingStandard: { active: false, archived: false, rank: 0, title: "Living Standard", name: "Free" },
    patronOrder: { active: false, archived: false },
  };
  await page.evaluate(() => document.dispatchEvent(new Event("htmx:afterSwap")));
  await expect(page.getByRole("complementary", { name: "Kinosail supporter status" })).toContainText("Free and supporter-funded.");
});

test("Supporter page presents twenty distinct first-class app badges", async ({ page }, testInfo) => {
  await login(page);
  for (const viewport of [{ width: 1440, height: 900 }, { width: 1024, height: 768 }, { width: 720, height: 450 }, { width: 390, height: 844 }, { width: 320, height: 800 }]) {
    await page.setViewportSize(viewport);
    await page.goto("/supporter");
    await expect(page.getByRole("heading", { name: "Two badge families. Every level has its own shape." })).toBeVisible();
    await expect(page.getByText("Kinosail Subtitles keeps every core feature.")).toBeVisible();
    await expect(page.getByRole("heading", { name: "Living Standards" })).toBeVisible();
    await expect(page.getByRole("heading", { name: "Patron Orders" })).toBeVisible();
    await expect(page.getByRole("heading", { name: "One key covers every included app." })).toBeVisible();
    await expect(page.getByText("Each app keeps its own emblem and Share Certificate. Living follows current apps while active. Dated Patron editions stay fixed.")).toBeVisible();
    await expect(page.getByRole("link", { name: "Choose monthly support" })).toHaveAttribute("rel", "external noreferrer");
    await expect(page.getByRole("link", { name: "Choose one-time support" })).toHaveAttribute("rel", "external noreferrer");
    const living = page.locator(".family-living-standard");
    const patron = page.locator(".family-patron-order");
    expect(await page.evaluate(() => {
      const livingFamily = document.querySelector(".family-living-standard")!;
      const patronFamily = document.querySelector(".family-patron-order")!;
      return Boolean(livingFamily.compareDocumentPosition(patronFamily) & Node.DOCUMENT_POSITION_FOLLOWING);
    })).toBe(true);
    await expect(living.locator(".badge-level")).toHaveCount(10);
    await expect(patron.locator(".badge-level")).toHaveCount(10);
    await expect(page.locator(".badge-level.current, .badge-level.is-active")).toHaveCount(0);
    await expect(page.getByText("Available", { exact: true })).toHaveCount(12);
    await expect(page.getByText("Elite level · Available", { exact: true })).toHaveCount(8);
    for (const family of [living, patron]) {
      const outlines = await family.locator(".badge-outline").evaluateAll((elements) => elements.map((element) => element.getAttribute("d")));
      expect(new Set(outlines).size).toBe(10);
      await expect(family.locator('.badge-level[data-rank="7"].elite, .badge-level[data-rank="8"].elite, .badge-level[data-rank="9"].elite, .badge-level[data-rank="10"].elite')).toHaveCount(4);
    }
    await expect(page.locator(".badge-sigil title", { hasText: "badge for Kinosail Subtitles" })).toHaveCount(20);
    const expectedColumns = viewport.width <= 340 ? 1 : viewport.width <= 700 ? 2 : viewport.width <= 1000 ? 3 : 5;
    const columns = await living.locator(".badge-gallery").evaluate((element) => getComputedStyle(element).gridTemplateColumns.split(" ").length);
    expect(columns).toBe(expectedColumns);
    await expect(page.getByRole("link", { name: "Public supporters" })).toHaveAttribute("rel", "external noreferrer");
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    const accessibility = await new AxeBuilder({ page }).include("main").analyze();
    expect(accessibility.violations).toEqual([]);
    await page.getByLabel("Supporter key").focus();
    await page.keyboard.press("Tab");
    await expect(page.getByLabel("Public recognition name")).toBeFocused();
    await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-all-supporter-badges.png`), fullPage: true });
  }
  await page.setViewportSize({ width: 390, height: 844 });
  await page.emulateMedia({ colorScheme: "dark", reducedMotion: "reduce" });
  await page.evaluate(() => localStorage.removeItem("kinosail-theme"));
  await page.goto("/supporter");
  await expect(page.locator(".badge-level")).toHaveCount(20);
  await page.screenshot({ path: testInfo.outputPath("390-supporter-badges-dark-reduced-motion.png"), fullPage: true });
  await page.emulateMedia({ forcedColors: "active" });
  expect(await page.locator('.badge-level[data-rank="10"] .badge-outline').first().evaluate((element) => getComputedStyle(element).stroke === getComputedStyle(document.body).color)).toBe(true);
  await page.screenshot({ path: testInfo.outputPath("390-supporter-badges-forced-colors.png"), fullPage: true });
});
}
