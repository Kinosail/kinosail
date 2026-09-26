import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { compactViewports, expectNoHorizontalOverflow, expectSkipLinkOffscreen, initiallyOccludedTargets, occludedTargets, setSubtitleLanguages, supportedViewports } from "./subtitle-dashboard-helpers";

export function registerSubtitleLayoutTests() {
test("Settings header keeps desktop destinations in one compact row", async ({ page }) => {
  for (const width of [1920, 1280, 1024, 901]) {
    await page.setViewportSize({ width, height: 900 });
    await page.goto("/settings#provider");
    const layout = await page.locator(".app-header").evaluate((header) => {
      const nav = header.querySelector('nav[aria-label="Main navigation"]');
      const links = [...nav.querySelectorAll(":scope > a")].map((link) => link.getBoundingClientRect());
      return { header: header.getBoundingClientRect().toJSON(), nav: nav.getBoundingClientRect().toJSON(), links: links.map((box) => box.toJSON()) };
    });
    expect(layout.header.height, `${width}px header height`).toBeLessThanOrEqual(100);
    expect(layout.links).toHaveLength(3);
    expect(layout.links.every((link) => Math.abs(link.top - layout.links[0].top) <= 1), `${width}px navigation row`).toBe(true);
    expect(layout.links.every((link) => link.left >= layout.nav.left && link.right <= layout.nav.right), `${width}px navigation bounds`).toBe(true);
    await expectNoHorizontalOverflow(page);
  }
});

test("Subtitle API reports the rendered inventory and rejects ambiguous input", async ({ page }) => {
  const inventory = await page.evaluate(async () => {
    const response = await fetch("/api/v1/subtitle-library?view=library");
    return { status: response.status, body: await response.json() };
  });
  expect(inventory.status).toBe(200);
  expect(inventory.body.total).toBeGreaterThan(0);
  expect(inventory.body.ready + inventory.body.wanted + inventory.body.pending + inventory.body.unavailable).toBe(inventory.body.total);
  expect(inventory.body.items).toHaveLength(Math.min(40, inventory.body.total));

  const malformed = await page.evaluate(async () => {
    const csrf = document.querySelector<HTMLMetaElement>('meta[name="kinosail-csrf"]')?.content ?? "";
    const response = await fetch("/api/v1/subtitle-library/maintain", {
      method: "POST",
      headers: { "Content-Type": "application/json", "X-Kinosail-CSRF": csrf },
      body: '{"language":"en","Language":"fr"}',
    });
    return response.status;
  });
  expect(malformed).toBe(400);
  await page.goto("/?view=wanted&view=library");
	await expect(page.getByText("Request could not be completed.", { exact: true })).toBeVisible();
});

test("Overview preview keeps the complete wanted inventory accessible", async ({ page }) => {
  const inventory = await page.evaluate(async () => (await fetch("/api/v1/subtitle-library?view=wanted")).json());
  const rows = page.locator(".subtitle-file");
  await expect(rows).toHaveCount(Math.min(6, inventory.matched));
  const allWanted = page.getByRole("link", { name: /^View all wanted/ });
  await expect(allWanted).toHaveCount(1);
  await allWanted.click();
  await expect(rows).toHaveCount(inventory.items.length);
});

test("Overview search finds covered titles and offers a clear recovery", async ({ page }) => {
  await page.goto("/");
  const search = page.getByRole("searchbox", { name: "Search subtitle library" });
  await search.fill("Arrival");
  await search.press("Enter");
  await expect(page).toHaveURL(/view=library/);
  expect([...new URL(page.url()).searchParams.keys()].sort()).toEqual(["q", "view"]);
  await expect(page.getByRole("heading", { name: "Search results" })).toBeVisible();
  await expect(page.locator('.subtitle-file')).toHaveCount(1);
  await page.getByRole("link", { name: "Clear filters", exact: true }).click();
  await expect(page).toHaveURL("/?view=library");
  await expect(search).toHaveValue("");
  await page.getByRole("link", { name: "Wanted", exact: true }).click();
  await expect(page).toHaveURL("/?view=wanted");
  await expect(search).toHaveValue("");
  await search.fill("No matching fixture title");
  await search.press("Enter");
  await expect(page).toHaveURL(url => url.searchParams.get("view") === "wanted" && url.searchParams.get("q") === "No matching fixture title");
  await expect(page.getByRole("heading", { name: "No files match." })).toBeVisible();
  await page.getByRole("link", { name: "Clear filters", exact: true }).click();
  await expect(page).toHaveURL("/?view=wanted");
});

test("Dashboard stays readable and accessible at every supported width", async ({ page }, testInfo) => {
  for (const viewport of supportedViewports) {
    await page.setViewportSize(viewport);
    await page.goto("/");
    await expectNoHorizontalOverflow(page);
    await expect(page.getByRole("navigation", { name: "Main navigation" })).toBeVisible();
    await expect(page.getByRole("link", { name: "Settings", exact: true })).toBeVisible();
    for (const link of await page.locator(".app-header nav > a").all()) {
      expect(await link.evaluate((element) => getComputedStyle(element, "::before").maskImage), await link.innerText()).not.toBe("none");
    }
    if (viewport.width > 390 && viewport.height <= 600) {
      const collisions = await page.evaluate(() => {
        const search = document.querySelector(".subtitle-filters")?.getBoundingClientRect();
        if (!search) return ["search is missing"];
        return [".subtitle-overview", ".app-header nav"].filter((selector) => {
          const target = document.querySelector(selector)?.getBoundingClientRect();
          return target && search.left < target.right && search.right > target.left && search.top < target.bottom && search.bottom > target.top;
        });
      });
      expect(collisions).toEqual([]);
    }
    if (viewport.width <= 390) {
      const alignment = await page.locator(".subtitle-coverage-stat").evaluate((element) => {
        const parent = element.getBoundingClientRect();
        const visible = [...element.querySelectorAll("p, .subtitle-text-link")].map((child) => child.getBoundingClientRect());
        return visible.length > 0 && visible.every((box) => box.width > 0 && box.left >= parent.left && box.right <= parent.right);
      });
      expect(alignment).toBe(true);
      await expect(page.locator(".subtitle-coverage-stat > p")).toContainText(/\d+ of \d+ files ready/);
      await expect(page.locator(".subtitle-coverage-stat meter")).toBeHidden();
      expect(await occludedTargets(page, [".subtitle-coverage-stat > p", ".subtitle-overview-actions :is(button, a.button)"], [".app-header nav"])).toEqual([]);
    }
    const accessibility = await new AxeBuilder({ page }).include("main").analyze();
    expect(accessibility.violations).toEqual([]);
    await expectSkipLinkOffscreen(page);
    await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-subtitle-dashboard.png`), fullPage: true });
  }
});

test("Dashboard preserves keyboard and high-contrast operation", async ({ page }, testInfo) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/");
  const skip = page.getByRole("link", { name: "Skip to content" });
  await skip.focus();
  await expect(skip).toBeFocused();
  await page.keyboard.press("Enter");
  await expect(page.locator("main")).toBeFocused();
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.screenshot({ path: testInfo.outputPath("subtitle-dashboard-reduced-motion.png") });
  await page.emulateMedia({ forcedColors: "active" });
  await expect(page.locator(".subtitle-coverage-stat > p")).toBeVisible();
  await expect(page.locator(".subtitle-coverage-stat > p")).toContainText(/\d+ of \d+ files ready/);
  await page.screenshot({ path: testInfo.outputPath("subtitle-dashboard-forced-colors.png") });
});

test("Compact navigation does not cover the current subtitle task", async ({ page }) => {
  const failures: string[] = [];
  for (const viewport of compactViewports) {
    await page.setViewportSize(viewport);
    await page.goto("/");
    if (viewport.height <= 600) {
      failures.push(...(await initiallyOccludedTargets(page, [".subtitle-overview-copy h2", ".subtitle-coverage-stat > p", ".subtitle-overview-actions :is(button, a.button)"], [".app-header nav"])).map((failure) => `${viewport.width}px dashboard: ${failure}`));
    }
    failures.push(...(await occludedTargets(page, [".subtitle-coverage-stat > p", ".subtitle-overview-copy h2", ".subtitle-overview-actions :is(button, a.button)", ".subtitle-system > summary"], [".app-header nav"])).map((failure) => `${viewport.width}px dashboard: ${failure}`));

    await page.goto("/settings#provider");
    failures.push(...(await occludedTargets(page, ["#provider h2", "#provider input[name=apiKey]", "#provider form[action='/settings/subtitles/subsource'] button"], [".app-header nav", ".search", ".settings-nav"])).map((failure) => `${viewport.width}px provider settings: ${failure}`));
    const skip = await page.locator(".skip").boundingBox();
    if (!skip || skip.y + skip.height > 0) failures.push(`${viewport.width}px settings: skip link is visible`);

    await page.goto("/settings#trusted-https");
    failures.push(...(await occludedTargets(page, ["#trusted-https input[name=provider]", "#trusted-https input[name=domain]", "#trusted-https form[action='/settings/trusted-https'] button"], [".app-header nav", ".search", ".settings-nav"])).map((failure) => `${viewport.width}px trusted HTTPS: ${failure}`));
  }
  expect(failures).toEqual([]);
});

test("Dashboard preserves its task at 200 percent reflow", async ({ page }) => {
  await page.setViewportSize({ width: 360, height: 450 });
  await page.goto("/");
  expect(await page.locator("html").evaluate((element) => element.scrollWidth <= element.clientWidth)).toBe(true);
  expect(await occludedTargets(page, [".subtitle-coverage-stat > p", ".subtitle-overview-copy h2", ".subtitle-overview-actions :is(button, a.button)", ".subtitle-system > summary"], [".app-header nav"])).toEqual([]);
});

test("Readiness, quota, and history remain usable on compact screens", async ({ page }, testInfo) => {
  for (const viewport of [{ width: 1024, height: 768 }, { width: 390, height: 844 }, { width: 320, height: 800 }]) {
    await page.setViewportSize(viewport);
    await page.goto("/");
    const system = page.locator(".subtitle-system");
    if (await system.getAttribute("open") === null) await system.locator(":scope > summary").click();
    await expect(page.getByRole("heading", { name: "Server readiness" })).toBeVisible();
    await expect(page.getByRole("heading", { name: "Subtitle sources" })).toBeVisible();
    await expectNoHorizontalOverflow(page);
    expect((await new AxeBuilder({ page }).include("main").analyze()).violations).toEqual([]);
    await page.getByRole("link", { name: "Library", exact: true }).click();
    await expect(page.getByRole("heading", { name: "Library", exact: true })).toBeVisible();
    const firstHistory = page.locator(".subtitle-file").first();
    await firstHistory.locator(":scope > summary").click();
    await expect(firstHistory).toHaveAttribute("open", "");
    await expect(firstHistory.getByText("Coverage", { exact: true })).toBeVisible();
    await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-subtitle-readiness-history.png`), fullPage: true });
  }
});

