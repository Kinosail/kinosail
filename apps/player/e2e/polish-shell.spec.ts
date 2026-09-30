import { execFileSync } from "node:child_process";
import { expect, test, type Page } from "@playwright/test";
import { configureLayoutAudit, login } from "./layout-audit-helpers";

configureLayoutAudit();
test.setTimeout(120_000);
test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires an isolated populated Player instance");
test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => localStorage.setItem("kinosail-theme", "dark"));
  await login(page);
});
test.afterEach(async ({ page }, info) => {
  await info.attach("run-context", { contentType: "application/json", body: JSON.stringify({
    revision: execFileSync("git", ["rev-parse", "HEAD"], { encoding: "utf8" }).trim(),
    sourceState: execFileSync("git", ["status", "--porcelain", "--untracked-files=no"], { encoding: "utf8" }).trim() ? "working tree" : "committed",
    command: "playwright test polish-shell.spec.ts", data: "Isolated Owner and generated CC0 library",
    environment: { platform: process.platform, browser: info.project.name, version: page.context().browser()?.version() }, result: info.status,
  }) });
});

test("administration uses a plain canvas while dark media browsing retains CinemaSail", { tag: "@smoke" }, async ({ page }, info) => {
  for (const width of [1440, 390]) {
    await page.setViewportSize({ width, height: 900 });
    await page.goto("/?view=movies");
    expect(await hasCanvasArtwork(page)).toBe(true);
    for (const route of ["/settings", "/settings/configuration", "/account", "/quick-connect"]) {
      await page.goto(route);
      await page.screenshot({ path: info.outputPath(`${width}-${route.replaceAll("/", "-")}.png`) });
      await expect(page.locator("body"), route).toHaveCSS("background-image", "none");
      expect(await hasCanvasArtwork(page), route).toBe(false);
    }
  }
});

test("landscape search stays reachable before and after focus on signed-in pages", { tag: "@smoke" }, async ({ page }, info) => {
  for (const width of [720, 390]) {
    await page.setViewportSize({ width, height: 450 });
    for (const route of ["/?view=movies", "/settings/configuration", "/account"]) {
      await page.goto(route);
      const search = page.getByRole("searchbox", { name: "Search all libraries" });
      await expect(page.locator(".header-actions > .header-supporter")).toBeHidden();
      const supporter = page.locator(".app-header > .mobile-supporter");
      await expect(supporter).toBeVisible();
      expect(await supporter.evaluate(element => {
        const box = element.getBoundingClientRect();
        return [...document.querySelectorAll(".app-header > .search, .header-compact-menu > summary")].filter(control => {
          const other = control.getBoundingClientRect();
          return box.left < other.right && box.right > other.left && box.top < other.bottom && box.bottom > other.top;
        }).map(control => control.className || control.tagName);
      }), "Support link must not overlap Search or Actions").toEqual([]);
      for (const focused of [false, true]) {
        if (focused) {
          await page.locator(".app-header > .brand-lockup").focus();
          await page.keyboard.press("Tab");
        }
        const geometry = await page.locator(".app-header .search").evaluate(element => {
          const box = element.getBoundingClientRect(); const style = getComputedStyle(element);
          return { left: box.left, right: box.right, width: box.width, viewport: innerWidth, position: style.position, cssWidth: style.width };
        });
        await page.screenshot({ path: info.outputPath(`${width}-${route.replaceAll("/", "-")}-${focused}.png`) });
        expect(geometry.left, JSON.stringify(geometry)).toBeGreaterThanOrEqual(0);
        expect(geometry.right, JSON.stringify(geometry)).toBeLessThanOrEqual(width);
        expect(geometry.width).toBeGreaterThanOrEqual(44);
        if (focused) await expect(search).toBeFocused();
      }
    }
  }
});

test("TMDB connection instructions remain readable without horizontal scrolling", { tag: "@smoke" }, async ({ page }, testInfo) => {
	test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");
	for (const viewport of [{ width: 1440, height: 900 }, { width: 390, height: 844 }, { width: 320, height: 800 }]) {
		await page.setViewportSize(viewport);
		await page.goto("/settings/configuration#integrations.tmdb");
		const section = page.locator('section[id="integrations.tmdb"]');
		const instructions = section.getByRole("list").getByRole("listitem");
		await expect(instructions).toHaveCount(3);
		for (const instruction of await instructions.all()) {
			const box = await instruction.boundingBox();
			expect(box).not.toBeNull();
			expect(box!.x, `${viewport.width}px instruction left edge`).toBeGreaterThanOrEqual(0);
			expect(box!.x + box!.width, `${viewport.width}px instruction right edge`).toBeLessThanOrEqual(viewport.width);
		}
		const requestAccess = section.getByRole("link", { name: "Request API access", exact: true });
		await requestAccess.focus();
		await expect(requestAccess).toBeFocused();
		await section.screenshot({ path: testInfo.outputPath(`${viewport.width}-tmdb-instructions.png`) });
	}
});

// Compare rendered canvas pixels; WebKit can report a painted body background as "none".
async function hasCanvasArtwork(page: Page): Promise<boolean> {
  const clip = { x: page.viewportSize()!.width - 4, y: 200, width: 4, height: 40 };
  const painted = await page.screenshot({ clip, animations: "disabled" });
  const original = await page.evaluate(() => {
    const original = document.body.style.cssText;
    document.body.style.setProperty("background", "none", "important");
    return original;
  });
  try {
    return !painted.equals(await page.screenshot({ clip, animations: "disabled" }));
  } finally {
    await page.evaluate(original => { document.body.style.cssText = original; }, original);
  }
}
