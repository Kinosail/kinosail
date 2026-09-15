import { expect, test } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { configureTestInstance, login } from "./test-instance-helpers";

configureTestInstance();
test.use({ serviceWorkers: "block" });
const directory = process.env.KINOSAIL_UI_FIXTURE_DIR;
const supporterPageURL = (url: URL) => url.pathname === "/supporter";

test("Sailcraft honors remain readable in every supporter state", async ({ page }, testInfo) => {
  test.skip(!directory, "requires rendered production supporter fixtures");
  test.setTimeout(180_000);
  await login(page);
  for (const [family, file] of [["living-standard", "living"], ["patron-order", "patron"]]) {
    const certificate = await readFile(join(directory!, `supporter-${file}-certificate.html`), "utf8");
    await page.route(`**/api/v1/supporter/certificates/${family}.svg`, (route) => route.fulfill({ status: 200, contentType: "image/svg+xml", body: certificate }));
  }
  for (const state of ["populated-both", "living", "patron", "archived", "long-name", "empty"]) {
    const body = await readFile(join(directory!, `supporter-${state}.html`), "utf8");
    const livingStandard = ["empty", "patron"].includes(state) ? undefined : { family: "living-standard", rank: 8, name: "Admiral", active: state !== "archived", expired: state === "archived" };
    const patronOrder = ["empty", "living", "archived"].includes(state) ? undefined : { family: "patron-order", rank: 6, name: "Lighthouse", active: true };
    await page.route("**/api/v1/supporter", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ livingStandard, patronOrder }) }));
    await page.route(supporterPageURL, (route) => route.fulfill({ status: 200, contentType: "text/html", body }));
    for (const width of [1440, 1024, 390, 320]) {
      await page.setViewportSize({ width, height: 900 });
      await page.goto("/supporter", { waitUntil: "domcontentloaded" });
      await expect(page.getByRole("heading", { name: "A place in the story.", exact: true })).toBeVisible();
      await expect(page.locator(".supporter-gallery .supporter-badge")).toHaveCount(20);
      await expect.poll(() => page.locator(".supporter-gallery img").evaluateAll((images) => images.every((image) => (image as HTMLImageElement).complete && (image as HTMLImageElement).naturalWidth > 0))).toBe(true);
      if (state === "empty") await expect(page.locator(".supporter-honor")).toHaveCount(0);
      else {
        await expect(page.locator(".supporter-thanks")).toContainText("Thank you");
        await page.locator(".supporter-certificate-preview summary").first().click();
        await expect.poll(() => page.locator(".supporter-certificate-preview img").first().evaluate((image: HTMLImageElement) => image.naturalWidth)).toBeGreaterThan(0);
      }
      if (state === "archived") await expect(page.locator(".supporter-honor-copy")).toContainText("stays in your archive");
      const geometry = await page.locator("main").evaluate((element) => {
        const bounds = element.getBoundingClientRect();
        const outside = [...element.querySelectorAll("*")].filter((child) => child.getBoundingClientRect().right > bounds.right + 1).map((child) => ({ tag: child.tagName, className: child.className, width: child.getBoundingClientRect().width, right: child.getBoundingClientRect().right }));
        const overflowing = [...element.querySelectorAll<HTMLElement>("*")].filter((child) => child.scrollWidth > child.clientWidth + 1).map((child) => ({ tag: child.tagName, className: child.className, width: child.clientWidth, scrollWidth: child.scrollWidth, overflow: getComputedStyle(child).overflowX }));
        return { width: element.clientWidth, scrollWidth: element.scrollWidth, outside, overflowing };
      });
      expect(geometry.scrollWidth <= geometry.width, `${state} ${width}px: ${JSON.stringify(geometry)}`).toBe(true);
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      expect((await new AxeBuilder({ page }).include("main").analyze()).violations, `${state} ${width}px`).toEqual([]);
      await page.evaluate(() => scrollTo({ top: 0, left: 0, behavior: "instant" }));
      await page.screenshot({ path: testInfo.outputPath(`${state}-${width}.png`), fullPage: width === 390 && state === "populated-both", animations: "disabled" });
    }
    await page.unroute(supporterPageURL);
    await page.unroute("**/api/v1/supporter");
  }
});

test("Contribution frequency shows all ten agreed amounts", async ({ page }) => {
  await login(page);
  await page.goto("/supporter");
  const expected = [
    { label: "Monthly", amounts: [3, 5, 8, 12, 18, 25, 35, 45, 60, 75], period: "/month" },
    { label: "Annual", amounts: [12, 40, 64, 96, 144, 200, 280, 360, 480, 600], period: "/year" },
    { label: "One-time", amounts: [5, 15, 30, 60, 100, 150, 250, 400, 550, 750], period: " once" },
  ];
  await expect(page.locator(".supporter-contribute details")).toHaveCount(0);
  for (const plan of [...expected, expected[0]]) {
    await page.getByRole("radio", { name: plan.label, exact: true }).check();
    await expect(page.locator("[data-supporter-family]:not([hidden]) .supporter-price:visible")).toHaveCount(10);
    await expect(page.locator("[data-supporter-family]:not([hidden]) .supporter-price")).toHaveText(plan.amounts.map((amount) => `$${amount}${plan.period}`));
    await expect(page.locator("[data-supporter-family]:not([hidden]) .supporter-price").last()).toBeVisible();
    await expect(page.locator("[data-supporter-family]:not([hidden]) .supporter-badge").first()).toHaveAttribute("data-family", plan.label === "One-time" ? "patron-order" : "living-standard");
  }
  await expect(page.getByRole("link", { name: "Continue to support site" })).toHaveAttribute("rel", "external noreferrer");
});

