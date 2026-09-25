import { readFile } from "node:fs/promises";
import { expect, test } from "@playwright/test";

const markup = await readFile(new URL("../internal/server/web/supporter.html", import.meta.url), "utf8");
const styles = await Promise.all([
  "../internal/server/web/static/base.css",
  "../../../packages/webassets/static/last-light.css",
  "../internal/server/web/static/supporter.css",
].map(path => readFile(new URL(path, import.meta.url), "utf8")));

for (const width of [320, 390, 1440]) {
  test(`Dashboard Badge Case skeleton follows loaded columns at ${width}px`, { tag: "@smoke" }, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 900 });
    await page.route("**/*", route => route.abort());
    await page.setContent(markup);
    for (const content of styles) await page.addStyleTag({ content });
    expect(await page.locator(".loading-badges").count()).toBe(2);
    for (const family of await page.locator(".loading-badges").all()) await expect(family.locator("span")).toHaveCount(10);
    const pendingMasterwork = await page.locator(".loading-masterwork").boundingBox();
    const pendingBadge = await page.locator(".loading-badges span").first().boundingBox();
    const pendingActivation = await page.locator(".loading-activation").boundingBox();
    await page.screenshot({ path: testInfo.outputPath(`${width}-pending.png`), fullPage: true });
    await page.locator("#supporter-content").evaluate(content => {
      for (const id of ["living-badges", "patron-badges"]) {
        document.getElementById(id)!.innerHTML = Array.from({ length: 10 }, (_, index) =>
          `<li class="case-badge"><span class="badge-mark"><i></i></span><span><small>${index + 1} / 10</small><strong>Sample badge</strong></span></li>`).join("");
      }
      content.removeAttribute("hidden");
      document.getElementById("supporter-loading")!.hidden = true;
    });
    const loadedMasterwork = await page.locator(".masterwork").boundingBox();
    const loadedBadge = await page.locator(".case-badge").first().boundingBox();
    const loadedActivation = await page.locator(".activation").boundingBox();
    expect(pendingMasterwork?.width).toBeCloseTo(loadedMasterwork!.width, 0);
    expect(Math.abs(pendingMasterwork!.height - loadedMasterwork!.height)).toBeLessThan(32);
    expect(pendingBadge?.width).toBeCloseTo(loadedBadge!.width, 0);
    expect(pendingBadge?.height).toBeCloseTo(loadedBadge!.height, 0);
    expect(Math.abs(pendingBadge!.y - loadedBadge!.y)).toBeLessThan(48);
    expect(pendingActivation?.width).toBeCloseTo(loadedActivation!.width, 0);
    expect(Math.abs(pendingActivation!.y - loadedActivation!.y)).toBeLessThan(96);
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width);
    await page.screenshot({ path: testInfo.outputPath(`${width}-loaded.png`), fullPage: true });
  });
}
