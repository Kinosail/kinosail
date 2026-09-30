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
      await expect(page.locator(".supporter-gallery .supporter-badge")).toHaveCount(30);
      await expect.poll(() => page.locator(".supporter-gallery img").evaluateAll((images) => images.every((image) => (image as HTMLImageElement).complete && (image as HTMLImageElement).naturalWidth > 0))).toBe(true);
      if (state === "empty") await expect(page.locator(".supporter-honor")).toHaveCount(0);
      else {
        await expect(page.locator(".supporter-thanks")).toContainText("Thank you");
        await page.locator(".supporter-certificate-preview summary").first().click();
        await expect.poll(() => page.locator(".supporter-certificate-preview img").first().evaluate((image: HTMLImageElement) => image.naturalWidth)).toBeGreaterThan(0);
      }
      if (state === "archived") await expect(page.locator(".supporter-honor-copy")).toContainText("stays in your collection");
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
    { label: "Yearly", amounts: [12, 40, 64, 96, 144, 200, 280, 360, 480, 600], period: "/year" },
    { label: "One-time", amounts: [5, 15, 30, 60, 100, 150, 250, 400, 550, 750], period: " once" },
  ];
  await expect(page.locator(".supporter-contribute details")).toHaveCount(0);
  for (const plan of [...expected, expected[0]]) {
    await page.getByRole("radio", { name: plan.label, exact: true }).check();
    await expect(page.locator("[data-supporter-family]:not([hidden]) .supporter-price:visible")).toHaveCount(10);
    await expect(page.locator("[data-supporter-family]:not([hidden]) .supporter-price")).toHaveText(plan.amounts.map((amount) => `$${amount}${plan.period}`));
    await expect(page.locator("[data-supporter-family]:not([hidden]) .supporter-price").last()).toBeVisible();
    await expect(page.locator("[data-supporter-family]:not([hidden]) .supporter-badge").first()).toHaveAttribute("data-family", plan.label === "One-time" ? "one-time" : plan.label === "Yearly" ? "yearly" : "monthly");
  }
  await expect(page.getByRole("link", { name: "Continue to support" })).toHaveAttribute("rel", "external noreferrer");
});

test("Collected badges and hidden recognition work on desktop and mobile", async ({ page }) => {
  let hidden = false;
  await page.route("**/api/v1/supporter/collection", route => route.fulfill({ json: { display: hidden ? "hidden" : "automatic", badges: [
    { family: "patron-order", edition: "one-time", rank: 6, name: "Lighthouse" },
    { family: "living-standard", edition: "monthly", rank: 8, name: "Admiral" },
    { family: "living-standard", rank: 8, name: "Legacy Admiral", archived: true },
  ] } }));
  await login(page);
  for (hidden of [false, true]) for (const width of [1440, 390]) {
    await page.setViewportSize({ width, height: 900 });
    await page.goto("/?view=movies");
    const link = page.locator(width > 900 ? ".header-actions .header-supporter" : ".mobile-supporter");
    if (hidden) await expect(link).toBeHidden();
    else {
      await expect(link).toBeVisible();
      await expect(link).toHaveAttribute("aria-label", "Your supporter collection");
      await expect(link.locator("img")).toHaveCount(3);
      await expect(link.locator("img").nth(0)).toHaveAttribute("alt", "Lighthouse · one-time");
      await expect(link.locator("img").nth(1)).toHaveAttribute("alt", "Admiral · monthly");
      await expect(link.locator("img").nth(2)).toHaveAttribute("alt", "Legacy Admiral · Legacy recurring · past support");
      await expect.poll(() => link.locator("img").evaluateAll(images => images.every(image => (image as HTMLImageElement).naturalWidth > 0))).toBe(true);
    }
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
    const { display } = await (await page.request.get("/api/v1/supporter/display")).json();
    const selected = display === "hidden" ? "hidden" : "automatic";
    const current = body.replace(/(<option value="(?:automatic|hidden)")\s+selected/g, "$1")
      .replace(`<option value="${selected}"`, `<option value="${selected}" selected`);
    const rendered = csrf ? current.replace('</head>', `${csrf[0]}></head>`).replace('action="/supporter/display">', `action="/supporter/display"><input type="hidden" name="_csrf" value="${csrf[1]}">`) : current;
    await route.fulfill({ response, contentType: "text/html", body: rendered });
  });
  await page.goto("/supporter");
  const setting = page.getByLabel("Show supporter badges around the app");
  const original = (await (await page.request.get("/api/v1/supporter/display")).json()).display;
  try {
    await setting.selectOption("hidden");
    await page.getByRole("button", { name: "Save display choice", exact: true }).click();
    await expect(page).toHaveURL(/\/supporter#recognition$/);
    await expect.poll(async () => (await (await page.request.get("/api/v1/supporter/display")).json()).display).toBe("hidden");
    await page.reload();
    await expect(setting).toHaveValue("hidden");
    await setting.selectOption("automatic");
    await page.getByRole("button", { name: "Save display choice", exact: true }).click();
    await expect(page).toHaveURL(/\/supporter#recognition$/);
    await expect.poll(async () => (await (await page.request.get("/api/v1/supporter/display")).json()).display).toBe("automatic");
    await page.reload();
    await expect(setting).toHaveValue("automatic");
  } finally {
    const status = await page.evaluate(async display => {
      const csrf = document.querySelector<HTMLMetaElement>('meta[name="kinosail-csrf"]')!.content;
      return (await fetch("/api/v1/supporter/display", { method: "PUT", headers: { "Content-Type": "application/json", "X-Kinosail-CSRF": csrf }, body: JSON.stringify({ display }) })).status;
    }, original);
    expect(status).toBe(200);
  }
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
  await page.getByLabel("Show supporter badges around the app").focus();
  await expect(page.getByLabel("Show supporter badges around the app")).toBeFocused();
  await page.keyboard.press(browserName === "webkit" ? "Alt+Tab" : "Tab");
  expect(await page.evaluate(() => document.activeElement !== document.body && document.activeElement?.getClientRects().length! > 0)).toBe(true);
  await page.evaluate(() => localStorage.setItem("kinosail-theme", "light"));
  await page.reload();
  expect((await new AxeBuilder({ page }).include("main").analyze()).violations).toEqual([]);
  await page.screenshot({ path: testInfo.outputPath("supporter-light-320.png"), fullPage: true });
  await page.emulateMedia({ forcedColors: "active" });
  await page.keyboard.press("Tab");
  expect((await new AxeBuilder({ page }).include("main").analyze()).violations).toEqual([]);
  await page.screenshot({ path: testInfo.outputPath("supporter-forced-colors-320.png"), fullPage: true });
});
