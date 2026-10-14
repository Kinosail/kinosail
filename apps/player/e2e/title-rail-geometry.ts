import { expect, type Page, type TestInfo } from "@playwright/test";

export async function recordTitleRail(page: Page, info: TestInfo, name: string, pending = false) {
  const geometry = await page.evaluate(async pending => {
    if (!pending) {
      await document.fonts.ready;
      await new Promise<void>(resolve => requestAnimationFrame(() => requestAnimationFrame(() => resolve())));
    }
    const rect = (selector: string) => {
      const element = document.querySelector(selector);
      if (!element || !element.getClientRects().length) return null;
      const { top, right, bottom, left, width, height } = element.getBoundingClientRect();
      return { top, right, bottom, left, width, height };
    };
    return {
      header: rect("body > .app-header"), search: rect(".app-header .search"),
      rail: rect("[data-title-jump-index]"), dock: rect("[data-mobile-tabs]"),
      compact: rect("[data-title-jump-open]"), viewport: { width: innerWidth, height: innerHeight },
      scrollY, overflow: document.documentElement.scrollWidth > innerWidth,
      fixed: getComputedStyle(document.querySelector("[data-title-jump-index]")!).position === "fixed",
    };
  }, pending);
  await info.attach(`${name}-geometry.json`, { body: JSON.stringify({
    revision: process.env.KINOSAIL_TEST_REVISION, browser: info.project.name, pending, ...geometry,
  }, null, 2), contentType: "application/json" });
  if (!pending) await page.screenshot({ path: info.outputPath(`${name}.png`) });
  expect.soft(geometry.overflow, `${name}: horizontal overflow`).toBe(false);
  expect(geometry.header, `${name}: header exists`).not.toBeNull();
  expect(geometry.search, `${name}: Search exists`).not.toBeNull();
  const tallPhone = geometry.viewport.width <= 700 && geometry.viewport.height >= 820;
  if (tallPhone) {
    expect(geometry.rail, `${name}: tall-phone rail exists`).not.toBeNull();
    expect(geometry.dock, `${name}: mobile dock exists`).not.toBeNull();
    expect.soft(geometry.fixed, `${name}: tall-phone rail stays fixed`).toBe(true);
  }
  if (geometry.rail) {
    const rail = geometry.rail, search = geometry.search!;
    const intersects = rail.left < search.right && rail.right > search.left && rail.top < search.bottom && rail.bottom > search.top;
    expect.soft(intersects, `${name}: rail does not intersect Search`).toBe(false);
  }
  if (geometry.rail && geometry.fixed) {
    expect.soft(geometry.rail.top, `${name}: rail clears header`).toBeGreaterThanOrEqual(geometry.header!.bottom);
    expect.soft(geometry.rail.bottom, `${name}: rail clears dock`).toBeLessThanOrEqual(geometry.dock!.top);
    expect.soft(geometry.rail.height, `${name}: rail has usable space`).toBeGreaterThan(0);
  }
  return geometry;
}

export async function verifyTitleRail(page: Page, info: TestInfo) {
  for (const width of [320, 390, 700]) {
    await page.setViewportSize({ width, height: 844 });
    let release!: () => void;
    const held = new Promise<void>(resolve => { release = resolve; });
    const bundle = "**/static/main.kinosail.bundle.js*";
    let entered = false, settled = false, continuationFailed = false, primaryFailed = false;
    await page.route(bundle, async route => {
      entered = true;
      try { await held; await route.continue(); }
      catch { continuationFailed = true; }
      finally { settled = true; }
    });
    try {
      await page.goto("/?view=movies", { waitUntil: "commit" });
      await expect.poll(() => entered, { message: "actual navigation bundle is held" }).toBe(true);
      // The tall-phone rail is server-rendered and visible before its handlers load.
      await expect(page.locator("[data-title-jump-index]")).toBeVisible();
      await recordTitleRail(page, info, `rail-${width}-pending`, true);
    } catch (error) {
      primaryFailed = true;
      throw error;
    } finally {
      release();
      let joinFailed = false, removalFailed = false, evidenceFailed = false;
      // Removing an active handler can continue its route before our handler does.
      // Release and join first, then remove the no-longer-active handler.
      try { await expect.poll(() => !entered || settled, { message: "held bundle handler settled" }).toBe(true); }
      catch { joinFailed = true; }
      try { await page.unroute(bundle); }
      catch { removalFailed = true; }
      try {
        await info.attach(`rail-${width}-bundle-cleanup.json`, { body: JSON.stringify({
          entered, settled, continuationFailed, joinFailed, removalFailed,
        }), contentType: "application/json" });
      } catch { evidenceFailed = true; }
      if (!primaryFailed && (continuationFailed || joinFailed || removalFailed || evidenceFailed)) {
        throw new Error("Held navigation bundle cleanup failed");
      }
    }
    await page.waitForLoadState("domcontentloaded");
    await expect(page.locator("[data-title-jump-index]")).toBeVisible();
    await recordTitleRail(page, info, `rail-${width}-loaded`);
    await page.evaluate(() => scrollTo({ top: document.documentElement.scrollHeight, behavior: "instant" }));
    await recordTitleRail(page, info, `rail-${width}-scrolled`);
  }
  // Resize the same document through compact, desktop, TV and tall-phone modes.
  for (const viewport of [{ width: 390, height: 650 }, { width: 1440, height: 900 }, { width: 1920, height: 1080 }, { width: 390, height: 844 }]) {
    await page.setViewportSize(viewport);
    await page.evaluate(() => scrollTo({ top: 0, behavior: "instant" }));
    const compact = viewport.height === 650;
    await expect(page.locator("[data-title-jump-index]")).toBeVisible({ visible: !compact });
    await recordTitleRail(page, info, `rail-resized-${viewport.width}x${viewport.height}`);
  }
  // Abort one actual title request; retain the loaded catalog and release busy state.
  const failedLetter = (url: URL) => url.pathname === "/" && url.searchParams.get("letter") === "B";
  await page.route(failedLetter, route => route.abort("failed"));
  try {
    await Promise.all([
      page.waitForEvent("requestfailed", request => failedLetter(new URL(request.url()))),
      page.locator('[data-title-jump-index] [data-title-letter="B"]').click(),
    ]);
    await expect(page.locator('#main[aria-busy="true"]')).toHaveCount(0);
    await expect(page.locator("#library .card")).toHaveCount(4);
    await recordTitleRail(page, info, "rail-failed-filter");
  } finally {
    await page.unroute(failedLetter);
  }
  await page.goto("/?view=movies&q=NoTitleRailFixtureMatch");
  await expect(page.getByRole("heading", { name: "No matching titles." })).toBeVisible();
  await expect(page.locator("[data-title-jump-index]")).toBeHidden();
  await page.screenshot({ path: info.outputPath("rail-empty.png") });
}