test("Rank recognition follows saved display choice on desktop and mobile", async ({ page }) => {
  await page.route("**/api/v1/supporter", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ livingStandard: { family: "living-standard", rank: 8, name: "Admiral", active: true }, patronOrder: { family: "patron-order", rank: 6, name: "Lighthouse", active: true } }) }));
  await login(page);
  for (const [display, name, family] of [["automatic", "Admiral", "living-standard"], ["patron-order", "Lighthouse", "patron-order"], ["hidden", "Supporter", ""]]) {
    await page.route("**/api/v1/supporter/display", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ display }) }));
    for (const width of [1440, 390]) {
      await page.setViewportSize({ width, height: 900 });
      await page.goto("/?view=movies");
      if (width < 900) await page.locator(".nav-more > summary").click();
      const link = page.locator(width < 900 ? ".nav-supporter" : ".nav-main-supporter");
      await expect(link).toContainText(name);
      if (family) await expect(link.locator("img")).toHaveAttribute("src", new RegExp(`${family}-\\d+-small\\.svg`));
      else {
        await expect(link.locator("img")).toHaveCount(0);
        await expect(page.locator(".supporter-signature")).toBeHidden();
        await expect(link).not.toHaveAttribute("aria-label");
      }
    }
    await page.unroute("**/api/v1/supporter/display");
  }
});

test("Display settings persist through the web form and API", async ({ page }) => {
  test.skip(!directory, "requires the production supporter fixture");
  await login(page);
  const body = await readFile(join(directory!, "supporter-populated-both.html"), "utf8");
  await page.route(supporterPageURL, async (route) => {
    const response = await route.fetch();
    const original = await response.text();
    const csrf = original.match(/<meta name="kinosail-csrf" content="([^"]+)"/);
    const rendered = csrf ? body.replace('</head>', `${csrf[0]}></head>`).replace('action="/supporter/display">', `action="/supporter/display"><input type="hidden" name="_csrf" value="${csrf[1]}">`) : body;
    await route.fulfill({ response, contentType: "text/html", body: rendered });
  });
  await page.goto("/supporter");
  await page.getByLabel("Badge shown in navigation").selectOption("hidden");
  await page.getByRole("button", { name: "Save display choice" }).click();
  await expect(page).toHaveURL(/\/supporter#recognition$/);
  const saved = await page.request.get("/api/v1/supporter/display");
  expect(await saved.json()).toEqual({ display: "hidden" });
  await page.goto("/supporter");
  await page.getByLabel("Badge shown in navigation").selectOption("automatic");
  await page.getByRole("button", { name: "Save display choice" }).click();
  await expect.poll(async () => (await (await page.request.get("/api/v1/supporter/display")).json()).display).toBe("automatic");
});

test("Honors support keyboard, light theme, reduced motion, and forced colors", async ({ page, browserName }, testInfo) => {
  test.skip(!directory, "requires the production supporter fixture");
  const body = await readFile(join(directory!, "supporter-populated-both.html"), "utf8");
  await page.route(supporterPageURL, (route) => route.fulfill({ status: 200, contentType: "text/html", body }));
  await login(page);
  await page.setViewportSize({ width: 320, height: 900 });
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.goto("/supporter");
  await page.getByRole("button", { name: "Replay badge reveal" }).click();
  expect(await page.locator(".supporter-honor-art .supporter-badge").evaluate((art) => art.getAnimations().length)).toBe(0);
  await page.getByLabel("Badge shown in navigation").focus();
  await expect(page.getByLabel("Badge shown in navigation")).toBeFocused();
  await page.keyboard.press(browserName === "webkit" ? "Alt+Tab" : "Tab");
  await expect(page.getByRole("button", { name: "Save display choice" })).toBeFocused();
  await page.evaluate(() => localStorage.setItem("kinosail-theme", "light"));
  await page.reload();
  expect((await new AxeBuilder({ page }).include("main").analyze()).violations).toEqual([]);
  await page.screenshot({ path: testInfo.outputPath("supporter-light-320.png"), fullPage: true });
  await page.emulateMedia({ forcedColors: "active" });
  await page.keyboard.press("Tab");
  expect((await new AxeBuilder({ page }).include("main").analyze()).violations).toEqual([]);
  await page.screenshot({ path: testInfo.outputPath("supporter-forced-colors-320.png"), fullPage: true });
});
