import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { compactViewports, expectNoHorizontalOverflow, expectSkipLinkOffscreen, initiallyOccludedTargets, occludedTargets, setSubtitleLanguages, supportedViewports } from "./subtitle-dashboard-helpers";

export function registerSubtitleLayoutTests() {
test("Subtitles navigation follows the Player shell at each breakpoint", async ({ page }, testInfo) => {
  for (const [width, height] of [[1440, 900], [1200, 900], [1024, 768], [390, 844]]) {
    await page.setViewportSize({ width, height });
    await page.goto("/");
    const shell = await page.evaluate(() => {
      const header = document.querySelector(".app-header")!.getBoundingClientRect();
      const nav = document.querySelector('.app-header nav[aria-label="Main navigation"]')!.getBoundingClientRect();
      const main = document.querySelector(".subtitle-main")!.getBoundingClientRect();
      return { header: { x: header.x, y: header.y, width: header.width, height: header.height }, nav: { x: nav.x, y: nav.y, width: nav.width, height: nav.height }, main: { x: main.x, y: main.y } };
    });
    expect(shell.header.x).toBe(0);
    expect(shell.header.width).toBe(width);
    if (width > 1100) {
      expect(shell.nav.y).toBeGreaterThanOrEqual(shell.header.height - 1);
      expect(shell.nav.x).toBe(0);
      expect(shell.nav.width).toBe(240);
      expect(shell.main.x).toBeGreaterThanOrEqual(shell.nav.width);
    } else if (width > 900) {
      expect(shell.nav.y).toBeGreaterThan(0);
      expect(shell.nav.y).toBeLessThan(shell.header.height);
      expect(shell.main.x).toBe(0);
    } else {
      expect(Math.round(shell.nav.y + shell.nav.height)).toBe(height);
      expect(shell.main.x).toBe(0);
    }
    await expect(page.getByRole("link", { name: "Settings", exact: true })).toBeVisible();
    await expectNoHorizontalOverflow(page);
    if (width !== 1024) await page.screenshot({ path: testInfo.outputPath(`subtitles-dashboard-${width}.png`) });
  }
});
test("phone settings expose every section without a hidden horizontal rail", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/settings");
  const sections = page.getByRole("navigation", { name: "Settings sections" });
  const layout = await sections.evaluate((nav) => ({
    scrollWidth: nav.scrollWidth,
    clientWidth: nav.clientWidth,
    links: [...nav.querySelectorAll("a")].map((link) => ({
      href: link.getAttribute("href"), left: link.getBoundingClientRect().left,
      right: link.getBoundingClientRect().right,
      top: link.getBoundingClientRect().top,
      bottom: link.getBoundingClientRect().bottom,
    })),
    bounds: nav.getBoundingClientRect().toJSON(),
  }));
  expect(layout.links.map((link) => link.href).join(" ")).toBe("#language #cleanup #provider #libraries #automation #appearance #security #account #thanks");
  expect(layout.scrollWidth).toBeLessThanOrEqual(layout.clientWidth + 1);
  expect(layout.links.every((link) => link.left >= layout.bounds.left - 1 && link.right <= layout.bounds.right + 1 && link.top >= layout.bounds.top - 1 && link.bottom <= layout.bounds.bottom + 1)).toBe(true);
});

test("Settings header keeps destinations inside the Player-style shell", async ({ page }) => {
  for (const width of [1920, 1280, 1024, 901]) {
    await page.setViewportSize({ width, height: 900 });
    await page.goto("/settings#provider");
    const layout = await page.locator(".app-header").evaluate((header) => {
      const nav = header.querySelector('nav[aria-label="Main navigation"]');
      const links = [...nav.querySelectorAll(":scope > a")].map((link) => link.getBoundingClientRect());
      return { header: header.getBoundingClientRect().toJSON(), nav: nav.getBoundingClientRect().toJSON(), links: links.map((box) => box.toJSON()) };
    });
    expect(layout.header.height, `${width}px header height`).toBeLessThanOrEqual(100);
    expect(layout.links).toHaveLength(4);
    if (width > 1100) {
      expect(layout.links.every((link) => link.top >= layout.header.bottom - 1), `${width}px navigation rail`).toBe(true);
      expect(layout.links[1].top).toBeGreaterThan(layout.links[0].top);
    } else {
      expect(layout.links.every((link) => Math.abs(link.top - layout.links[0].top) <= 1), `${width}px navigation row`).toBe(true);
    }
    expect(layout.links.every((link) => link.left >= layout.nav.left - 1 && link.right <= layout.nav.right + 1), `${width}px navigation bounds`).toBe(true);
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
    if (viewport.width === 1440 || viewport.width === 390) {
      await page.screenshot({ path: testInfo.outputPath(`${viewport.width}-subtitle-dashboard-viewport.png`) });
    }
  }
  await page.setViewportSize({ width: 1200, height: 630 });
  await page.goto("/");
  await expect(page.getByRole("heading", { name: /subtitle/i }).first()).toBeVisible();
  await page.screenshot({ path: testInfo.outputPath("1200-subtitle-dashboard-viewport.png") });
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
    for (const [name, url] of [
      ["SubDL", "https://subdl.com/panel/register"],
      ["OpenSubtitles", "https://www.opensubtitles.com/en/users/sign_up"],
      ["SubSource", "https://subsource.net/"],
    ]) {
      const link = providers.getByRole("link", { name: `Create ${name} account` });
      await expect(link).toHaveAttribute("href", url);
      await expect(link).toHaveAttribute("target", "_blank");
      await expect(link).toHaveAttribute("rel", /noopener noreferrer/);
    }
    await expect(providers).toContainText("On SubSource, choose Create Account.");
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
