import { execFileSync } from "node:child_process";
import { expect, test, type Page } from "@playwright/test";
import { login } from "./subtitle-dashboard-helpers";

test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires an isolated populated Subtitles instance");
test.setTimeout(120_000);
test.afterEach(async ({ page }, info) => {
  await info.attach("run-context", { contentType: "application/json", body: JSON.stringify({
    revision: execFileSync("git", ["rev-parse", "HEAD"], { encoding: "utf8" }).trim(),
    sourceState: execFileSync("git", ["status", "--porcelain", "--untracked-files=no"], { encoding: "utf8" }).trim() ? "working tree" : "committed",
    command: "playwright test polish-shell.spec.ts", data: "Isolated Owner and generated CC0 library",
    environment: { platform: process.platform, browser: info.project.name, version: page.context().browser()?.version() }, result: info.status,
  }) });
});
test("administration uses a plain canvas while the dark subtitle workspace retains CinemaSail", { tag: "@smoke" }, async ({ page }, info) => {
  await page.addInitScript(() => localStorage.setItem("kinosail-theme", "dark"));
  await login(page);
  for (const width of [1440, 390]) {
    await page.setViewportSize({ width, height: 900 });
    await page.goto("/");
    expect(await hasCanvasArtwork(page)).toBe(true);
    for (const route of ["/settings", "/settings/configuration", "/account", "/quick-connect"]) {
      await page.goto(route);
      await page.screenshot({ path: info.outputPath(`${width}-${route.replaceAll("/", "-")}.png`) });
      await expect(page.locator("body"), route).toHaveCSS("background-image", "none");
      expect(await hasCanvasArtwork(page), route).toBe(false);
    }
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
