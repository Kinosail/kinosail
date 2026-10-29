import { expect, test } from "@playwright/test";
import { login } from "./subtitle-dashboard-helpers";

test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires an isolated populated Subtitles instance");
test.setTimeout(120_000);
test.use({ serviceWorkers: "block" });

test("light appearance keeps the header opaque over scrolled content", async ({ page }) => {
  await page.addInitScript(() => { try { localStorage.setItem("kinosail-theme", "light"); } catch {} });
  await login(page);
  for (const width of [390, 1440]) {
    await page.setViewportSize({ width, height: 900 });
    for (const route of ["/", "/settings"]) {
      await page.goto(route);
      const alpha = await page.locator(".app-header").first().evaluate((element) => {
        const parts = getComputedStyle(element).backgroundColor.match(/[\d.]+/g) || [];
        return parts.length > 3 ? Number(parts[3]) : 1;
      });
      expect(alpha, `${route} at ${width}px`).toBe(1);
    }
  }
});

test("enlarged phone text keeps backup history inside the window", async ({ page }) => {
  await login(page);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/settings/backups");
  await page.evaluate(() => document.documentElement.style.setProperty("font-size", "200%", "important"));
  expect(await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)).toBeLessThanOrEqual(0);
});
