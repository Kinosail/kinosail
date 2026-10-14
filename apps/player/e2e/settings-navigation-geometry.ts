import { expect, type Page, type TestInfo } from "@playwright/test";

async function recordNavigation(page: Page, info: TestInfo, noticeExpected: boolean, phase: string) {
  const pending = phase === "pending-bundle";
  const geometry = await page.evaluate(async pending => {
    if (!pending) await document.fonts.ready;
    scrollTo({ top: 0, behavior: "instant" });
    await new Promise<void>(resolve => requestAnimationFrame(() => requestAnimationFrame(() => resolve())));
    const box = (selector: string) => {
      const element = document.querySelector(selector);
      if (!element?.getClientRects().length) return null;
      const { top, right, bottom, left, width, height } = element.getBoundingClientRect();
      return { top, right, bottom, left, width, height };
    };
    return {
      nav: box("[data-settings-nav]"), levels: box("[data-settings-levels]"),
      content: box("[data-settings-flow]"), notice: box("#restart-required"),
      viewport: { width: innerWidth, height: innerHeight }, scrollY,
      overflow: document.documentElement.scrollWidth > innerWidth,
    };
  }, pending);
  const name = `settings-navigation-${noticeExpected ? "restart" : "no-restart"}-${geometry.viewport.width}-${phase}`;
  await info.attach(`${name}.json`, { body: JSON.stringify({
    revision: process.env.KINOSAIL_TEST_REVISION, browser: info.project.name, noticeExpected, phase, ...geometry,
  }), contentType: "application/json" });
  // Playwright screenshots also await font readiness, which a held script can delay.
  if (!pending) await page.screenshot({ path: info.outputPath(`${name}.png`) });
  expect(geometry.nav, `${name}: category menu exists`).not.toBeNull();
  expect(geometry.levels, `${name}: level controls exist`).not.toBeNull();
  expect(geometry.content, `${name}: content exists`).not.toBeNull();
  expect.soft(Boolean(geometry.notice), `${name}: expected restart state`).toBe(noticeExpected);
  expect.soft(geometry.scrollY, `${name}: starts at top`).toBe(0);
  expect.soft(geometry.overflow, `${name}: no horizontal overflow`).toBe(false);
  const nav = geometry.nav!, content = geometry.content!, levels = geometry.levels!;
  expect.soft(nav.top, `${name}: categories below level controls`).toBeGreaterThanOrEqual(levels.bottom - 1);
  expect.soft(content.top, `${name}: content below level controls`).toBeGreaterThanOrEqual(levels.bottom - 1);
  if (geometry.notice) expect.soft(levels.top, `${name}: levels below restart notice`).toBeGreaterThanOrEqual(geometry.notice.bottom - 1);
  if (geometry.viewport.width > 900) {
    expect.soft(Math.abs(nav.top - content.top), `${name}: categories beside content`).toBeLessThanOrEqual(1);
    expect.soft(nav.right, `${name}: columns do not overlap`).toBeLessThanOrEqual(content.left);
    expect.soft(nav.top, `${name}: categories start in the viewport`).toBeLessThan(geometry.viewport.height);
  } else {
    expect.soft(content.top, `${name}: mobile content below categories`).toBeGreaterThanOrEqual(nav.bottom - 1);
  }
}

export async function verifySettingsNavigation(page: Page, info: TestInfo, noticeExpected: boolean) {
  for (const width of [320, 390, 900, 901, 1024, 1440, 1920]) {
    await page.setViewportSize({ width, height: width === 1920 ? 1080 : 900 });
    await page.goto("/settings");
    await expect(page.locator("[data-settings-ready]")).toHaveCount(1);
    await recordNavigation(page, info, noticeExpected, "loaded");
  }
  // Keep the actual server document and stylesheet when its deferred bundle is pending or fails.
  await page.setViewportSize({ width: 1440, height: 900 });
  const bundle = "**/static/main.kinosail.bundle.js*";
  let release!: () => void;
  const held = new Promise<void>(resolve => { release = resolve; });
  let entered = false, settled = false, abortFailed = false, primaryFailed = false;
  await page.route(bundle, async route => {
    entered = true;
    try { await held; await route.abort("failed"); }
    catch { abortFailed = true; }
    finally { settled = true; }
  });
  try {
    await page.goto("/settings", { waitUntil: "commit" });
    await expect.poll(() => entered, { message: "actual settings bundle held" }).toBe(true);
    await expect(page.locator("[data-settings-nav]")).toBeVisible();
    await recordNavigation(page, info, noticeExpected, "pending-bundle");
  } catch (error) {
    primaryFailed = true;
    throw error;
  } finally {
    release();
    let joinFailed = false, removalFailed = false, evidenceFailed = false;
    try { await expect.poll(() => !entered || settled, { message: "held settings bundle settled" }).toBe(true); }
    catch { joinFailed = true; }
    try { await page.unroute(bundle); }
    catch { removalFailed = true; }
    try {
      await info.attach(`settings-navigation-${noticeExpected}-cleanup.json`, {
        body: JSON.stringify({ entered, settled, abortFailed, joinFailed, removalFailed }), contentType: "application/json",
      });
    } catch { evidenceFailed = true; }
    if (!primaryFailed && (abortFailed || joinFailed || removalFailed || evidenceFailed)) {
      throw new Error("Held settings bundle cleanup failed");
    }
  }
  expect(abortFailed, "bundle failure injection settled").toBe(false);
  await page.waitForLoadState("domcontentloaded");
  await expect(page.locator("[data-settings-ready]")).toHaveCount(0);
  await recordNavigation(page, info, noticeExpected, "failed-bundle");
}
