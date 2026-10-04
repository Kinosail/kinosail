import { expect, test } from "@playwright/test";
import { assertReturn, observe, origin, record, selectMovie, snapshot } from "./browse-return-helpers";

const mode = process.env.KINOSAIL_BROWSE_RETURN_CASES || "primary";
const modes = ["primary", "cold", "bfcache", "htmx", "shows", "search", "all"];
if (!modes.includes(mode)) throw new Error("invalid browse-return case selection");
test.skip(!origin, "requires TestBrowseReturnBrowserJourney disposable Go Server");
test.setTimeout(25_000);
test.beforeEach(async ({ page }) => observe(page));
const selected = (value: string) => mode === value || mode === "all";

// A separate file gives this worker its own launch configuration. Cached full
// Chromium's new headless mode still must demonstrate real native admission.
test.use({ serviceWorkers: "block", video: "off", channel: "chromium", launchOptions: { ignoreDefaultArgs: ["--disable-back-forward-cache"] } });

if (selected("bfcache")) test.describe("observed native browser cache", () => {
  test("native BFCache preserves loaded Movie DOM without repeated continuation", async ({ page }, info) => {
    await page.setViewportSize({ width: 1440, height: 844 });
    const { href, before } = await selectMovie(page, info);
    const leaving = await (await page.request.get(`${origin}/__browse-return`)).json();
    try {
      await page.goBack({ waitUntil: "commit" });
      await expect.poll(async () => (await snapshot(page, href)).document.shows.at(-1)?.persisted, {
        timeout: 4000, message: "BFCache prerequisite: observed native persisted pageshow; failure is not product RED",
      }).toBe(true);
      const returned = await record(page, info, "native-cache-boundary", href);
      expect(returned.state.document.document, "BFCache prerequisite: original document retained").toBe(before.state.document.document);
      expect(returned.peer).toHaveLength(leaving.length);
      await assertReturn(page, before.state, href);
      const settledPeer = await (await page.request.get(`${origin}/__browse-return`)).json();
      expect(settledPeer, "native cached return does not start delayed continuation").toHaveLength(leaving.length);
    } finally {
      await record(page, info, "returned", href);
    }
  });
});
