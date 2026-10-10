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


async function cacheDiagnostic(page: import("@playwright/test").Page) {
  const unavailable = { schemaVersion: 1, supported: false, present: false, frameCount: 0, reasons: [] as string[], truncated: false, navigationType: "unknown" };
  try {
    return await page.evaluate(allowed => {
      type Frame = { reasons?: { reason?: string }[]; children?: Frame[] };
      const entry = performance.getEntriesByType("navigation")[0] as PerformanceNavigationTiming & { notRestoredReasons?: Frame | null };
      const supported = Boolean(entry && "notRestoredReasons" in entry), root = entry?.notRestoredReasons;
      const present = Boolean(root && typeof root === "object"), reasons = new Set<string>();
      const queue = present ? [{ node: root!, depth: 0 }] : [];
      let frameCount = 0, truncated = false;
      while (queue.length && frameCount < 64) {
        const { node, depth } = queue.shift()!;
        frameCount++;
        const items = Array.isArray(node.reasons) ? node.reasons : [];
        if (items.length > 64) truncated = true;
        for (const item of items.slice(0, 64)) reasons.add(typeof item?.reason === "string" && allowed.includes(item.reason) ? item.reason : "other");
        const children = Array.isArray(node.children) ? node.children : [];
        if (children.length > 64 || depth >= 8 && children.length) truncated = true;
        if (depth < 8) for (const child of children.slice(0, 64)) if (child && typeof child === "object") queue.push({ node: child, depth: depth + 1 });
      }
      if (queue.length) truncated = true;
      const navigationType = typeof entry?.type === "string" && ["navigate", "reload", "back_forward", "prerender"].includes(entry.type) ? entry.type : "unknown";
      return { schemaVersion: 1, supported, present, frameCount, reasons: [...reasons].sort(), truncated, navigationType };
    }, ["unload-listener","unload-handler","response-cache-control-no-store","response-cache-control-no-store-with-cookie-modification","related-active-contents","masked","websocket","outstanding-network-request","other"]);
  } catch { return unavailable; }
}

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
      try { await info.attach("native-cache-diagnostic", { body: JSON.stringify(await cacheDiagnostic(page)), contentType: "application/json" }); } catch { /* Preserve the original assertion outcome. */ }
    }
  });
});
