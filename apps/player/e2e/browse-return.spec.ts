import { expect, test } from "@playwright/test";
import { assertReturn, assetReceipt, loadAll, movieTitles, observe, origin, record, selectMovie, settle, showTitles, snapshot } from "./browse-return-helpers";

const mode = process.env.KINOSAIL_BROWSE_RETURN_CASES || "primary";
const modes = ["primary", "cold", "bfcache", "htmx", "shows", "search", "all"];
if (!modes.includes(mode)) throw new Error("invalid browse-return case selection");
test.skip(!origin, "requires TestBrowseReturnBrowserJourney disposable Go Server");
test.use({ serviceWorkers: "block", video: "off" });
test.setTimeout(25_000);
test.beforeEach(async ({ page }) => observe(page));
const selected = (value: string) => mode === value || mode === "all";

if (selected("primary")) for (const width of [390, 1440]) {
  test(`visible Player Back preserves Movies query, offset, extent, focus and scroll at ${width}px`, async ({ page }, info) => {
    await page.setViewportSize({ width, height: 844 });
    const { href, before } = await selectMovie(page, info);
    try {
      const back = page.locator("a.back");
      await expect(back).toBeVisible();
      await back.focus();
      await expect(back).toBeFocused();
      await page.keyboard.press("Enter");
      await expect(page.locator("#main")).toBeVisible();
      await assertReturn(page, before.state, href);
    } finally {
      await record(page, info, "returned", href);
    }
  });
}

if (selected("htmx")) test("HTMX title-letter Back fetches current browse data and restores extent without a native pageshow", async ({ page }, info) => {
  const expected = ["Anchor Movie", ...Array.from({ length: 32 }, (_, position) => `Return Movie ${String(position + 1).padStart(2, "0")}`), "Zeta Movie"];
  await page.setViewportSize({ width: 1440, height: 844 });
  await page.goto(`${origin}/?view=movies&sort=title&limit=4`);
  await assetReceipt(page, info);
  await loadAll(page, expected);
  const href = (await page.locator("#library a.card").filter({ hasText: "Return Movie 25" }).getAttribute("href"))!;
  // The actual departing action is the title-letter link, so focus expectation
  // for this history-only control belongs to that link, not a playback card.
  const letter = page.locator('[data-title-jump-index] [data-title-letter="A"]');
  await letter.scrollIntoViewIfNeeded();
  await letter.focus();
  await settle(page, href);
  const before = await record(page, info, "before", href);
  await letter.click();
  await expect(page).toHaveURL(/letter=A/);
  await expect(page.locator("#library .card h2")).toHaveText(["Anchor Movie"]);
  const atLetter = await record(page, info, "letter", href);
  try {
    // A real same-document history traversal triggers HTMX's normal fetch.
    await page.evaluate(() => history.back());
    await expect.poll(async () => (await snapshot(page, href)).document.histories, { timeout: 4000, message: "HTMX prerequisite: actual history restore event" }).toBeGreaterThan(atLetter.state.document.histories);
    await expect.poll(async () => {
      const peer = await (await page.request.get(`${origin}/__browse-return`)).json();
      return peer.slice(atLetter.peer.length).some((request: { history: boolean }) => request.history);
    }, { timeout: 1500, message: "HTMX prerequisite: native peer observed the history request" }).toBe(true);
    const returned = await record(page, info, "htmx-boundary", href);
    expect(returned.state.document.document).toBe(before.state.document.document);
    expect(returned.state.document.shows).toEqual(before.state.document.shows);
    expect(returned.peer.slice(atLetter.peer.length).some(request => request.history), "HTMX prerequisite: actual HX-History-Restore-Request").toBe(true);
    await expect.poll(async () => (await snapshot(page, href)).values).toEqual(before.state.values);
    await expect(page.locator("#library .card h2")).toHaveText(expected, { timeout: 5000 });
    await expect(page.locator('[data-title-jump-index] [data-title-letter="A"]')).toBeFocused();
    await settle(page, href);
    expect(Math.abs((await snapshot(page, href)).scroll.y - before.state.scroll.y)).toBeLessThanOrEqual(2);
  } finally {
    await record(page, info, "returned", href);
  }
});

if (selected("shows")) for (const viaDetails of [false, true]) {
  test(`visible Player Back restores original Shows action via ${viaDetails ? "details and episode" : "direct Play"}`, async ({ page }, info) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto(`${origin}/?view=shows&q=Return+Show&sort=title&limit=4`);
    await assetReceipt(page, info);
    await loadAll(page, showTitles);
    const card = page.locator("#library .show-card").filter({ has: page.getByRole("heading", { name: "Return Show 11", exact: true }) });
    const action = card.locator(viaDetails ? "a.show-details" : "a.show-play");
    const href = (await action.getAttribute("href"))!;
    expect(href).toMatch(viaDetails ? /^\/show\/[a-f0-9]{16}$/ : /^\/watch\/[a-f0-9]{16}$/);
    await action.scrollIntoViewIfNeeded();
    await action.focus();
    await settle(page, href);
    const before = await record(page, info, "before", href);
    expect(before.state.profile).toBe("local-owner");
    expect(before.state.scroll.y).toBeGreaterThan(100);
    await action.click();
    if (viaDetails) {
      await expect(page.locator("body.show-detail")).toBeVisible();
      await page.locator(".episode-ledger a.episode").click();
    }
    await expect(page.locator("body.player-page")).toBeVisible();
    expect(await page.locator("body").getAttribute("data-viewer-profile")).toBe(before.state.profile);
    await record(page, info, "player", href);
    try {
      await page.locator("a.back").click();
      await expect(page.locator("#main")).toBeVisible();
      await assertReturn(page, before.state, href, showTitles);
    } finally {
      await record(page, info, "returned", href);
    }
  });
}
