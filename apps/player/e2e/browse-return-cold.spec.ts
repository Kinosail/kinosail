import { expect, test } from "@playwright/test";
import { assertReturn, observe, origin, record, selectMovie, settle, snapshot } from "./browse-return-helpers";

const mode = process.env.KINOSAIL_BROWSE_RETURN_CASES || "primary";
const modes = ["primary", "cold", "bfcache", "htmx", "shows", "search", "all"];
if (!modes.includes(mode)) throw new Error("invalid browse-return case selection");
test.skip(!origin, "requires TestBrowseReturnBrowserJourney disposable Go Server");
test.setTimeout(25_000);
test.beforeEach(async ({ page }) => observe(page));
const selected = (value: string) => mode === value || mode === "all";

// Worker-scoped browser options must stay at file scope. Pinned Playwright
// already supplies this switch; it also states this cold boundary explicitly.
test.use({ serviceWorkers: "block", video: "off", launchOptions: { args: ["--disable-back-forward-cache"] } });

if (selected("cold")) test.describe("native Back without browser cache", () => {
  for (const width of [390, 1440]) test(`cold native Back restores later Movie cards at ${width}px`, async ({ page }, info) => {
    await page.setViewportSize({ width, height: 844 });
    const { href, before } = await selectMovie(page, info);
    try {
      await page.goBack({ waitUntil: "domcontentloaded" });
      await expect.poll(async () => (await snapshot(page, href)).document.shows.at(-1)?.persisted, { timeout: 1500, message: "cold boundary prerequisite: completed non-persisted pageshow" }).toBe(false);
      const returned = await record(page, info, "cold-boundary", href);
      expect(returned.state.document.document, "cold boundary prerequisite: fresh document").not.toBe(before.state.document.document);
      expect(returned.state.document.shows.at(-1)?.persisted, "cold boundary prerequisite: native pageshow").toBe(false);
      expect(returned.state.navigation).toContain("back_forward");
      expect(returned.peer.slice(before.peer.length).some(request => !request.continuation && !request.history), "cold boundary prerequisite: real root document GET").toBe(true);
      await assertReturn(page, before.state, href);
    } finally {
      await record(page, info, "returned", href);
    }
  });
});

if (selected("search")) test.describe("live HTMX search then cold playback Back", () => {
  test("live query uses current URL rather than the document's initial browse key", async ({ page }, info) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto(`${origin}/?view=movies&sort=title&limit=4`);
    const initial = await record(page, info, "original-url", "");
    await page.locator('form.search input[type="search"]').fill("Return Movie");
    await expect(page).toHaveURL(/q=Return(?:\+|%20)Movie/);
    // Search's public form resets scope/offset/limit. The 32-title search result
    // fits its default 100 limit; this guards the changed-URL key, not extent.
    const expected = Array.from({ length: 32 }, (_, position) => `Return Movie ${String(position + 1).padStart(2, "0")}`);
    await expect(page.locator("#library .card h2")).toHaveText(expected);
    const link = page.locator("#library a.card").filter({ hasText: "Return Movie 25" });
    const href = (await link.getAttribute("href"))!;
    await link.scrollIntoViewIfNeeded();
    await link.focus();
    await settle(page, href);
    const before = await record(page, info, "before", href);
    expect(before.state.document.document).toBe(initial.state.document.document);
    expect(before.state.values).not.toEqual(initial.state.values);
    await link.click();
    await expect(page.locator("body.player-page")).toBeVisible();
    try {
      await page.goBack({ waitUntil: "domcontentloaded" });
      await expect.poll(async () => (await snapshot(page, href)).document.shows.at(-1)?.persisted, { timeout: 1500, message: "cold search prerequisite: completed non-persisted pageshow" }).toBe(false);
      const returned = await record(page, info, "cold-boundary", href);
      expect(returned.state.document.document).not.toBe(before.state.document.document);
      expect(returned.state.document.shows.at(-1)?.persisted).toBe(false);
      expect(returned.state.navigation).toContain("back_forward");
      await assertReturn(page, before.state, href, expected);
    } finally {
      await record(page, info, "returned", href);
    }
  });
});