test("Subtitle setup guide keeps readiness and recovery visible", async ({ page }, testInfo) => {
  for (const viewport of supportedViewports) {
    await page.setViewportSize(viewport);
    await page.goto("/onboarding/connection");
    await expect(page.getByRole("heading", { name: "Connect a provider. Let Kinosail handle the rest." })).toBeVisible();
    await expect(page.getByRole("navigation", { name: "Setup progress" }).getByText("Subtitle plan")).toHaveAttribute("aria-current", "step");
    await page.getByText("Review language, media scope, and schedule", { exact: true }).click();
    const primaryLanguage = page.getByLabel("Primary language");
    await expect(primaryLanguage).toHaveValue("en");
    expect(await primaryLanguage.locator("option").count()).toBeGreaterThan(187);
    await expect(primaryLanguage.locator("option:checked")).toContainText("English (en)");
    await expect(page.getByRole("group", { name: "Preferred subtitle role" }).locator('input[value="standard"]')).toBeChecked();
    await expect(page.locator("#libraries")).toContainText(/Movies|Entire media mount/);
    await expect(page.getByRole("button", { name: /^Remove/ })).toHaveCount(0);
    const providers = page.locator("#providers");
    await expect(providers).toContainText("Connect a subtitle provider");
    await expect(providers.getByRole("link", { name: "Provider account" }).nth(0)).toHaveAttribute("href", "https://subdl.com/panel");
    await expect(providers.getByRole("link", { name: "Provider account" }).nth(1)).toHaveAttribute("href", "https://dl.opensubtitles.com/en/users/sign_in");
    await expect(providers.getByRole("link", { name: "Provider account" }).nth(2)).toHaveAttribute("href", "https://subsource.net/");
    await expect(page.getByRole("complementary", { name: "Your subtitle plan" }).getByText("Not configured", { exact: true })).toBeVisible();
    await expect(page.getByRole("link", { name: "Finish and open overview" })).toBeVisible();
    await expectNoHorizontalOverflow(page);
    expect((await new AxeBuilder({ page }).include("main").analyze()).violations).toEqual([]);
    await expectSkipLinkOffscreen(page);
    await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-subtitle-setup.png`), fullPage: true });
  }

  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/onboarding/connection");
  await page.getByText("Review language, media scope, and schedule", { exact: true }).click();
  await page.getByLabel("Primary language").focus();
  await expect(page.getByLabel("Primary language")).toBeFocused();
  await page.emulateMedia({ reducedMotion: "reduce", forcedColors: "active" });
  await expect(page.getByRole("link", { name: "Finish and open overview" })).toBeVisible();
  await page.screenshot({ path: testInfo.outputPath("subtitle-setup-forced-colors.png"), fullPage: true });
});

test("Subtitle setup respects saved light and dark themes", async ({ page }) => {
  for (const theme of ["light", "dark"]) {
    await page.evaluate(value => localStorage.setItem("kinosail-theme", value), theme);
    await page.goto("/onboarding/connection");
    await expect(page.locator("html")).toHaveAttribute("data-theme", theme);
    await expect(page.locator("html")).toHaveCSS("--bg", theme === "dark" ? "#0b0d0b" : "#f4f8ef");
    expect((await new AxeBuilder({ page }).include("main").analyze()).violations).toEqual([]);
  }
});

}
