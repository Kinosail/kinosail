import { expect, test } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { observe, origin, record } from "./browse-return-helpers";

test.skip(!origin, "requires disposable Go Server browse-return fixture");
test.use({ serviceWorkers: "block", video: "off" });
test.beforeEach(async ({ page }) => observe(page));

for (const width of [390, 1440, 1920]) {
  test(`Player has one accessible return link with Movies context and a direct-entry fallback at ${width}px`, async ({ page }, info) => {
    await page.setViewportSize({ width, height: width === 1920 ? 1080 : 844 });
    const path = "/?view=movies&sort=title&limit=4";
    await page.goto(`${origin}${path}`);
    const movie = page.locator("#library a.card").first();
    const href = (await movie.getAttribute("href"))!;
    await movie.click();
    const back = page.getByRole("link", { name: "Back to Movies", exact: true });
    await expect(back).toBeVisible();
    await expect(back).toHaveAttribute("href", path);
    await expect(page.getByRole("link", { name: "Library", exact: true })).toHaveCount(0);
    expect((await new AxeBuilder({ page }).include("a.back").analyze()).violations).toEqual([]);
    await record(page, info, "movies-player", href);
    await back.focus();
    await expect(back).toBeFocused();
    await page.keyboard.press("Enter");
    await expect(page).toHaveURL(`${origin}${path}`);
    await expect(page.locator(`#library a.card[href="${href}"]`)).toBeFocused();

    const direct = await page.context().newPage();
    await observe(direct);
    await direct.goto(`${origin}${href}`);
    const fallback = direct.getByRole("link", { name: "Library", exact: true });
    await expect(fallback).toHaveCount(1);
    await expect(fallback).toBeVisible();
    await expect(fallback).toHaveAttribute("href", "/");
    await record(direct, info, "direct-player", href);
    await fallback.click();
    await expect(direct).toHaveURL(`${origin}/`);
    await direct.close();
  });
}

for (const path of ["/?view=all", "/"]) test(`${path === "/" ? "Plain root" : "Home"} keeps its exact return after Mark watched`, async ({page}, info) => {
  if (path === "/?view=all") await page.setViewportSize({width: 390, height: 844});
  await page.goto(`${origin}${path}`);
  const movie = page.locator('a.card[href^="/watch/"]').first();
  const href = (await movie.getAttribute("href"))!;
  await movie.click();
  const unwatched = page.getByRole("button", {name: "Mark unwatched"});
  if (await unwatched.isVisible()) await unwatched.click();
  await page.getByRole("button", {name: "Mark watched"}).click();
  await expect(unwatched).toBeVisible();
  const back = page.getByRole("link", {name: path === "/" ? "Library" : "Back to Library", exact: true});
  await expect(back).toBeVisible();
  await expect(back).toHaveAttribute("href", path);
  await expect(page.getByRole("link", {name: path === "/" ? "Back to Library" : "Library", exact: true})).toHaveCount(0);
  await record(page, info, path === "/" ? "root-watched-return" : "home-watched-return", href);
  await back.click();
  await expect(page).toHaveURL(`${origin}${path}`);
  await expect(page.getByRole("link", {name: /\bResume\b/})).toHaveCount(0);
  if (path === "/?view=all") {
    await expect(page.locator("nav.mobile-navigation")).toHaveClass(/has-personal-tabs/);
    const homeLinks = page.locator("nav.mobile-navigation").getByRole("link", {name: "Home", exact: true, includeHidden: true});
    await expect(homeLinks).toHaveCount(2);
    await expect(homeLinks.first()).toBeHidden();
    const visibleHome = homeLinks.filter({visible: true});
    await expect(visibleHome).toHaveCount(1);
    await expect(visibleHome).toHaveAttribute("href", "/?view=all");
    await visibleHome.click();
    await expect(page).toHaveURL(`${origin}/?view=all`);
  }
});
